package main

import (
	"fmt"
	"strings"
)

// 界面绘制：一屏一屏整屏重画。布局固定七段，终端再小也先保场景与见闻。
//
//  顶栏 1 ｜ 场景 N ｜ 目标 2 ｜ 路标 1 ｜ 见闻 M ｜ 行动 2 ｜ 提示 1

type span struct {
	text string
	st   style
}

func spansWidth(sp []span) int {
	n := 0
	for _, s := range sp {
		n += textWidth(s.text)
	}
	return n
}

// putSpans 从 x 开始依次写，返回结束列。
func putSpans(sc *screen, x, y int, sp []span) int {
	for _, s := range sp {
		x = sc.put(x, y, s.text, s.st)
	}
	return x
}

func (a *app) render(w, h int) *screen {
	var sc *screen
	if a.scr != nil && a.scr.w == w && a.scr.h == h {
		sc = a.scr
		sc.reset()
	} else {
		sc = newScreen(w, h)
		a.scr = sc
	}
	switch a.ui.view {
	case viewLogin:
		drawLogin(sc, &a.ui)
	case viewCreate:
		drawCreate(sc, &a.ui)
	case viewFatal:
		drawFatal(sc, &a.ui)
	case viewMain:
		if a.ui.page == pageGame {
			a.drawGame(sc)
		} else {
			a.drawPage(sc)
		}
	}
	drawToast(sc, &a.ui)
	return sc
}

// ---------------- 登录 / 建号 / 致命 ----------------

func drawLogin(sc *screen, ui *uiState) {
	w, h := sc.w, sc.h
	boxW, boxH := 62, 14
	x0 := (w - boxW) / 2
	y0 := (h - boxH) / 2
	if x0 < 1 {
		x0 = 1
	}
	if y0 < 0 {
		y0 = 0
	}
	l := &ui.login
	y := y0
	sc.put(x0, y, "", fg(colText))
	y++
	sc.put(x0, y, "九 州 问 道", fgBold(colAccent))
	y++
	sc.put(x0, y, "云端存档 · 服务端权威", fg(colDim))
	y++
	y++
	fieldRow(sc, x0, y, "服务器", l.server, l.focus == 0, "ws://127.0.0.1:8080")
	y++
	fieldRow(sc, x0, y, "道号", l.name, l.focus == 1, "2–8 个字符")
	y++
	fieldRow(sc, x0, y, "密码", maskPass(l.pass), l.focus == 2, "6–32 个字符")
	y++
	y++

	mode := "登录"
	if l.register {
		mode = "注册"
	}
	x := sc.put(x0, y, "  [ "+mode+" ]  ", fgBg(colBG, colAccent))
	x = sc.put(x, y, "  Ctrl+R 切登录/注册", fg(colFaint))
	if l.loading {
		sc.put(x, y, "   连接中…", fg(colWarn))
	}
	y++
	y++
	if l.err != "" {
		sc.put(x0, y, "  ! "+truncateWidth(l.err, max(8, w-x0-4)), fg(colBad))
	} else {
		sc.put(x0, y, "  Tab 切换  Enter 提交  Ctrl+C 退出", fg(colFaint))
	}
}

func fieldRow(sc *screen, x0, y int, label, value string, focused bool, hint string) {
	lblSt := fg(colDim)
	if focused {
		lblSt = fg(colAccent)
	}
	x := sc.put(x0, y, "  "+padWidth(label, 6), lblSt)
	cursor := ""
	if focused {
		cursor = "▌"
	}
	x = sc.put(x, y, "["+cursor, fg(colFaint))
	st := fg(colText)
	if value == "" {
		st = fg(colFaint)
		value = hint
	}
	x = sc.put(x, y, value, st)
	sc.put(x, y, "]", fg(colFaint))
}

