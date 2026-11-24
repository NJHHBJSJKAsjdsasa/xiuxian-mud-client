package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

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

	// 由启动器拉起时，退出到登录窗口需要重新打开的原生启动器路径。
	launcherPath string
	// 服务端踢人公告里的理由，close 事件到达后带给启动器提示框。
	kickReason string

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

	// 桌面窗口就绪后由 GUI 注入；TUI 里为 nil。
	applyWindowMode func(borderless bool)
}

type loginAttempt struct{ name, pass string }

func newApp(cfg config) *app {
	cmds := make(chan netCmd, 64)
	evs := make(chan netEvent, 2048)
	a := &app{
		st:           newAppState(),
		ui:           newUIState(cfg.Server),
		cfg:          cfg,
		cmds:         cmds,
		evs:          evs,
		launcherPath: relauncherPath(),
	}
	a.ui.login.name = cfg.LastName
	if a.ui.login.name == "" {
		a.ui.login.focus = 1
	} else {
		a.ui.login.focus = 2 // 有上次道号时直接落到密码，少按一次 Tab
	}
	a.ui.login.syncCursors()
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
	a.ui.login.syncCursors()
	a.pendingLogin = &loginAttempt{name: name, pass: pass}
}

// autoToken 用启动器换来的 token 直接连；密码不落到命令行。
func (a *app) autoToken(server, token string) {
	a.ui.login.server = server
	a.ui.login.loading = true
	a.ui.login.syncCursors()
	a.send(netCmd{kind: cmdLogin, server: server, token: token})
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
	case evClosed:
		a.ui.login.loading = false
		a.ui.quitMenu = false
		a.ui.help = false
		a.ui.talkPick = false
		a.st.clearSession()
		kickReason := ""
		notice := sessionEndNotice(e.code, e.reason, e.msg)
		if e.code == 4003 && e.reason == "banned" {
			kickReason = strings.TrimSpace(a.kickReason)
			if kickReason == "" {
				kickReason = "管理员未填写理由"
			}
			notice = "你已被移出此界：" + kickReason
		}
		if a.returnToLauncher(notice, kickReason) {
			return
		}
		a.ui.fatal = notice
		a.ui.view = viewFatal
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
			a.ui.createCur = len([]rune(a.ui.createName))
			if name := a.pendingCreate; name != "" {
				a.pendingCreate = ""
				a.act(actCreate, map[string]any{"name": name})
			}
		}
	case evState:
		a.ensurePageSelection()
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
		a.addLogs(e.lines)
	case evChat:
		line := fmt.Sprintf("[%s] %s%s：%s",
			chatChannelLabel(e.c.Channel),
			prefixIf(e.c.NPC, "【旁白】"),
			e.c.From, e.c.Text)
		a.addLogs([]logLine{{K: "chat", S: truncateChars(line, 512)}})
		a.st.pushChat(e.c)
		// 收到私语就把回复对象记下来，/r 才有地方去。
		if e.c.Channel == "dm" && !e.c.NPC && e.c.From != "" {
			a.ui.chatTo = e.c.From
		}
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
			a.smartExitSelection()
			// 换了节点就重新居中：还留着上一个节点的平移会让人找不到自己。
			a.ui.mapPanX, a.ui.mapPanY = 0, 0
		}
		if a.pendingMove != nil {
			i := *a.pendingMove
			a.pendingMove = nil
			if i >= 0 && i < len(e.m.Exits) && e.m.Exits[i].OK {
				a.act(actMove, map[string]any{"to": e.m.Exits[i].To})
			}
		}
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
		a.addLogs([]logLine{{K: "sys", S: "【公告】" + e.text}})
		if strings.HasPrefix(e.text, "你已被移出此界：") {
			a.kickReason = strings.TrimSpace(strings.TrimPrefix(e.text, "你已被移出此界："))
		}
	case evEvent:
		a.st.encounter = e.e
		// 奇遇是模态：帮助与交谈弹层让位，免得盖住选项。
		a.ui.help = false
		a.ui.talkPick = false
	case evBattle:
		old := a.st.battle
		a.st.battle = e.b
		// 开打了就收起交谈/帮助，别让弹层盖住战况。
		a.ui.talkPick = false
		a.ui.help = false
		if e.b.Over {
			// 结算帧只带 id/over/won（没有敌方与我方），记一行结果后收掉战斗盒。
			a.addLogs([]logLine{{K: "lvl", S: battleResultText(e.b)}})
			a.st.battle = nil
		} else if old == nil || old.ID != e.b.ID {
			// 新一场战斗：上一场选中的 2/3 号不能带进来（新战斗可能只有一只）。
			a.ui.target = firstAliveFoeTarget(e.b)
		} else if f := a.foeByIndex(a.ui.target); f == nil || f.Dead {
			a.ui.target = firstAliveFoeTarget(e.b)
		}
	}
}

// addLogs 追加日志，并保住玩家的"上翻位置"：
// 新到 N 行就让离底距离也多 N 行，屏幕上原本看的那几行不会动。
// 贴底时（logBack=0）保持贴底，新行照常出现在最下面。
func (a *app) addLogs(lines []logLine) {
	if a.logBack > 0 {
		a.logBack += len(lines)
	}
	a.st.pushLogs(lines)
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
		if k.is("esc") || k.is("enter") || k.isRune('q') {
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
		a.ui.login.setFocus((a.ui.login.focus + 1) % 3)
	case k.is("up"):
		a.ui.login.setFocus((a.ui.login.focus + 2) % 3)
	case k.is("enter"):
		a.submitLogin()
	case k.is("left"):
		a.ui.editMove(-1)
	case k.is("right"):
		a.ui.editMove(1)
	case k.is("home"):
		a.ui.editHome()
	case k.is("end"):
		a.ui.editEnd()
	case k.is("delete"):
		a.ui.editDelete()
	case k.is("backspace"):
		a.ui.backspace()
	case k.is("esc"):
		a.quit = true
	case k.kind == keyRune:
		a.ui.appendRune(k.r)
	}
}

