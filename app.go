package main

import "fmt"

// 应用层：键盘 → 意图 → 网络命令；网络事件 → 状态 → 重绘。
// 这一层不碰终端，也不解析协议细节，单测可以直接喂按键与事件。

type app struct {
	st   *appState
	ui   uiState
	cfg  config
	quit bool

	// 复用的字符屏：同一尺寸下保留上一帧签名，没变的行不重写。
	scr *screen

	// 日志向上翻了几行（0 = 贴底）。
	logBack int

	cmds chan netCmd
	evs  chan netEvent

	submitAt      int64
	pendingLogin  *loginAttempt
	pendingCreate string
	pendingMove   *int
	exploreLeft   int
	exploreAt     int64
	preferredPage *page
	// 下一次允许补拉舆图的时刻（毫秒）。赶路中服务端会拒 worldMap（E-BUSY），
	// 拒了就等一会儿再要，否则地名会一直停在出发地。
	mapRetryAt int64
}

type loginAttempt struct{ name, pass string }

func newApp(cfg config) *app {
	cmds := make(chan netCmd, 64)
	evs := make(chan netEvent, 2048)
	a := &app{
		st:   newAppState(),
		ui:   newUIState(cfg.Server),
		cfg:  cfg,
		cmds: cmds,
		evs:  evs,
	}
	a.ui.login.name = cfg.LastName
	go runNet(cmds, evs)
	return a
}

func (a *app) send(c netCmd) {
	select {
	case a.cmds <- c:
	default: // 队列满说明网络侧卡住；丢一条动作好过卡住界面
	}
}

func (a *app) act(action string, data any) {
	a.send(netCmd{kind: cmdAct, action: action, data: data})
}

func (a *app) online() bool { return a.st.conn.kind == connOnline }

func (a *app) autoLogin(name, pass string) {
	a.ui.login.name, a.ui.login.pass = name, pass
	a.pendingLogin = &loginAttempt{name: name, pass: pass}
}

func (a *app) autoCreate(name string) { a.pendingCreate = name }

func (a *app) autoMove(i int) { a.pendingMove = &i }

// autoExplore 进主界面后自动历练 n 次（间隔 1.5s），联调时用来撞战斗与奇遇。
func (a *app) autoExplore(n int) { a.exploreLeft = n }

func (a *app) startPage(name string) {
	p := pageGame
	switch name {
	case "character":
		p = pageCharacter
	case "bag":
		p = pageBag
	case "skills":
		p = pageSkills
	case "social":
		p = pageSocial
	case "market":
		p = pageMarket
	case "sect":
		p = pageSect
	case "map":
		p = pageMap
	case "settings":
		p = pageSettings
	}
	a.ui.page = p
	a.preferredPage = &p
}

// drain 把已到达的网络事件全部消化掉（最多 512 条，避免饿死界面）。
func (a *app) drain() {
	for i := 0; i < 512; i++ {
		select {
		case ev := <-a.evs:
			a.handleEvent(ev)
		default:
			return
		}
	}
}