func drawCreate(sc *screen, ui *uiState) {
	w, h := sc.w, sc.h
	x0 := (w - 56) / 2
	y0 := (h - 10) / 2
	if x0 < 1 {
		x0 = 1
	}
	if y0 < 0 {
		y0 = 0
	}
	y := y0 + 1
	sc.put(x0, y, "踏 入 凡 界", fgBold(colAccent))
	y++
	sc.put(x0, y, "测灵石摆在案上，还空着。", fg(colDim))
	y += 2
	fieldRow(sc, x0, y, "道号", ui.createName, true, "2–8 个字符")
	y += 2
	sc.put(x0, y, "  Enter 落笔  Ctrl+C 退出", fg(colFaint))
}

func drawFatal(sc *screen, ui *uiState) {
	w, h := sc.w, sc.h
	x0 := (w - 64) / 2
	y0 := (h - 8) / 2
	if x0 < 1 {
		x0 = 1
	}
	if y0 < 0 {
		y0 = 0
	}
	y := y0 + 1
	sc.put(x0, y, "无法进入游戏", fgBold(colBad))
	y += 2
	sc.put(x0, y, "  "+truncateWidth(ui.fatal, max(8, w-x0-4)), fg(colText))
	y++
	sc.put(x0, y, "  本机协议 v"+itoa(protoVersion), fg(colFaint))
	y += 2
	sc.put(x0, y, "  q 退出", fg(colFaint))
}

// ---------------- 主界面 ----------------

func (a *app) drawGame(sc *screen) {
	h := sc.h
	logH := 7
	if h < 22 {
		logH = 5
	}
	if h < 18 {
		logH = 3
	}
	sceneH := h - 7 - logH
	if sceneH < 4 {
		sceneH = 4
		logH = max(1, h-7-sceneH)
	}

	y := 0
	a.drawTop(sc, y)
	y++
	a.drawSceneArea(sc, y, sceneH)
	y += sceneH
	a.drawObjective(sc, y)
	y += 2
	a.drawExits(sc, y)
	y++
	a.drawLogs(sc, y, logH)
	y += logH
	a.drawActions(sc, y)
	y += 2
	a.drawHint(sc, y)
}

func (a *app) drawTop(sc *screen, y int) {
	w := sc.w
	place, region := "雾中", ""
	if m := a.st.world; m != nil {
		place, region = m.Name, m.RegionName
		if place == "" {
			place = "无名之地"
		}
	}
	var left []span
	left = append(left, span{" " + place, fgBold(colText)})
	if region != "" {
		left = append(left, span{" " + region, fg(colFaint)})
	}
	if m := a.st.world; m != nil {
		var bits []string
		if s := seasonName(deref(m.Season)); s != "" {
			bits = append(bits, s)
		}
		if s := weatherName(deref(m.Weather)); s != "" {
			bits = append(bits, s)
		}
		if len(bits) > 0 {
			left = append(left, span{"   " + strings.Join(bits, "·"), fg(colDim)})
		}
	}
	cc, ct := a.st.conn.badge()
	left = append(left, span{"   ● " + ct, fg(cc)})
	left = append(left, span{fmt.Sprintf("  在线 %d", a.st.presence), fg(colFaint)})

	right := ""
	if p := a.st.player; p != nil {
		right = fmt.Sprintf("%s · %s   灵石 %s   功德 %d ", p.Name, p.Realm, thousands(p.Stones), p.Merit)
	}
	lw := spansWidth(left)
	room := w - lw - 1
	if room < 0 {
		// 顶栏挤爆时先砍右侧，再砍左侧。
		right = ""
		room = 0
	}
	if textWidth(right) > room {
		right = truncateWidth(right, room)
	}
	x := putSpans(sc, 0, y, left)
	pad := w - x - textWidth(right)
	if pad < 0 {
		pad = 0
	}
	sc.put(x+pad, y, right, fg(colDim))
}