func (a *app) submitLogin() {
	if a.ui.login.loading {
		return
	}
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
	a.kickReason = ""
	a.submitAt = nowMs()
	a.ui.login.loading = true
	a.ui.login.err = ""
	a.send(netCmd{kind: cmdLogin, server: server, name: name, pass: pass, register: a.ui.login.register})
}

func (a *app) keyCreate(k key) {
	switch {
	case k.is("esc"):
		// 建号页按 Esc 退回登录页：原本只能 Ctrl+C 整只客户端退出。
		a.ui.view = viewLogin
	case k.is("enter"):
		name := a.ui.createName
		n := len([]rune(name))
		if n < 2 || n > 8 {
			a.ui.toast("道号长度需在 2–8 个字符之间", true)
			return
		}
		a.submitAt = nowMs()
		a.act(actCreate, map[string]any{"name": name})
	case k.is("left"):
		a.ui.editMove(-1)
	case k.is("right"):
		a.ui.editMove(1)
	case k.is("home"):
		a.ui.editHome()
	case k.is("end"):
		a.ui.editEnd()
	case k.is("delete"):
		a.ui.editDelete()
	case k.is("backspace"):
		a.ui.backspace()
	case k.kind == keyRune:
		a.ui.appendRune(k.r)
	}
}

func (a *app) keyMain(k key) {
	if a.ui.typing {
		a.keyChat(k)
		return
	}
	if a.ui.quitMenu {
		a.keyQuitMenu(k)
		return
	}
	// 奇遇是模态：只认数字键，别让别的操作把它顶掉。
	if e := a.st.encounter; e != nil && a.ui.page == pageGame {
		a.keyEncounter(k, e)
		return
	}
	if a.ui.talkPick && a.ui.page == pageGame {
		a.keyTalk(k)
		return
	}

	// 帮助浮层是全局的：? 开关，Esc / Enter 也能收起。
	if k.isRune('?') {
		a.ui.help = !a.ui.help
		a.ui.confirm = ""
		return
	}
	if a.ui.help {
		if k.is("esc") || k.is("enter") || k.isRune('q') {
			a.ui.help = false
		}
		return
	}

	// q 打开离开菜单，再选退出游戏或回登录界面。
	if k.isRune('q') {
		a.ui.quitMenu = true
		a.ui.quitSel = 0
		a.ui.confirm = ""
		return
	}
	// 按了别的键就撤销待确认（o 的判据按键分：一律清空会让第二次 o
	// 先抹掉自己刚立的确认，永远退不出去）。
	if a.ui.confirm == "logout" && !k.isRune('o') {
		a.ui.confirm = ""
	}

	if b := a.st.battle; b != nil && a.ui.page == pageGame {
		a.keyBattle(k, b)
		return
	}

	if k.kind == keySpecial {
		if p, ok := pageFromFn(k.name); ok {
			a.ui.page = p
			a.ensurePageSelection()
			return
		}
	}
	switch {
	case k.isRune(']'):
		a.cyclePage(1)
		a.ensurePageSelection()
		return
	case k.isRune('['):
		a.cyclePage(-1)
		a.ensurePageSelection()
		return
	}

	if a.ui.page != pageGame {
		if k.is("esc") {
			a.ui.page = pageGame
			return
		}
		a.keyPage(k)
		return
	}

	switch {
	case k.isRune('/'):
		a.startChat()
	case k.is("space"):
		a.explore()
	case k.isRune('c'):
		a.intentCultivate(!a.st.vitals.Cultivating)
	case k.isRune('b'):
		a.intentBreakthrough()
	case k.isRune('l'):
		a.intentLook()
	case k.isRune('t'):
		a.openTalk()
	case k.isRune('o'):
		a.confirmTwice("logout", "再按一次 o 退出登录", func() {
			if !a.returnToLauncher("", "") {
				a.intentLogout()
			}
		})
	case k.is("up"), k.isRune('k'):
		a.moveExit(-1)
	case k.is("down"), k.isRune('j'):
		a.moveExit(1)
	case k.is("enter"):
		a.enterExit()
	case k.is("pgup"), k.is("pgdn"), k.is("home"), k.is("end"):
		a.scrollLogs(k)
	}
}

// openTalk 打开"与人物交谈"的弹层。节点上没人就直说，不弹空框。
func (a *app) openTalk() {
	if a.st.player != nil && a.st.player.Travel != nil {
		a.ui.toast("正在赶路，到了再聊", true)
		return
	}
	if len(a.npcList()) == 0 {
		a.ui.toast("此间没有别人", true)
		return
	}
	a.ui.talkPick = true
	a.ui.talkSel = 0
}

func (a *app) npcList() []npcView {
	if a.st.world == nil {
		return nil
	}
	return a.st.world.NPCs
}

// keyTalk 交谈弹层：↑↓ 选、Enter 或数字确认、Esc 取消。
func (a *app) keyTalk(k key) {
	list := a.npcList()
	switch {
	case k.is("esc"):
		a.ui.talkPick = false
	case k.is("up"), k.isRune('k'):
		if n := len(list); n > 0 {
			a.ui.talkSel = (a.ui.talkSel + n - 1) % n
		}
	case k.is("down"), k.isRune('j'):
		if n := len(list); n > 0 {
			a.ui.talkSel = (a.ui.talkSel + 1) % n
		}
	case k.is("enter"):
		a.talkTo(a.ui.talkSel)
	case k.kind == keyRune && digitOf(k.r) > 0:
		a.talkTo(digitOf(k.r) - 1)
	}
}