func (a *app) handleEvent(ev netEvent) {
	switch e := ev.(type) {
	case evStatus:
		prev := a.st.conn
		a.st.conn = e.connStatus
		switch e.kind {
		case connOnline:
			a.ui.login.loading = false
			if a.ui.view == viewLogin && a.st.hasChar {
				a.ui.view = viewMain
			}
		case connOffline:
			a.ui.login.loading = false
			a.ui.toast(e.detail, true)
			if a.ui.view != viewFatal {
				a.ui.view = viewLogin
			}
			a.st.clearSession()
		case connFatal:
			a.ui.login.loading = false
			a.ui.fatal = e.detail
			a.ui.view = viewFatal
		case connBackoff:
			if a.ui.view == viewMain && prev.kind != connBackoff {
				a.ui.toast("连接中断，正在重连…", true)
			}
		}
	case evReady:
		a.st.applyReady(e.r)
		a.ui.login.loading = false
		if e.r.hasCharacter() {
			a.ui.view = viewMain
			a.ui.page = pageGame
			if a.preferredPage != nil {
				a.ui.page = *a.preferredPage
				a.preferredPage = nil
			}
			a.ui.selectedExit = 0
			a.act(actWorldMap, map[string]any{})
		} else {
			a.ui.view = viewCreate
			if name := a.pendingCreate; name != "" {
				a.pendingCreate = ""
				a.act(actCreate, map[string]any{"name": name})
			}
		}
	case evState:
		// 战斗开始/奇遇出现之后服务端也会补一条 state，这里不能清战斗盒与弹层：
		// 战斗结束靠 over=true 的 battle 帧，奇遇靠玩家选完主动收起。
		moved := a.st.player != nil && a.st.player.Loc != e.p.Loc
		a.st.applyState(e.p)
		if moved {
			a.act(actWorldMap, map[string]any{})
		}
	case evVital:
		a.st.applyVital(e.v)
	case evLogs:
		a.st.pushLogs(e.lines)
		a.logBack = 0
	case evMap:
		oldLoc := ""
		if a.st.world != nil {
			oldLoc = a.st.world.Loc
		}
		a.st.applyMap(e.m)
		// 只有真的换了节点才把选中项归零：`look` 的应答、定时补拉的同一节点帧
		// 都不该把玩家刚选好的去处打回第一项。
		if e.m.Loc != oldLoc {
			a.ui.selectedExit = 0
		}
		if a.pendingMove != nil {
			i := *a.pendingMove
			a.pendingMove = nil
			if i >= 0 && i < len(e.m.Exits) && e.m.Exits[i].OK {
				a.act(actMove, map[string]any{"to": e.m.Exits[i].To})
			}
		}
	case evChat:
		line := fmt.Sprintf("[%s] %s%s：%s",
			chatChannelLabel(e.c.Channel),
			prefixIf(e.c.NPC, "【旁白】"),
			e.c.From, e.c.Text)
		a.st.pushLogs([]logLine{{K: "chat", S: truncateChars(line, 512)}})
		a.st.pushChat(e.c)
		a.logBack = 0
	case evPresence:
		a.st.presence = e.n
	case evErr:
		a.ui.login.loading = false
		// 建号竞态：create 刚落库时若先到的 worldMap 被服务端判成"还没角色"，
		// 不是用户的错，紧跟其后的 ready/state 会把画面补齐。
		if e.code == errNoChar {
			return
		}
		msg := fmt.Sprintf("%s（%s）", e.msg, e.code)
		a.ui.toast(msg, true)
		if a.ui.view == viewLogin {
			a.ui.login.err = e.msg
		}
	case evAnnounce:
		a.st.pushLogs([]logLine{{K: "sys", S: "【公告】" + e.text}})
	case evEvent:
		a.st.encounter = e.e
	case evBattle:
		a.st.battle = e.b
		if e.b.Over {
			// 结算帧只带 id/over/won（没有敌方与我方），记一行结果后收掉战斗盒。
			a.st.pushLogs([]logLine{{K: "lvl", S: battleResultText(e.b)}})
			a.st.battle = nil
		}
		if a.ui.target < 1 {
			a.ui.target = 1
		}
	}
}

func prefixIf(cond bool, s string) string {
	if cond {
		return s
	}
	return ""
}

func (a *app) onKey(k key) {
	// Ctrl+C 是控制字节 0x03，parseKeys 产出 name="c" ctrl=true。
	if k.kind == keySpecial && k.ctrl && k.name == "c" {
		a.quit = true
		return
	}
	switch a.ui.view {
	case viewLogin:
		a.keyLogin(k)
	case viewCreate:
		a.keyCreate(k)
	case viewFatal:
		if k.is("q") || k.is("esc") || k.is("enter") || k.isRune('q') {
			a.quit = true
		}
	case viewMain:
		a.keyMain(k)
	}
}

func (a *app) keyLogin(k key) {
	if k.kind == keySpecial && k.ctrl && k.name == "r" {
		a.ui.login.register = !a.ui.login.register
		a.ui.login.err = ""
		return
	}
	switch {
	case k.is("tab"), k.is("down"):
		a.ui.login.focus = (a.ui.login.focus + 1) % 3
	case k.is("up"):
		a.ui.login.focus = (a.ui.login.focus + 2) % 3
	case k.is("enter"):
		a.submitLogin()
	case k.is("backspace"):
		a.ui.backspace()
	case k.kind == keyRune:
		a.ui.appendRune(k.r)
	}
}

func (a *app) submitLogin() {
	server := a.ui.login.server
	name := a.ui.login.name
	pass := a.ui.login.pass
	if server == "" || name == "" || len([]rune(pass)) < 6 {
		a.ui.login.loading = false
		a.ui.login.err = "服务器、道号、密码（≥6 位）都要填"
		return
	}
	a.cfg.Server = server
	a.cfg.LastName = name
	a.cfg.save()
	a.submitAt = nowMs()
	a.ui.login.loading = true
	a.ui.login.err = ""
	a.send(netCmd{kind: cmdLogin, server: server, name: name, pass: pass, register: a.ui.login.register})
}

func (a *app) keyCreate(k key) {
	switch {
	case k.is("enter"):
		name := a.ui.createName
		n := len([]rune(name))
		if n < 2 || n > 8 {
			a.ui.toast("道号长度需在 2–8 个字符之间", true)
			return
		}
		a.submitAt = nowMs()
		a.act(actCreate, map[string]any{"name": name})
	case k.is("backspace"):
		a.ui.backspace()
	case k.kind == keyRune:
		a.ui.appendRune(k.r)
	}
}