func (a *app) drawSceneArea(sc *screen, y, h int) {
	terrain, weather, daypart := "plain", "", ""
	if m := a.st.world; m != nil {
		if m.Terrain != "" {
			terrain = m.Terrain
		}
		weather = deref(m.Weather)
		daypart = deref(m.Daypart)
	}
	p := poseStanding
	if a.st.vitals.Cultivating {
		p = poseSitting
	} else if a.st.player != nil && a.st.player.Travel != nil {
		p = poseWalking
	}
	drawScene(sc, 0, y, sc.w, h, sceneSpec{terrain: terrain, weather: weather, daypart: daypart, pose: p})
}

func (a *app) drawObjective(sc *screen, y int) {
	title, line := a.objective()
	sc.put(0, y, " > "+title, fg(colGold))
	sc.put(2, y+1, truncateWidth(line, sc.w-4), fg(colText))
}

func (a *app) objective() (string, string) {
	if p := a.st.player; p != nil && p.Travel != nil {
		return "赶路中", fmt.Sprintf("前往 %s · 剩 %s", p.Travel.Name, fmtCountdown(p.Travel.ArriveAt-a.st.serverNow()))
	}
	if a.st.vitals.Cultivating {
		return "打坐中", "真气在慢慢回，按 c 起身"
	}
	if a.st.vitals.Stamina < 30 {
		return "体力不济", "坐下调息，或回城歇脚"
	}
	explored := 0
	if m := a.st.world; m != nil {
		explored = m.Explored
	}
	if explored < 3 {
		return "摸清这条路", "按 ↑↓ 选路标，Enter 就走"
	}
	return "寻个去处", "去没走过的地方看看"
}

func (a *app) drawExits(sc *screen, y int) {
	m := a.st.world
	if m == nil {
		return
	}
	traveling := a.st.player != nil && a.st.player.Travel != nil
	x := sc.put(0, y, " 路 ", fg(colGold))
	for i, ex := range m.Exits {
		arrow := ""
		if m.X != nil && m.Y != nil && ex.X != nil && ex.Y != nil {
			arrow = bearingArrow(*ex.X-*m.X, *ex.Y-*m.Y)
		}
		label := ""
		if ex.Name != nil && *ex.Name != "" {
			label = fmt.Sprintf("%s%s %ds", arrow, truncateChars(*ex.Name, 5), ex.CostSec)
		} else {
			label = fmt.Sprintf("%s%s %ds", arrow, terrainWord(ex.Terrain), ex.CostSec)
		}
		chip := "[" + label + "]"
		cw := textWidth(chip) + 1
		if x+cw > sc.w-2 {
			sc.put(x, y, "…", fg(colFaint))
			break
		}
		ok := ex.OK && !traveling
		switch {
		case i == a.ui.selectedExit && ok:
			x = sc.put(x, y, chip, fgBg(colBG, colAccent))
		case ok:
			x = sc.put(x, y, chip, fg(colText))
		default:
			x = sc.put(x, y, chip, fg(colFaint))
		}
		sc.put(x, y, " ", fg(colText))
		x++
	}
}

func (a *app) drawLogs(sc *screen, y, rows int) {
	n := a.st.logs.len()
	end := n - a.logBack
	if end < 0 {
		end = 0
	}
	if end > n {
		end = n
	}
	start := end - rows
	if start < 0 {
		start = 0
	}
	for i := 0; i < rows; i++ {
		idx := start + i
		if idx >= end {
			break
		}
		e, ok := a.st.logs.get(idx)
		if !ok {
			break
		}
		c := tone(e.line.K)
		text := truncateChars(e.line.S, sc.w-4)
		x := sc.put(0, y+i, " • ", fg(c))
		sc.put(x, y+i, text, fg(c))
	}
}