func (a *app) talkTo(i int) {
	list := a.npcList()
	if i < 0 || i >= len(list) {
		return
	}
	npc := list[i]
	a.ui.talkPick = false
	if !a.online() {
		return
	}
	a.act(actTalk, map[string]any{"npc": npc.ID})
}

func (a *app) keyQuitMenu(k key) {
	switch {
	case k.is("esc"), k.isRune('q'), k.isRune('3'):
		a.ui.quitMenu = false
	case k.is("up"), k.isRune('k'):
		a.ui.quitSel = (a.ui.quitSel + 2) % 3
	case k.is("down"), k.isRune('j'), k.is("tab"):
		a.ui.quitSel = (a.ui.quitSel + 1) % 3
	case k.is("enter"), k.is("space"):
		a.chooseQuitMenu()
	case k.isRune('1'):
		a.ui.quitSel = 0
		a.chooseQuitMenu()
	case k.isRune('2'):
		a.ui.quitSel = 1
		a.chooseQuitMenu()
	}
}

func (a *app) chooseQuitMenu() {
	switch a.ui.quitSel {
	case 0:
		a.ui.quitMenu = false
		a.quit = true
	case 1:
		if a.returnToLauncher("", "") {
			return
		}
		a.intentLogout()
	default:
		a.ui.quitMenu = false
	}
}

func sessionEndNotice(code int, reason, fallback string) string {
	switch {
	case code == 4003 && reason == "banned":
		return "你已被移出此界，游戏已关闭。"
	case code == 4001:
		return "账号在别处登录，游戏已关闭。"
	case code == 4003:
		return "登录已过期，请重新登录。"
	case code == 4004:
		return "客户端与服务端版本不兼容，请更新客户端。"
	default:
		return fallback
	}
}

func relauncherPath() string {
	path := strings.TrimSpace(os.Getenv("XIUXIAN_LAUNCHER"))
	if path == "" {
		return ""
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return ""
	}
	return path
}

// returnToLauncher 关闭游戏并重新拉起独立启动器；直接运行游戏时返回 false，
// 由调用方回退到游戏内登录页。
func (a *app) returnToLauncher(notice, kickReason string) bool {
	if a.launcherPath == "" {
		return false
	}
	cmd := exec.Command(a.launcherPath)
	cmd.Dir = filepath.Dir(a.launcherPath)
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "XIUXIAN_TOKEN=") ||
			strings.HasPrefix(kv, "XIUXIAN_SERVER=") ||
			strings.HasPrefix(kv, "XIUXIAN_LAUNCHER=") ||
			strings.HasPrefix(kv, "XIUXIAN_NOTICE=") {
			continue
		}
		env = append(env, kv)
	}
	if notice != "" {
		env = append(env, "XIUXIAN_NOTICE="+notice)
	}
	if kickReason != "" {
		env = append(env, "XIUXIAN_KICK_REASON="+kickReason)
	}
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return false
	}
	_ = cmd.Process.Release()
	a.send(netCmd{kind: cmdLogout})
	a.st.clearSession()
	a.ui.quitMenu = false
	a.ui.quitSel = 0
	a.quit = true
	return true
}

// confirmTwice 两段确认：第一次只提示，2.5 秒内再按一次才执行。
func (a *app) confirmTwice(kind, hint string, do func()) {
	if a.ui.confirm == kind && nowMs()-a.ui.confirmAt < 2500 {
		a.ui.confirm = ""
		do()
		return
	}
	a.ui.confirm, a.ui.confirmAt = kind, nowMs()
	a.ui.toast(hint, false)
}

// keyChat 输入行：Enter 送出、Tab 换频道、↑↓ 翻历史、Esc 取消。
func (a *app) keyChat(k key) {
	switch {
	case k.is("enter"):
		a.sendChatInput()
	case k.is("esc"):
		a.ui.chatText = ""
		a.ui.chatCur = 0
		a.ui.typing = false
	case k.is("tab"):
		a.ui.cycleChannel()
	case k.is("up"):
		a.ui.histStep(-1)
	case k.is("down"):
		a.ui.histStep(1)
	case k.is("space"):
		// 空格在 parseKeys 里是特殊键，不处理就等于"聊天打不出空格"，
		// `mail list`、`排行 修为` 这类带参数的指令全都发不出去。
		a.ui.appendRune(' ')
	case k.is("left"):
		a.ui.editMove(-1)
	case k.is("right"):
		a.ui.editMove(1)
	case k.is("home"):
		a.ui.editHome()
	case k.is("end"):
		a.ui.editEnd()
	case k.is("delete"):
		a.ui.editDelete()
	case k.is("backspace"):
		a.ui.backspace()
	case k.kind == keyRune:
		a.ui.appendRune(k.r)
	}
}

func (a *app) startChat() {
	a.ui.typing = true
	a.ui.chatText = ""
	a.ui.chatCur = 0
	a.ui.inputAt = len(a.ui.inputHist)
	a.ui.histDraft = ""
	a.ui.histDraftAt = 0
}