func (a *app) keyMain(k key) {
	if a.ui.typing {
		switch {
		case k.is("enter"):
			text := a.ui.chatText
			if text != "" {
				to := ""
				if a.ui.chatCh == "dm" {
					to = a.ui.chatTo
				}
				if a.online() {
					a.send(netCmd{kind: cmdChat, ch: a.ui.chatCh, to: to, text: text})
				}
			}
			a.ui.chatText = ""
			a.ui.typing = false
			a.logBack = 0
		case k.is("esc"):
			a.ui.chatText = ""
			a.ui.typing = false
		case k.is("tab"):
			a.ui.cycleChannel()
		case k.is("backspace"):
			a.ui.backspace()
		case k.kind == keyRune:
			a.ui.appendRune(k.r)
		}
		return
	}

	if e := a.st.encounter; e != nil && a.ui.page == pageGame {
		a.keyEncounter(k, e)
		return
	}

	if b := a.st.battle; b != nil && a.ui.page == pageGame {
		a.keyBattle(k, b)
		return
	}

	if k.kind == keySpecial {
		if p, ok := pageFromFn(k.name); ok {
			a.ui.page = p
			return
		}
	}

	// 数据页只认"返回 / 退出 / 说话 / 翻见闻"，不要让 ↑↓ Enter 去改主界面的选中项。
	if a.ui.page != pageGame {
		switch {
		case k.is("esc"):
			a.ui.page = pageGame
		case k.isRune('q'):
			a.quit = true
		case k.isRune('/'):
			a.ui.typing = true
			a.ui.chatText = ""
		case k.is("pgup"), k.is("pgdn"), k.is("home"), k.is("end"):
			a.scrollLogs(k)
		}
		return
	}

	switch {
	case k.is("esc"):
		if a.ui.page != pageGame {
			a.ui.page = pageGame
		}
	case k.isRune('q'):
		a.quit = true
	case k.isRune('/'):
		a.ui.typing = true
		a.ui.chatText = ""
	case k.is("space"):
		a.explore()
	case k.isRune('c'):
		a.intentCultivate(!a.st.vitals.Cultivating)
	case k.isRune('b'):
		a.intentBreakthrough()
	case k.isRune('l'):
		a.intentLook()
	case k.isRune('o'):
		a.intentLogout()
	case k.is("up"), k.isRune('k'):
		if n := a.exitCount(); n > 0 {
			a.ui.selectedExit = (a.ui.selectedExit + n - 1) % n
		}
	case k.is("down"), k.isRune('j'):
		if n := a.exitCount(); n > 0 {
			a.ui.selectedExit = (a.ui.selectedExit + 1) % n
		}
	case k.is("enter"):
		if m := a.st.world; m != nil && a.ui.selectedExit < len(m.Exits) {
			ex := m.Exits[a.ui.selectedExit]
			if ex.OK && a.online() {
				a.act(actMove, map[string]any{"to": ex.To})
			}
		}
	case k.is("pgup"), k.is("pgdn"), k.is("home"), k.is("end"):
		a.scrollLogs(k)
	}
}

// scrollLogs 翻见闻：PgUp 往上（看更早），PgDn 回最新，Home 跳到最早，End 回底。
func (a *app) scrollLogs(k key) {
	switch {
	case k.is("pgup"):
		if n := a.st.logs.len() - 1; n > 0 {
			a.logBack = min(a.logBack+5, n)
		}
	case k.is("pgdn"):
		a.logBack = max(0, a.logBack-5)
	case k.is("home"):
		if n := a.st.logs.len() - 1; n > 0 {
			a.logBack = n
		}
	case k.is("end"):
		a.logBack = 0
	}
}

func (a *app) exitCount() int {
	if a.st.world == nil {
		return 0
	}
	return len(a.st.world.Exits)
}

func (a *app) explore() {
	if !a.online() {
		return
	}
	region := ""
	if a.st.world != nil {
		region = a.st.world.Region
	}
	a.act(actExplore, map[string]any{"region": region})
}

func (a *app) intentCultivate(on bool) {
	if a.online() {
		a.act(actCultivate, map[string]any{"on": on})
	}
}

func (a *app) intentBreakthrough() {
	if a.online() {
		a.act(actBreakthrough, map[string]any{})
	}
}

func (a *app) intentLook() {
	if a.online() {
		a.act(actLook, map[string]any{})
	}
}

func (a *app) intentLogout() {
	a.send(netCmd{kind: cmdLogout})
	a.st.clearSession()
	a.ui.view = viewLogin
	a.ui.login.loading = false
	a.ui.page = pageGame
	a.ui.typing = false
}