func (a *app) drawActions(sc *screen, y int) {
	busy := !a.online()
	dim := fg(colText)
	if busy {
		dim = fg(colFaint)
	}
	hot := fgBold(colAccent)
	explore := hot
	if busy {
		explore = dim
	}
	cultLabel := "打坐"
	if a.st.vitals.Cultivating {
		cultLabel = "收功"
	}
	x := sc.put(2, y, "[ 历练 Space ]", explore)
	x = sc.put(x, y, " [ "+cultLabel+" C ]", dim)
	x = sc.put(x, y, " [ 突破 B ]", dim)
	sc.put(x, y, " [ 看看 L ]", dim)

	p := a.st.player
	maxHP, maxMP, staminaMax := int64(1), int64(1), int64(200)
	if p != nil {
		maxHP = maxi(p.MaxHP, 1)
		maxMP = maxi(p.MaxMP, 1)
		if p.StaminaMax > 0 {
			staminaMax = p.StaminaMax
		}
	}
	v := a.st.vitals
	y2 := y + 1
	x = sc.put(2, y2, fmt.Sprintf("气血%s %d/%d", bar(v.HP, maxHP, 10), v.HP, maxHP), fg(colBad))
	x = sc.put(x, y2, fmt.Sprintf("  真气%s %d/%d", bar(v.MP, maxMP, 10), v.MP, maxMP), fg(colAccent))
	x = sc.put(x, y2, fmt.Sprintf("  修为%s %d/%d", bar(v.Exp, maxi(v.ExpNeed, 1), 10), v.Exp, v.ExpNeed), fg(colGold))
	staSt := fg(colDim)
	if percent(v.Stamina, staminaMax) < 0.2 {
		staSt = fg(colBad)
	}
	sc.put(x, y2, fmt.Sprintf("  体力 %d/%d", v.Stamina, staminaMax), staSt)
}

func (a *app) drawHint(sc *screen, y int) {
	if a.ui.typing {
		sc.put(0, y, a.ui.chatInputHint(), fg(colAccent))
		tail := "   Enter 送出  Tab 换频道  Esc 取消"
		sc.put(textWidth(a.ui.chatInputHint()), y, tail, fg(colFaint))
		return
	}
	sc.put(0, y, " / 说话   ↑↓ 选路  Enter 前往   Space 历练   c 打坐   b 突破   PgUp 见闻   F1–F8 页面   q 退出", fg(colFaint))
}

// ---------------- toast ----------------

func drawToast(sc *screen, ui *uiState) {
	t, ok := ui.activeToast()
	if !ok || sc.h < 3 {
		return
	}
	w := min(sc.w, textWidth(t.text)+6)
	x0 := sc.w - w
	st := style{fg: colAccent, bg: colPanel, hasBG: true}
	if t.bad {
		st.fg = colBad
	}
	sc.fill(x0, 1, w, 1, ' ', st)
	sc.put(x0, 1, " ! "+truncateWidth(t.text, w-4), st)
}

// ---------------- 小工具 ----------------

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func maxi(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func terrainWord(t string) string {
	switch t {
	case "water", "river", "lake":
		return "水"
	case "woods", "forest", "bamboo":
		return "林"
	case "slope", "cliff":
		return "坡"
	case "town", "city", "gate":
		return "镇"
	case "plain":
		return "原"
	default:
		return "路"
	}
}

func seasonName(s string) string {
	switch s {
	case "spring":
		return "春"
	case "summer":
		return "夏"
	case "autumn":
		return "秋"
	case "winter":
		return "冬"
	default:
		return s
	}
}

func weatherName(s string) string {
	switch s {
	case "clear":
		return "晴"
	case "rain":
		return "雨"
	case "fog":
		return "雾"
	case "wind":
		return "风"
	case "snow":
		return "雪"
	case "frost":
		return "霜"
	case "hail":
		return "冰雹"
	case "sandstorm":
		return "沙尘"
	case "storm":
		return "雷暴"
	default:
		return s
	}
}

func kindName(k string) string {
	switch k {
	case "pill", "dan":
		return "丹药"
	case "weapon":
		return "法器"
	case "armor":
		return "护甲"
	case "boots":
		return "鞋履"
	case "accessory":
		return "饰物"
	case "herb", "material":
		return "材料"
	case "manual":
		return "功法"
	case "token":
		return "信物"
	case "food":
		return "食物"
	default:
		return k
	}
}