// sendChatInput 解析输入行：普通发言、/w 私语、@道号 私语、/r 回复、/help 帮助。
// 认不出的 / 开头只提示，不当成普通发言发出去——免得把命令喊给全服看。
func (a *app) sendChatInput() {
	raw := strings.TrimSpace(a.ui.chatText)
	if raw == "" {
		a.ui.chatText = ""
		a.ui.chatCur = 0
		a.ui.typing = false
		return
	}
	// 中文输入法常把斜杠与 at 打成全角，这里统一半角。
	raw = strings.NewReplacer("／", "/", "＠", "@").Replace(raw)
	if raw == "/help" || raw == "/?" {
		a.ui.chatText = ""
		a.ui.chatCur = 0
		a.ui.typing = false
		a.ui.help = true
		return
	}
	if !a.online() {
		// 保留输入与输入态：重连后按 Enter 还能再发，不必重敲一遍。
		a.ui.toast("未连接，消息未发送；重连后再按 Enter", true)
		return
	}
	a.ui.chatText = ""
	a.ui.chatCur = 0
	a.ui.typing = false

	switch {
	case strings.HasPrefix(raw, "//"):
		// 双斜杠强制当普通发言：想喊一句"任务"又不触发指令时用。
		a.sendChannel(raw[2:])
		return
	case strings.HasPrefix(raw, "/w "), strings.HasPrefix(raw, "/W "):
		name, text := splitFirst(strings.TrimSpace(raw[3:]))
		if name == "" || text == "" {
			a.ui.toast("用法：/w 道号 内容", true)
			return
		}
		a.sendWhisper(name, text)
		return
	case strings.HasPrefix(raw, "@"):
		name, text := splitFirst(strings.TrimSpace(raw[1:]))
		if name == "" || text == "" {
			a.ui.toast("用法：@道号 内容", true)
			return
		}
		a.sendWhisper(name, text)
		return
	case strings.HasPrefix(raw, "/r "):
		text := strings.TrimSpace(raw[3:])
		if a.ui.chatTo == "" {
			a.ui.toast("还没有私语对象，先 /w 道号 内容", true)
			return
		}
		if text == "" {
			a.ui.toast("用法：/r 内容", true)
			return
		}
		a.sendWhisper(a.ui.chatTo, text)
		return
	case strings.HasPrefix(raw, "/"):
		// 其余斜杠命令交给服务端的文本指令通道（/任务、/排行 修为、/学 功法…）。
		// 客户端不另抄一份动作映射，服务端改了也不会两边对不上。
		a.sendCmdText(strings.TrimSpace(raw[1:]))
		return
	}

	// 命令动词开头（任务、排行 修为、学 太玄剑典…）也走文本指令：
	// MUD 的习惯就是直接敲，不必先加斜杠。
	if isServerVerb(raw) {
		a.sendCmdText(raw)
		return
	}

	a.sendChannel(raw)
}

// sendChannel 普通发言：发到当前频道（私语需要先有对象）。
func (a *app) sendChannel(text string) {
	if text == "" {
		return
	}
	a.ui.rememberInput(text)
	if a.ui.chatCh == "dm" && a.ui.chatTo == "" {
		a.ui.toast("私语要先指定对象：/w 道号 内容", true)
		return
	}
	if !a.online() {
		return
	}
	to := ""
	if a.ui.chatCh == "dm" {
		to = a.ui.chatTo
	}
	a.send(netCmd{kind: cmdChat, ch: a.ui.chatCh, to: to, text: text})
}

// sendCmdText 走服务端的 t=cmd 文本指令通道。参数校验、冷却与错误码都在服务端，
// 客户端只把整句送过去——这样 30 多条指令不必在客户端再实现一遍。
func (a *app) sendCmdText(text string) {
	if text == "" {
		return
	}
	a.ui.rememberInput("/" + text)
	if !a.online() {
		return
	}
	a.send(netCmd{kind: cmdText, text: text})
}

// serverVerbs 与服务端 internal/game/cmd.go 的动词表一致（12-API §5.3 由它生成）。
// 只用来判断"这句话像不像指令"，猜错最多多一条服务端的提示，不会丢消息。
var serverVerbs = map[string]bool{
	"势力": true, "门派": true, "faction": true,
	"加入": true, "入伙": true,
	"捐献": true, "捐赠": true, "兑换": true,
	"退出势力": true, "退出宗门": true,
	"帮助": true, "help": true, "?": true,
	"状态": true, "面板": true,
	"打坐": true, "修炼": true, "停": true, "收功": true, "停止": true,
	"历练": true, "探索": true, "突破": true, "冲阶": true,
	"用": true, "服": true, "吃": true,
	"装备": true, "卸下": true, "买": true, "卖": true,
	"炼": true, "炼丹": true,
	"学": true, "参悟": true, "主修": true, "加点": true,
	"聊天": true, "说": true,
	"排行": true, "榜": true, "任务": true,
	"切磋": true, "生死斗": true, "接受": true, "应战": true, "拒绝": true,
	"挂": true, "挂单": true, "摊": true, "逛": true, "集市": true,
	"买摊": true, "撤摊": true, "选": true,
	"mail": true, "邮件": true,
}

// isServerVerb 看首个词是不是服务端指令动词。
func isServerVerb(text string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false
	}
	return serverVerbs[fields[0]]
}

func (a *app) sendWhisper(name, text string) {
	a.ui.rememberInput("/w " + name + " " + text)
	a.ui.chatTo = name
	a.ui.chatCh = "dm"
	if !a.online() {
		return
	}
	a.send(netCmd{kind: cmdChat, ch: "dm", to: name, text: text})
}

// splitFirst 按第一段空白切开：`沈砚 在吗` → (`沈砚`, `在吗`)。
// 全角空格也当分隔符，中文输入法下常打出来。
func splitFirst(s string) (string, string) {
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '\u3000' {
			return s[:i], strings.TrimSpace(s[i:])
		}
	}
	return s, ""
}