// tick 处理与时间有关的事：舆图补拉、登录超时、toast 过期、待办登录。
func (a *app) tick() {
	a.refreshStaleMap()
	a.tickAutoExplore()
	if a.ui.login.loading && a.submitAt > 0 && nowMs()-a.submitAt > 12_000 {
		a.ui.login.loading = false
		a.ui.login.err = "服务端没有响应，请检查地址与网络"
	}
	a.ui.pruneToasts()
	if a.pendingLogin != nil && a.ui.view == viewLogin && !a.ui.login.loading {
		att := a.pendingLogin
		a.pendingLogin = nil
		a.ui.login.loading = true
		a.ui.login.err = ""
		a.submitAt = nowMs()
		a.send(netCmd{kind: cmdLogin, server: a.cfg.Server, name: att.name, pass: att.pass})
	}
}

// refreshStaleMap 在"角色位置与舆图不一致"时补拉 worldMap。
// 赶路途中服务端会以 E-BUSY 拒掉，所以按 600ms 节流重试，直到对齐。
func (a *app) refreshStaleMap() {
	if !a.online() || !a.st.hasChar || a.st.world == nil || a.st.player == nil {
		return
	}
	if a.st.world.Loc == a.st.player.Loc {
		return
	}
	if nowMs() < a.mapRetryAt {
		return
	}
	a.mapRetryAt = nowMs() + 600
	a.act(actWorldMap, map[string]any{})
}

func (a *app) shutdown() {
	a.send(netCmd{kind: cmdShutdown})
}

// keyBattle 战斗中的按键：a 普攻、s 技能、f 逃、1–9 选目标。
func (a *app) keyBattle(k key, b *battleView) {
	switch {
	case k.is("pgup"), k.is("pgdn"), k.is("home"), k.is("end"):
		a.scrollLogs(k)
	case k.kind == keyRune && k.r >= '1' && k.r <= '9':
		n := int(k.r - '0')
		if a.foeByIndex(n) != nil {
			a.ui.target = n
		}
	case k.isRune('a'), k.is("space"), k.is("enter"):
		a.act(actBattle, map[string]any{"op": "attack", "target": a.ui.target})
	case k.isRune('s'):
		if id := a.firstSkill(); id != "" {
			a.act(actBattle, map[string]any{"op": "skill", "id": id, "target": a.ui.target})
		} else {
			a.ui.toast("还没装配技能，先按 a 普攻", true)
		}
	case k.isRune('f'), k.is("esc"):
		if b.CanFlee {
			a.act(actBattle, map[string]any{"op": "flee"})
		} else {
			a.ui.toast("这一战逃不掉", true)
		}
	}
}

// foeByIndex 按 battle.foes[].i 找敌方；找不到返回 nil。
func (a *app) foeByIndex(i int) *foeView {
	if a.st.battle == nil {
		return nil
	}
	for idx := range a.st.battle.Foes {
		got := a.st.battle.Foes[idx].I
		if got == 0 {
			// 服务端个别战斗帧没填 i，按序号兜底。
			got = idx + 1
		}
		if got == i {
			return &a.st.battle.Foes[idx]
		}
	}
	return nil
}

// firstSkill 返回已装配的第一个技能 id（没有就空串）。
func (a *app) firstSkill() string {
	if a.st.player == nil {
		return ""
	}
	for _, s := range a.st.player.SkillBook {
		if s.Equipped {
			return s.ID
		}
	}
	return ""
}

func battleResultText(b *battleView) string {
	switch {
	case b.Won:
		return "战斗胜利。"
	case b.Fled:
		return "你脱离了战斗。"
	case b.Over:
		return "战斗结束。"
	default:
		return "战斗继续。"
	}
}

// keyEncounter 奇遇三选一：只认数字键，选完立刻收起弹层（服务端还会回 state）。
func (a *app) keyEncounter(k key, e *eventView) {
	if k.kind != keyRune || k.r < '1' || k.r > '9' {
		return
	}
	i := int(k.r-'0') - 1
	if i >= len(e.Options) {
		return
	}
	a.act(actEncounter, map[string]any{"key": e.Options[i].Key})
	a.st.encounter = nil
}

// tickAutoExplore 自动历练：只在主界面、无战斗、无奇遇时发，间隔 1.5 秒。
func (a *app) tickAutoExplore() {
	if a.exploreLeft <= 0 || !a.online() || a.st.player == nil {
		return
	}
	if a.st.battle != nil || a.st.encounter != nil {
		return
	}
	if nowMs()-a.exploreAt < 1500 {
		return
	}
	a.exploreAt = nowMs()
	a.exploreLeft--
	a.explore()
}