// keyPage 数据页按键：列表光标、使用/装备/装配/买入、刷新舆图。
func (a *app) keyPage(k key) {
	if k.isRune('/') {
		a.startChat()
		return
	}
	switch a.ui.page {
	case pageBag:
		if a.navList(k, len(a.bagItems())) {
			return
		}
		switch {
		case k.is("enter"), k.is("space"), k.isRune('u'):
			a.useSelectedBag(false)
		case k.isRune('e'):
			a.useSelectedBag(true)
		case k.isRune('s'), k.isRune('x'):
			a.sellSelectedBag()
		}
	case pageSkills:
		if a.navList(k, len(a.skillList())) {
			return
		}
		if k.is("enter") || k.is("space") || k.isRune('e') {
			a.toggleSelectedSkill()
		}
	case pageMarket:
		if a.navList(k, len(a.shopList())) {
			return
		}
		switch {
		case k.isRune('+'), k.isRune('='), k.is("right"):
			a.bumpQty(1)
		case k.isRune('-'), k.isRune('_'), k.is("left"):
			a.bumpQty(-1)
		case k.is("enter"), k.isRune('b'):
			a.buySelected()
		}
	case pageMap:
		switch {
		case k.is("enter"):
			if a.online() {
				a.act(actWorldMap, map[string]any{})
			}
		case k.isRune('l'):
			a.intentLook()
		case k.isRune('0'):
			a.ui.mapPanX, a.ui.mapPanY = 0, 0
		}
	case pageSettings:
		if k.is("space") || k.is("enter") || k.isRune('w') {
			a.toggleWindowFrame()
		}
	}
}

func (a *app) toggleWindowFrame() {
	if a.cfg.borderless() {
		a.cfg.WindowMode = windowModeWindowed
	} else {
		a.cfg.WindowMode = windowModeBorderless
	}
	a.cfg.save()
	if a.applyWindowMode == nil {
		a.ui.toast("窗口设置已保存：桌面窗口下次启动生效", false)
		return
	}
	a.applyWindowMode(a.cfg.borderless())
	if a.cfg.borderless() {
		a.ui.toast("窗口边框已关闭：只留游戏界面", false)
	} else {
		a.ui.toast("窗口边框已开启：可拖动与缩放", false)
	}
}

// firstOKExit 返回第一条可走出口；全被挡住时返回 0 给"看原因"。
func (a *app) firstOKExit() int {
	if a.st.world == nil {
		return 0
	}
	for i, ex := range a.st.world.Exits {
		if ex.OK {
			return i
		}
	}
	return 0
}

// smartExitSelection 让默认选中落在可走的路线上，而不是第一条断路。
func (a *app) smartExitSelection() {
	if a.st.world == nil || len(a.st.world.Exits) == 0 {
		a.ui.selectedExit = 0
		return
	}
	sel := a.ui.selectedExit
	if sel < 0 || sel >= len(a.st.world.Exits) || !a.st.world.Exits[sel].OK {
		a.ui.selectedExit = a.firstOKExit()
	}
}

// affordableShop 判断当前货物按 1 件是否买得起、阶位够不够。
func (a *app) affordableShop(i int) bool {
	items := a.shopList()
	p := a.st.player
	if p == nil || i < 0 || i >= len(items) {
		return false
	}
	it := items[i]
	return p.Stage >= it.Min && p.Stones >= it.Price
}

// ensurePageSelection 在进入/刷新数据页时夹住光标；坊市优先落到买得起的货上。
func (a *app) ensurePageSelection() {
	switch a.ui.page {
	case pageBag:
		if n := len(a.bagItems()); n > 0 {
			a.ui.setSel(pageBag, clampInt(a.ui.selOf(pageBag), 0, n-1))
		}
	case pageSkills:
		if n := len(a.skillList()); n > 0 {
			a.ui.setSel(pageSkills, clampInt(a.ui.selOf(pageSkills), 0, n-1))
		}
	case pageMarket:
		items := a.shopList()
		if len(items) == 0 {
			a.ui.setSel(pageMarket, 0)
			return
		}
		sel := clampInt(a.ui.selOf(pageMarket), 0, len(items)-1)
		if !a.affordableShop(sel) {
			for i := range items {
				if a.affordableShop(i) {
					sel = i
					break
				}
			}
		}
		a.ui.setSel(pageMarket, sel)
	}
}

// mouseClick 处理窗口点击：单击选中，双击执行（对话框/按钮单击即执行）。
func (a *app) mouseClick(col, row int, activate bool) {
	if a.scr == nil {
		return
	}
	act, ok := a.scr.hitAt(col, row)
	if !ok {
		return
	}
	switch act.kind {
	case hitHelpClose:
		if !activate {
			a.ui.help = false
		}
	case hitLoginField:
		if a.ui.view == viewLogin && act.index >= 0 && act.index < 3 {
			a.ui.login.setFocus(act.index)
		}
	case hitLoginButton:
		if !activate && a.ui.view == viewLogin {
			a.submitLogin()
		}
	case hitRegisterToggle:
		if !activate && a.ui.view == viewLogin {
			a.ui.login.register = !a.ui.login.register
			a.ui.login.err = ""
		}
	case hitCreateField:
		if a.ui.view == viewCreate {
			a.ui.createCur = len([]rune(a.ui.createName))
		}
	case hitEncounter:
		if !activate {
			a.chooseEncounter(act.index)
		}
	case hitTalk:
		if !activate && a.ui.talkPick {
			a.talkTo(act.index)
		}
	case hitExit:
		if a.ui.view == viewMain && a.ui.page == pageGame && a.st.battle == nil &&
			a.st.world != nil && act.index >= 0 && act.index < len(a.st.world.Exits) {
			a.ui.selectedExit = act.index
			if activate {
				a.enterExit()
			}
		}
	case hitBagItem:
		if a.ui.page == pageBag && act.index >= 0 && act.index < len(a.bagItems()) {
			a.ui.setSel(pageBag, act.index)
			if activate {
				a.useSelectedBag(false)
			}
		}
	case hitSkillItem:
		if a.ui.page == pageSkills && act.index >= 0 && act.index < len(a.skillList()) {
			a.ui.setSel(pageSkills, act.index)
			if activate {
				a.toggleSelectedSkill()
			}
		}
	case hitShopItem:
		if a.ui.page == pageMarket && act.index >= 0 && act.index < len(a.shopList()) {
			a.ui.setSel(pageMarket, act.index)
			if activate {
				a.buySelected()
			}
		}
	case hitFoe:
		if a.st.battle != nil {
			if f := a.foeByIndex(act.index); f != nil && !f.Dead {
				a.ui.target = act.index
				if activate {
					a.battleAct(a.st.battle, map[string]any{"op": "attack", "target": act.index})
				}
			}
		}
	case hitWindowToggle:
		if !activate {
			a.toggleWindowFrame()
		}
	case hitQuitOption:
		if !activate && act.index >= 0 && act.index < 3 {
			a.ui.quitSel = act.index
			a.chooseQuitMenu()
		}
	case hitQuitCancel:
		if !activate {
			a.ui.quitMenu = false
		}
	}
}

// navList 处理列表通用移动键；认识这个键并处理了返回 true。
func (a *app) navList(k key, n int) bool {
	if n <= 0 {
		return false
	}
	p := a.ui.page
	sel := a.ui.selOf(p)
	switch {
	case k.is("up"), k.isRune('k'):
		sel--
	case k.is("down"), k.isRune('j'):
		sel++
	case k.is("pgup"):
		sel -= 10
	case k.is("pgdn"):
		sel += 10
	case k.is("home"):
		sel = 0
	case k.is("end"):
		sel = n - 1
	default:
		return false
	}
	a.ui.setSel(p, clampInt(sel, 0, n-1))
	return true
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---------- 背包 ----------

func (a *app) bagItems() []bagItem {
	if a.st.player == nil {
		return nil
	}
	return a.st.player.Bag
}

func (a *app) bagSel() (bagItem, bool) {
	items := a.bagItems()
	i := a.ui.selOf(pageBag)
	if i < 0 || i >= len(items) {
		return bagItem{}, false
	}
	return items[i], true
}

// useSelectedBag 对选中物品动手：已装备的卸下，装备类穿上，其余按"用"处理。
// forceEquip 对应 e 键——明确要穿，不像 Enter 那样猜。
func (a *app) useSelectedBag(forceEquip bool) {
	it, ok := a.bagSel()
	if !ok {
		return
	}
	if !a.online() {
		return
	}
	if it.Equipped && it.Slot != "" {
		// 已装备的再按 Enter 就是卸下：不然"s 出售"提示"先卸下"却没有卸下的键。
		// e 键是"明确要穿"，对着已穿的东西就只提示，不该反过来脱掉。
		if forceEquip {
			a.ui.toast("这件已经穿着了；要脱按 Enter", false)
			return
		}
		a.act(actUnequip, map[string]any{"slot": it.Slot})
		return
	}
	if isEquipKind(it.Kind) {
		a.act(actEquip, map[string]any{"item": it.ID})
		return
	}
	if forceEquip {
		a.ui.toast("这件不是装备，穿不上", true)
		return
	}
	a.act(actUse, map[string]any{"item": it.ID, "n": 1})
}

// sellSelectedBag 卖一件。已装备的卖不掉（服务端会拒），这里先拦一下。
func (a *app) sellSelectedBag() {
	it, ok := a.bagSel()
	if !ok {
		return
	}
	if it.Equipped {
		a.ui.toast("先卸下再卖（选中后按 Enter）", true)
		return
	}
	if it.Sell <= 0 {
		a.ui.toast("这件卖不出手", true)
		return
	}
	if !a.online() {
		return
	}
	a.act(actSell, map[string]any{"item": it.ID, "n": 1})
}

func isEquipKind(kind string) bool {
	switch kind {
	case "weapon", "armor", "boots", "accessory":
		return true
	}
	return false
}

// ---------- 技能 ----------

func (a *app) skillList() []skillItem {
	if a.st.player == nil {
		return nil
	}
	return a.st.player.SkillBook
}

// skillSlotCap 与 content.SlotCount（N-SKL-02）同规则：阶 0/5/10/13 各解锁一槽。
// 只用于界面提示，能不能装以服务端 setSkills 的判定为准。
func skillSlotCap(stage int) int {
	switch {
	case stage >= 13:
		return 4
	case stage >= 10:
		return 3
	case stage >= 5:
		return 2
	default:
		return 1
	}
}

// equippedSkillIDs 取已装配技能，按槽位顺序。优先用 state.slots，
// 老服务端没下发时退回 skillBook 的 equipped/slot 字段。
func equippedSkillIDs(p *playerView) []string {
	if p == nil {
		return nil
	}
	if len(p.Slots) > 0 {
		return append([]string(nil), p.Slots...)
	}
	type pair struct {
		slot int
		id   string
	}
	var ps []pair
	for _, s := range p.SkillBook {
		if s.Equipped {
			ps = append(ps, pair{s.Slot, s.ID})
		}
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].slot < ps[j].slot })
	out := make([]string, 0, len(ps))
	for _, x := range ps {
		out = append(out, x.id)
	}
	return out
}

// toggleSelectedSkill 装配 / 卸下选中技能：setSkills 要的是完整名单。
func (a *app) toggleSelectedSkill() {
	p := a.st.player
	list := a.skillList()
	i := a.ui.selOf(pageSkills)
	if p == nil || i < 0 || i >= len(list) {
		return
	}
	it := list[i]
	ids := equippedSkillIDs(p)
	next := make([]string, 0, len(ids)+1)
	for _, id := range ids {
		if id != it.ID {
			next = append(next, id)
		}
	}
	if len(next) == len(ids) {
		// 原来没装：加进去，先看槽够不够。
		if limit := skillSlotCap(p.Stage); len(ids) >= limit {
			a.ui.toast(fmt.Sprintf("技能槽已满（%d 个），先卸下一个", limit), true)
			return
		}
		next = append(next, it.ID)
	}
	if !a.online() {
		return
	}
	a.act(actSetSkills, map[string]any{"ids": next})
}

// ---------- 坊市 ----------

func (a *app) shopList() []shopItem {
	if a.st.catalog == nil {
		return nil
	}
	return a.st.catalog.Shop
}

func (a *app) bumpQty(d int) {
	q := a.ui.shopQty + d
	if q < 1 {
		q = 1
	}
	if q > 99 {
		q = 99
	}
	a.ui.shopQty = q
}

func (a *app) buySelected() {
	items := a.shopList()
	i := a.ui.selOf(pageMarket)
	if i < 0 || i >= len(items) {
		return
	}
	it := items[i]
	p := a.st.player
	qty := a.ui.shopQty
	if p == nil {
		return
	}
	if p.Stage < it.Min {
		a.ui.toast(fmt.Sprintf("%s 要 %d 阶才买得到", it.Name, it.Min), true)
		return
	}
	if cost := it.Price * int64(qty); cost > p.Stones {
		a.ui.toast(fmt.Sprintf("灵石不够：要 %s，身上 %s", thousands(cost), thousands(p.Stones)), true)
		return
	}
	if !a.online() {
		return
	}
	a.act(actBuy, map[string]any{"item": it.ID, "n": qty})
}

// ---------- 页与去处 ----------

var pageOrder = []page{
	pageCharacter, pageBag, pageSkills, pageSocial,
	pageMarket, pageSect, pageMap, pageSettings,
}

// cyclePage 前后翻数据页；在主界面按 ] 进第一页、[ 进最后一页。
func (a *app) cyclePage(dir int) {
	cur := -1
	for i, p := range pageOrder {
		if p == a.ui.page {
			cur = i
		}
	}
	n := len(pageOrder)
	if cur < 0 {
		if dir > 0 {
			a.ui.page = pageOrder[0]
		} else {
			a.ui.page = pageOrder[n-1]
		}
		return
	}
	a.ui.page = pageOrder[(cur+dir+n)%n]
}

func (a *app) moveExit(d int) {
	n := a.exitCount()
	if n == 0 {
		return
	}
	a.ui.selectedExit = (a.ui.selectedExit + d + n) % n
}

// enterExit 前往选中出口。走不了要说清楚为什么——
// 之前按 Enter 没反应，玩家只会以为客户端卡了。
func (a *app) enterExit() {
	if !a.online() {
		a.ui.toast("未连接，无法赶路；正在重连…", false)
		return
	}
	if a.st.player != nil && a.st.player.Travel != nil {
		a.ui.toast("正在赶路，到了再说", true)
		return
	}
	m := a.st.world
	if m == nil || len(m.Exits) == 0 {
		a.ui.toast("这里没有路可走", true)
		return
	}
	sel := a.ui.selectedExit
	if sel < 0 || sel >= len(m.Exits) {
		sel = 0
	}
	ex := m.Exits[sel]
	if !ex.OK {
		a.ui.toast(exitReason(ex), true)
		return
	}
	if !a.online() {
		a.ui.toast("还没连上，先等等", true)
		return
	}
	a.act(actMove, map[string]any{"to": ex.To})
}

// exitReason 把服务端出口字段翻成人话。cond 是代码（fly / water / blocked:<id>），
// 直接印出来玩家看不懂。
func exitReason(ex exitView) string {
	if ex.Blocked != nil && *ex.Blocked {
		if ex.Until != nil && *ex.Until > nowMs() {
			return "这条路封着，还要 " + fmtCountdown(*ex.Until-nowMs())
		}
		return "这条路封着"
	}
	if ex.Cond != nil {
		c := *ex.Cond
		switch {
		case strings.HasPrefix(c, "blocked:"):
			return "这条路封着"
		case c == "fly":
			return "要会御空才过得去"
		case c == "water":
			return "要会泅水才过得去"
		case c == "owner":
			return "要此地主人才能过"
		case strings.HasPrefix(c, "realm:"):
			return "要 " + strings.TrimPrefix(c, "realm:") + " 阶才过得去"
		case strings.HasPrefix(c, "item:"):
			return "身上要有指定的物件"
		case strings.HasPrefix(c, "token:"):
			return "要有对应的信物"
		case strings.HasPrefix(c, "manual:"):
			return "要学过对应的功法"
		case strings.HasPrefix(c, "time:"):
			return "这条路过时辰才开"
		case c != "":
			return "过不去（" + c + "）"
		}
	}
	if ex.Fee != nil && *ex.Fee > 0 {
		return fmt.Sprintf("要交 %d 灵石才放行", *ex.Fee)
	}
	return "此路不通"
}

func (a *app) exitCount() int {
	if a.st.world == nil {
		return 0
	}
	return len(a.st.world.Exits)
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
	a.ui.help = false
	a.ui.quitMenu = false
	a.ui.quitSel = 0
	a.ui.chatTo = ""
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

// keyBattle 战斗中的按键：a 普攻、s 技能、i 丹药、f 逃；↑↓/jk、←→、Tab、1–9 选目标。
func (a *app) keyBattle(k key, b *battleView) {
	switch {
	case k.is("pgup"), k.is("pgdn"), k.is("home"), k.is("end"):
		a.scrollLogs(k)
	case k.is("up"), k.is("left"), k.isRune('k'):
		a.cycleFoe(-1)
	case k.is("down"), k.is("right"), k.is("tab"), k.isRune('j'):
		a.cycleFoe(1)
	case k.kind == keyRune && digitOf(k.r) > 0:
		n := digitOf(k.r)
		if f := a.foeByIndex(n); f != nil && !f.Dead {
			a.ui.target = n
		} else if f != nil {
			a.ui.toast("这一只已经倒下了", false)
		}
	case k.isRune('a'), k.is("space"), k.is("enter"):
		a.battleAct(b, map[string]any{"op": "attack", "target": a.ui.target})
	case k.isRune('s'):
		if id := a.firstSkill(); id != "" {
			a.battleAct(b, map[string]any{"op": "skill", "id": id, "target": a.ui.target})
		} else {
			a.ui.toast("还没装配技能，先按 a 普攻", true)
		}
	case k.isRune('i'):
		if it, ok := a.healPill(); ok {
			a.battleAct(b, map[string]any{"op": "item", "id": it.ID})
		} else {
			a.ui.toast("行囊里没有丹药", true)
		}
	case k.isRune('f'):
		if b.CanFlee {
			a.act(actBattle, map[string]any{"op": "flee"})
		} else {
			a.ui.toast("这一战逃不掉", true)
		}
	case k.is("esc"):
		// Esc 不替你逃：打 Boss 时手滑一次就白打一场。
		a.ui.toast("逃跑按 f；Esc 不替你逃", false)
	}
}

// battleAct 轮到自己才发动作；否则只提示，免得白吃一条 E-BUSY。
func (a *app) battleAct(b *battleView, data map[string]any) {
	if !a.online() {
		a.ui.toast("未连接，无法出手；正在重连…", false)
		return
	}
	if b.State != "" && b.State != "player" {
		a.ui.toast("等对方出手…", false)
		return
	}
	a.act(actBattle, data)
}

// cycleFoe 在活着的敌人之间前后切换；全倒了就退化成全体，至少能选。
func (a *app) cycleFoe(d int) {
	b := a.st.battle
	if b == nil {
		return
	}
	list := b.Foes
	if len(list) == 0 && b.Enemy != nil {
		list = []foeView{*b.Enemy}
	}
	if len(list) == 0 {
		return
	}
	idxOf := func(i int) int {
		if list[i].I == 0 {
			return i + 1
		}
		return list[i].I
	}
	var ids []int
	for i := range list {
		if !list[i].Dead {
			ids = append(ids, idxOf(i))
		}
	}
	if len(ids) == 0 {
		for i := range list {
			ids = append(ids, idxOf(i))
		}
	}
	pos := -1
	for i, n := range ids {
		if n == a.ui.target {
			pos = i
		}
	}
	if pos < 0 {
		a.ui.target = ids[0]
		return
	}
	a.ui.target = ids[(pos+d+len(ids))%len(ids)]
}

// firstAliveFoeTarget 返回第一只活着的敌方序号；全倒或单怪旧帧时回 1。
func firstAliveFoeTarget(b *battleView) int {
	if b == nil || len(b.Foes) == 0 {
		return 1
	}
	for i, f := range b.Foes {
		if f.Dead {
			continue
		}
		if f.I > 0 {
			return f.I
		}
		return i + 1
	}
	return 1
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

// healPill 找一颗回复类丹药。货架说明里带"气血/真气"的优先——
// 凝神丹、破境丹这类在战斗里吃了纯属浪费；找不到再用第一颗丹药兜底。
func (a *app) healPill() (bagItem, bool) {
	desc := map[string]string{}
	for _, s := range a.shopList() {
		desc[s.ID] = s.Desc
	}
	var fallback bagItem
	hasFallback := false
	for _, it := range a.bagItems() {
		if it.Kind != "pill" && it.Kind != "dan" {
			continue
		}
		d := desc[it.ID]
		if strings.Contains(d, "气血") || strings.Contains(d, "真气") {
			return it, true
		}
		if !hasFallback {
			fallback, hasFallback = it, true
		}
	}
	return fallback, hasFallback
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
	if k.kind != keyRune || digitOf(k.r) == 0 {
		return
	}
	a.chooseEncounter(digitOf(k.r) - 1)
}

func (a *app) chooseEncounter(i int) {
	e := a.st.encounter
	if e == nil || i < 0 || i >= len(e.Options) {
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
	if a.st.battle != nil || a.st.encounter != nil || a.ui.talkPick || a.ui.help {
		return
	}
	if nowMs()-a.exploreAt < 1500 {
		return
	}
	a.exploreAt = nowMs()
	a.exploreLeft--
	a.explore()
}

// digitOf 把 '1'–'9' 与全角 '１'–'９' 都认成数字；其余返回 0。
func digitOf(r rune) int {
	switch {
	case r >= '1' && r <= '9':
		return int(r - '0')
	case r >= '１' && r <= '９':
		return int(r - '０')
	}
	return 0
}
