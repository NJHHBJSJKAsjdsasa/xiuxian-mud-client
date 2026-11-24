package main

import (
	"fmt"
	"strings"
)

// 界面：盒子化仪表盘。
//
// 一屏 = 一条标题栏 + 状态盒 + 去处盒 + 见闻盒 + 目标行 + 按键行。
// 不再画字符场景：终端里信息密度优先，山水留给 Rust 参考实现。

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
	if a.st.encounter != nil && a.ui.view == viewMain {
		drawEncounter(sc, a.st.encounter)
	}
	if a.ui.talkPick && a.ui.view == viewMain && a.st.encounter == nil {
		drawTalk(sc, a.st.world, a.ui.talkSel)
	}
	if a.ui.help && a.ui.view == viewMain && a.st.encounter == nil {
		drawHelp(sc)
		sc.addHit(0, 0, sc.w, sc.h, hitAction{hitHelpClose, 0})
	}
	drawToast(sc, &a.ui)
	if a.ui.quitMenu && a.ui.view == viewMain {
		drawQuitMenu(sc, a.ui.quitSel, a.launcherPath != "")
	}
	return sc
}

// ---------------- 登录 / 建号 / 致命 ----------------

func centeredBox(sc *screen, w, h int, title string) (int, int, int, int) {
	bw, bh := w, h
	if bw > sc.w-2 {
		bw = sc.w - 2
	}
	if bh > sc.h-2 {
		bh = sc.h - 2
	}
	x := (sc.w - bw) / 2
	y := (sc.h - bh) / 2
	box(sc, x, y, bw, bh, title, true)
	return x, y, bw, bh
}

func drawLogin(sc *screen, ui *uiState) {
	x, y, _, _ := centeredBox(sc, 52, 14, "登录")
	l := &ui.login
	ix, iw := boxInner(x, 52)
	rowY := y + 2

	sc.put(ix, rowY, "九州问道", fgBold(colAccent))
	sc.put(ix, rowY+1, "云端存档 · 服务端权威", fg(colDim))
	rowY += 3

	fieldRow(sc, ix, rowY, iw, "服务器", l.server, l.serverCur, l.focus == 0, "ws://127.0.0.1:8080")
	fieldRow(sc, ix, rowY+1, iw, "道号", l.name, l.nameCur, l.focus == 1, "2–8 个字符")
	fieldRow(sc, ix, rowY+2, iw, "密码", maskPass(l.pass), l.passCur, l.focus == 2, "6–32 个字符")
	sc.addHit(ix, rowY, iw, 1, hitAction{hitLoginField, 0})
	sc.addHit(ix, rowY+1, iw, 1, hitAction{hitLoginField, 1})
	sc.addHit(ix, rowY+2, iw, 1, hitAction{hitLoginField, 2})
	rowY += 4

	mode := "登录"
	if l.register {
		mode = "注册"
	}
	btn := fmt.Sprintf(" %s ", mode)
	bx := sc.put(ix, rowY, "[", fg(colAccent))
	bx = sc.put(bx, rowY, btn, fgBg(colBG, colAccent))
	sc.put(bx, rowY, "]", fg(colAccent))
	sc.put(bx+3, rowY, "Ctrl+R 切登录/注册", fg(colFaint))
	sc.addHit(ix, rowY, bx+1-ix, 1, hitAction{hitLoginButton, 0})
	sc.addHit(bx+3, rowY, 18, 1, hitAction{hitRegisterToggle, 0})
	if l.loading {
		sc.put(ix+20, rowY, "连接中…", fg(colWarn))
	}
	rowY += 2

	if l.err != "" {
		sc.put(ix, rowY, "! "+truncateWidth(l.err, iw-2), fg(colBad))
	} else {
		sc.put(ix, rowY, "Tab 切换   Enter 提交   Ctrl+C 退出", fg(colFaint))
	}
}

// fieldRow 一行输入框：标签占固定列，聚焦时括号与光标提亮。
// putCursorText 画一段带光标的输入文本：光标占一列，前后的字按显示宽度截断。
func putCursorText(sc *screen, x, y, avail int, text string, cur int, st style) int {
	if avail <= 0 {
		return x
	}
	rs := []rune(text)
	pos := clampPos(cur, 0, len(rs))
	before := truncateWidth(string(rs[:pos]), max(0, avail-1))
	x = sc.put(x, y, before, st)
	x = sc.put(x, y, "▌", fg(colAccent))
	left := max(0, avail-1-textWidth(before))
	if left > 0 {
		x = sc.put(x, y, truncateWidth(string(rs[pos:]), left), st)
	}
	return x
}

func fieldRow(sc *screen, x, y, w int, label, value string, cur int, focused bool, hint string) {
	lbl := fg(colDim)
	brk := fg(colFaint)
	if focused {
		lbl = fg(colAccent)
		brk = fg(colAccent)
	}
	const used = 8
	sc.put(x, y, padWidth(label, used), lbl)
	cx := sc.put(x+used, y, "[", brk)
	avail := w - used - 4
	if focused {
		if value == "" {
			end := sc.put(cx+1, y, "▌", fg(colAccent))
			end = sc.put(end, y, truncateWidth(hint, max(0, avail-1)), fg(colFaint))
			sc.put(end, y, " ]", brk)
			return
		}
		end := putCursorText(sc, cx+1, y, avail, value, cur, fg(colText))
		sc.put(end, y, " ]", brk)
		return
	}
	text, st := value, fg(colText)
	if text == "" {
		text, st = hint, fg(colFaint)
	}
	cx = sc.put(cx+1, y, truncateWidth(text, avail), st)
	sc.put(cx, y, " ]", brk)
}

func drawCreate(sc *screen, ui *uiState) {
	x, y, _, _ := centeredBox(sc, 48, 10, "建号")
	ix, iw := boxInner(x, 48)
	sc.put(ix, y+2, "踏入凡界", fgBold(colAccent))
	sc.put(ix, y+3, "测灵石摆在案上，还空着。", fg(colDim))
	fieldRow(sc, ix, y+5, iw, "道号", ui.createName, ui.createCur, true, "2–8 个字符")
	sc.addHit(ix, y+5, iw, 1, hitAction{hitCreateField, 0})
	sc.put(ix, y+7, "Enter 落笔   Ctrl+C 退出", fg(colFaint))
}

func drawFatal(sc *screen, ui *uiState) {
	x, y, _, _ := centeredBox(sc, 56, 8, "无法进入游戏")
	ix, iw := boxInner(x, 56)
	sc.put(ix, y+2, truncateWidth(ui.fatal, iw), fg(colBad))
	sc.put(ix, y+3, "本机协议 v"+itoa(protoVersion), fg(colFaint))
	sc.put(ix, y+5, "q 退出", fg(colDim))
}

// ---------------- 主界面 ----------------

func (a *app) drawGame(sc *screen) {
	w, h := sc.w, sc.h
	a.drawTopBar(sc)

	mainH := 9
	if h < 26 {
		mainH = 7
	}
	if h < 20 {
		mainH = 6
	}
	logY := 1 + mainH
	logH := h - logY - 2
	if logH < 3 {
		logH = 3
		mainH = h - 1 - 2 - logH
		if mainH < 4 {
			mainH = 4
		}
		logY = 1 + mainH
	}

	if w >= 76 {
		statusW := 34
		if w < 96 {
			statusW = 30
		}
		a.drawStatusBox(sc, 0, 1, statusW, mainH)
		if a.st.battle != nil {
			a.drawBattleBox(sc, statusW+1, 1, w-statusW-1, mainH)
		} else {
			a.drawExitsBox(sc, statusW+1, 1, w-statusW-1, mainH)
		}
	} else {
		// 窄屏：三根命脉压成一行，省下的高度全给去处。
		a.drawStatusLine(sc, 1)
		a.drawExitsBox(sc, 0, 2, w, mainH-1)
	}

	a.drawLogBox(sc, 0, logY, w, logH)
	a.drawObjective(sc, h-2)
	a.drawHint(sc, h-1)
}

func (a *app) drawTopBar(sc *screen) {
	st := style{fg: colText, bg: colPanel, hasBG: true}
	dim := style{fg: colDim, bg: colPanel, hasBG: true}
	row(sc, 0, 0, sc.w, st)

	x := sc.put(0, 0, " 九州问道 ", style{fg: colAccent, bg: colPanel, hasBG: true, bold: true})

	place, region := "雾中", ""
	if m := a.st.world; m != nil {
		place, region = m.Name, m.RegionName
		if place == "" {
			place = "无名之地"
		}
	}
	x = sc.put(x, 0, "│ ", dim)
	x = sc.put(x, 0, place, st)
	if region != "" {
		x = sc.put(x, 0, " · "+region, dim)
	}

	// 中段：季节天气 + 在线
	var bits []string
	if m := a.st.world; m != nil {
		if s := seasonName(deref(m.Season)); s != "" {
			bits = append(bits, s)
		}
		if s := weatherName(deref(m.Weather)); s != "" {
			bits = append(bits, s)
		}
	}
	if len(bits) > 0 {
		x = sc.put(x, 0, "   "+strings.Join(bits, "·"), dim)
	}
	badge, text := a.st.conn.badge()
	x = sc.put(x, 0, "   ● "+text, style{fg: badge, bg: colPanel, hasBG: true})
	sc.put(x, 0, fmt.Sprintf(" · %d 人", a.st.presence), dim)

	// 右侧：道号、境界、灵石
	if p := a.st.player; p != nil {
		right := fmt.Sprintf("%s · %s   灵石 %s ", p.Name, p.Realm, thousands(p.Stones))
		rw := textWidth(right)
		if sc.w-rw-2 > x+2 {
			sc.put(sc.w-rw-1, 0, right, st)
		}
	}
}

// drawStatusLine 窄屏专用：气血/真气/修为 三段并排，一行看完。
func (a *app) drawStatusLine(sc *screen, y int) {
	p := a.st.player
	v := a.st.vitals
	maxHP, maxMP := int64(1), int64(1)
	if p != nil {
		maxHP = maxi(p.MaxHP, 1)
		maxMP = maxi(p.MaxMP, 1)
	}
	segW := sc.w / 3
	if segW < 18 {
		return
	}
	barW := segW - 5 - 8
	if barW < 3 {
		barW = 3
	}
	draw := func(i int, label string, cur, max int64, c rgb) {
		gaugeRow(sc, i*segW, y, 5, barW, label, cur, max, c)
	}
	draw(0, "气血", v.HP, maxHP, colBad)
	draw(1, "真气", v.MP, maxMP, colAccent)
	draw(2, "修为", v.Exp, maxi(v.ExpNeed, 1), colGold)
}

func (a *app) drawStatusBox(sc *screen, x, y, w, h int) {
	box(sc, x, y, w, h, "状态", false)
	ix, iw := boxInner(x, w)
	p := a.st.player
	if p == nil || iw < 12 || h < 3 {
		return
	}
	v := a.st.vitals
	rows := h - 2

	const labelW = 8
	// 条的宽度按"标签 + 值"剩余列算：值最宽 7 列（200/200），再留 1 列间隔。
	barW := iw - labelW - 11
	if barW > 16 {
		barW = 16
	}
	if barW < 4 {
		barW = 4
	}

	r := 0
	put := func(label string, cur, max int64, c rgb) {
		if r >= rows {
			return
		}
		gaugeRow(sc, ix, y+1+r, labelW, barW, label, cur, max, c)
		r++
	}

	// 窄盒只留三条命脉，不塞身份行。
	if rows >= 5 {
		line := fmt.Sprintf("%s · 战力 %s", orDash(p.Realm), thousands(p.Power))
		kv(sc, ix, y+1, labelW, "身份", truncateWidth(line, iw-labelW), fg(colText))
		r++
	}
	put("气血", v.HP, maxi(p.MaxHP, 1), colBad)
	put("真气", v.MP, maxi(p.MaxMP, 1), colAccent)
	put("修为", v.Exp, maxi(v.ExpNeed, 1), colGold)
	if rows >= 5 {
		put("体力", v.Stamina, maxi(p.StaminaMax, 200), colGood)
	}
	if rows >= 6 {
		line := fmt.Sprintf("灵石 %s   功德 %d", thousands(p.Stones), p.Merit)
		kv(sc, ix, y+1+r, labelW, "身外", truncateWidth(line, iw-labelW), fg(colDim))
	}
}

func (a *app) drawExitsBox(sc *screen, x, y, w, h int) {
	traveling := a.st.player != nil && a.st.player.Travel != nil
	title := "去处"
	if traveling {
		title = "去处 · 赶路中"
	} else if a.ui.desktop {
		title = "去处 · 单击选 / 双击走"
	}
	box(sc, x, y, w, h, title, a.ui.page == pageGame && !traveling)
	ix, iw := boxInner(x, w)
	m := a.st.world
	if m == nil || iw < 10 || h < 3 {
		sc.put(ix, y+1, "雾里看不清路。", fg(colFaint))
		return
	}
	rows := h - 2
	sel := a.ui.selectedExit
	if sel >= len(m.Exits) {
		sel = 0
	}

	start := 0
	if sel >= rows {
		start = sel - rows + 1
	}

	for i := 0; i < rows; i++ {
		idx := start + i
		if idx >= len(m.Exits) {
			break
		}
		ex := m.Exits[idx]
		arrow := ""
		if m.X != nil && m.Y != nil && ex.X != nil && ex.Y != nil {
			arrow = bearingArrow(*ex.X-*m.X, *ex.Y-*m.Y)
		}
		name := ""
		if ex.Name != nil && *ex.Name != "" {
			name = *ex.Name
		} else {
			name = terrainWord(ex.Terrain)
		}
		cost := fmt.Sprintf("%ds", ex.CostSec)
		marker := "  "
		switch {
		case idx == sel:
			marker = "▸ "
		case !ex.OK:
			marker = "× "
		}
		left := fmt.Sprintf("%s%s %s", marker, arrow, name)
		selected := idx == a.ui.selectedExit && ex.OK && !traveling
		st := fg(colText)
		if !ex.OK || traveling {
			st = fg(colFaint)
		}
		if selected {
			st = fgBg(colBG, colAccent)
		}
		line := truncateWidth(left, iw-textWidth(cost)-2)
		row(sc, ix, y+1+i, iw, st)
		sc.put(ix, y+1+i, line, st)
		// 地名与耗时之间用点线引导：宽盒里不至于两地名漂在两头。
		gap := iw - textWidth(line) - textWidth(cost) - 2
		if gap > 1 {
			// 最多画 24 个点：再长就成了一条横贯全屏的虚线，反而看不清。
			n := gap - 1
			if n > 24 {
				n = 24
			}
			sc.put(ix+textWidth(line)+1, y+1+i, strings.Repeat("·", n), fg(colFaint))
		}
		sc.put(ix+iw-textWidth(cost)-1, y+1+i, cost, st)
		sc.addHit(ix, y+1+i, iw, 1, hitAction{hitExit, idx})
	}
}

// drawBattleBox 战斗面板：敌方逐只血条（1–9 选目标）、队友血条、底部按键提示。
//
// 我方气血真气不在这里重复——左边的状态盒一直在显示。
func (a *app) drawBattleBox(sc *screen, x, y, w, h int) {
	b := a.st.battle
	title := fmt.Sprintf("战斗 · 第 %d 回合", battleRound(b))
	box(sc, x, y, w, h, title, true)
	ix, iw := boxInner(x, w)
	if iw < 20 || h < 5 {
		return
	}

	foes := b.Foes
	if len(foes) == 0 && b.Enemy != nil {
		foes = []foeView{*b.Enemy}
	}

	barW := 8
	if iw > 64 {
		barW = 12
	}

	rowY := y + 1
	rows := h - 2
	reserve := 1 // 底部按键提示
	foeRows := len(foes)
	if maxFoe := rows - reserve; foeRows > maxFoe {
		foeRows = maxFoe
	}

	for i := 0; i < foeRows && i < len(foes); i++ {
		f := foes[i]
		idx := f.I
		if idx == 0 {
			idx = i + 1
		}
		marker := "  "
		st := fg(colText)
		if idx == a.ui.target {
			marker = "▸ "
			st = fgBg(colBG, colAccent)
		} else if f.Dead {
			st = fg(colFaint)
		}
		row(sc, ix, rowY, iw, st)

		val := fmt.Sprintf("%d/%d", f.HP, f.MaxHP)
		vv := textWidth(val)
		barX := ix + iw - vv - barW - 1
		nameW := barX - ix - 4 - 2
		if nameW < 4 {
			nameW = 4
		}
		left := fmt.Sprintf("%d %s%s", idx, marker, truncateWidth(f.Name, nameW))
		sc.put(ix, rowY, left, st)
		// 名字与血条之间用点线引导，宽盒里不至于两头漂。
		if gap := barX - (ix + textWidth(left)) - 1; gap > 0 {
			n := min(gap, 10)
			sc.put(ix+textWidth(left)+1, rowY, strings.Repeat("·", n), fg(colFaint))
		}
		sc.put(barX, rowY, gauge(f.HP, maxi(f.MaxHP, 1), barW), fg(colBad))
		sc.put(ix+iw-vv, rowY, val, st)
		sc.addHit(ix, rowY, iw, 1, hitAction{hitFoe, idx})
		rowY++
	}

	for _, al := range b.Allies {
		if rowY >= y+h-1-reserve {
			break
		}
		label := truncateWidth(al.Name, 4)
		gaugeRow(sc, ix, rowY, 5, barW, label, al.HP, maxi(al.MaxHP, 1), colGood)
		rowY++
	}

	if b.State == "player" && rowY < y+h-1-reserve {
		sc.put(ix, rowY, "轮到你了", fg(colDim))
		rowY++
	}
	if y+h-2 > y+1 {
		sc.put(ix, y+h-2, "a 普攻   s 技能   f 逃", fg(colFaint))
	}
}

func battleRound(b *battleView) int {
	if b.Round > b.Turn {
		return b.Round
	}
	if b.Turn > 0 {
		return b.Turn
	}
	return 1
}

func (a *app) drawLogBox(sc *screen, x, y, w, h int) {
	title := "见闻"
	if a.logBack > 0 {
		title = fmt.Sprintf("见闻 · 上翻 %d 行（End 回底）", a.logBack)
	}
	box(sc, x, y, w, h, title, false)
	ix, iw := boxInner(x, w)
	rows := h - 2
	if rows <= 0 || iw < 10 {
		return
	}
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
	// 见闻贴底：最新的永远在最下面一行，和聊天窗口的习惯一致。
	count := min(end-start, rows)
	offset := rows - count
	for i := 0; i < count; i++ {
		e, ok := a.st.logs.get(start + i)
		if !ok {
			break
		}
		ry := y + 1 + offset + i
		c := tone(e.line.K)
		text := truncateWidth(e.line.S, iw-7)
		sc.put(ix, ry, fmtHM(e.ts), fg(colFaint))
		sc.put(ix+7, ry, text, fg(c))
	}
}

func (a *app) drawObjective(sc *screen, y int) {
	title, line := a.objective()
	x := sc.put(1, y, "▸ ", fg(colGold))
	x = sc.put(x, y, title, fgBold(colGold))
	sc.put(x, y, " · "+truncateWidth(line, sc.w-x-6), fg(colDim))
}

func (a *app) objective() (string, string) {
	if b := a.st.battle; b != nil {
		target := "第一只活着的"
		if f := a.foeByIndex(a.ui.target); f != nil {
			target = f.Name
		}
		return fmt.Sprintf("战斗 · 第 %d 回合", battleRound(b)),
			fmt.Sprintf("目标 %s（a 普攻 · s 技能 · i 丹药 · f 逃）", target)
	}
	if p := a.st.player; p != nil && p.Travel != nil {
		return "赶路中", fmt.Sprintf("前往 %s，还剩 %s", p.Travel.Name, fmtCountdown(p.Travel.ArriveAt-a.st.serverNow()))
	}
	if a.st.vitals.Cultivating {
		return "打坐中", "真气在慢慢回，按 c 起身"
	}
	if m := a.st.world; m != nil && len(m.NPCs) > 0 {
		return "此间有人", "t 与 " + m.NPCs[0].Name + " 说话"
	}
	if a.st.vitals.Stamina < 30 {
		return "体力不济", "坐下调息，或回城歇脚"
	}
	explored := 0
	if m := a.st.world; m != nil {
		explored = m.Explored
	}
	if explored < 3 {
		return "摸清这条路", "↑↓ 选路，Enter 就走"
	}
	return "寻个去处", "去没走过的地方看看"
}

func (a *app) drawHint(sc *screen, y int) {
	if a.ui.typing {
		st := style{fg: colBG, bg: colAccent, hasBG: true}
		row(sc, 0, y, sc.w, st)
		tip := "Enter 送出  Tab 换频道  ↑↓ 历史  Esc 取消"
		if a.ui.chatCh == "dm" {
			if a.ui.chatTo == "" {
				tip = "/w 道号 内容 指定对象  Esc 取消"
			} else {
				tip = "Enter 送出  /r 回复 " + a.ui.chatTo + "  Esc 取消"
			}
		}
		if tw := textWidth(tip); tw > sc.w/2 {
			tip = truncateWidth(tip, sc.w/2)
		}
		tw := textWidth(tip)
		prompt := a.ui.chatPrompt()
		x := sc.put(1, y, prompt, st)
		room := sc.w - 2 - tw - 2 - textWidth(prompt)
		putCursorText(sc, x, y, max(0, room), a.ui.chatText, a.ui.chatCur, st)
		sc.put(sc.w-tw-1, y, tip, style{fg: colBG, bg: colPanel, hasBG: true})
		return
	}
	keys := ""
	switch {
	case a.st.battle != nil:
		keys = "a 普攻   s 技能   i 丹药   f 逃   ↑↓/1–9 选目标   PgUp 战报"
	case sc.w >= 100:
		keys = "Space 历练   c 打坐   b 突破   l 四周   / 说话   ? 帮助   F1–F8 页面   [ ] 翻页   q 菜单"
	default:
		// 窄屏只留最常用的：宁可少写几个键，也不要"…"截断让人以为坏了。
		keys = "Space 历练   c 打坐   b 突破   / 说话   ? 帮助   F1–F8 页面   q 菜单"
	}
	sc.put(1, y, truncateWidth(keys, sc.w-2), fg(colFaint))
}

// ---------------- 数据页 ----------------

func (a *app) drawPage(sc *screen) {
	p := a.ui.page
	box(sc, 0, 0, sc.w, sc.h-1, p.label(), true)
	sc.put(sc.w-12, 0, " Esc 返回 ", fg(colFaint))

	ix, iw := boxInner(0, sc.w)
	top := 1
	body := sc.h - 3
	if body < 1 {
		body = 1
	}
	switch p {
	case pageCharacter:
		a.drawCharacter(sc, ix, top, iw, body)
	case pageBag:
		a.drawBag(sc, ix, top, iw, body)
	case pageSkills:
		a.drawSkills(sc, ix, top, iw, body)
	case pageSect:
		a.drawSect(sc, ix, top, iw, body)
	case pageMap:
		a.drawMap(sc, ix, top, iw, body)
	case pageSettings:
		a.drawSettings(sc, ix, top, iw, body)
	case pageMarket:
		a.drawMarket(sc, ix, top, iw, body)
	case pageSocial:
		sc.put(ix, top+1, "江湖", fgBold(colGold))
		sc.put(ix, top+2, "好友 · 师徒 · 道侣 · 仇敌 · 盟约 · 队伍", fg(colDim))
		sc.put(ix, top+4, "此处在 M3 开张；眼下可先在见闻里 / 说话。", fg(colFaint))
	}
	if a.ui.typing {
		a.drawHint(sc, sc.h-1)
		return
	}
	sc.put(1, sc.h-1, truncateWidth(a.pageHint(), sc.w-2), fg(colFaint))
}

// pageHint 数据页底部的按键提示：每页只写这页真能用的键。
func (a *app) pageHint() string {
	switch a.ui.page {
	case pageBag:
		return "↑↓ 选择   Enter 使用/装备/卸下   s 出售   [ ] 翻页   Esc 返回"
	case pageSkills:
		return "↑↓ 选择   Enter 装配/卸下   [ ] 翻页   Esc 返回"
	case pageMarket:
		return "↑↓ 选择   +/- 数量   Enter 买入   [ ] 翻页   Esc 返回"
	case pageMap:
		return "Enter 刷新   l 看四周   0 回中   [ ] 翻页   Esc 返回"
	default:
		return "/ 说话   [ ] 翻页   ? 帮助   Esc 返回"
	}
}

// listWindow 计算列表可见窗口的起点：光标始终留在窗口内。
func listWindow(sel, n, rows int) int {
	if rows <= 0 {
		return 0
	}
	if sel < rows {
		return 0
	}
	return sel - rows + 1
}

func (a *app) drawCharacter(sc *screen, x, y, w, h int) {
	p := a.st.player
	if p == nil {
		return
	}
	root := "—"
	if p.Root != nil && p.Root.Name != "" {
		root = p.Root.Name
	}
	left := [][2]string{
		{"境界", p.Realm},
		{"灵根", root},
		{"攻防", fmt.Sprintf("%d / %d", p.Atk, p.Def)},
		{"身法 / 会心", fmt.Sprintf("%d / %.0f%%", p.Spd, p.Crit*100)},
		{"战力", thousands(p.Power)},
	}
	right := [][2]string{
		{"体魄", itoa(p.Attrs.Ti)},
		{"灵性", itoa(p.Attrs.Ling)},
		{"身法", itoa(p.Attrs.Feng)},
		{"护体", itoa(p.Attrs.Hu)},
		{"神识", itoa(p.Attrs.Shen)},
		{"未分配", itoa(p.Free)},
	}

	if w < 64 {
		// 窄屏单栏：先基础，再根骨。
		rows := append(append([][2]string{}, left...), right...)
		for i, kvp := range rows {
			if i >= h-1 {
				break
			}
			kv(sc, x, y+i, 14, kvp[0], truncateWidth(kvp[1], w-14), fg(colText))
		}
		return
	}

	half := w / 2
	for i, kvp := range left {
		if i >= h-1 {
			break
		}
		kv(sc, x, y+i, 14, kvp[0], truncateWidth(kvp[1], half-16), fg(colText))
	}
	for i, kvp := range right {
		if i >= h-1 {
			break
		}
		kv(sc, x+half, y+i, 12, kvp[0], kvp[1], fg(colText))
	}

	// 右栏下半：名分、功德、丹毒。
	by := y + len(right) + 1
	extra := [][2]string{
		{"势力", orDash(p.FactionName)},
		{"宗门", orDash(p.GuildName)},
		{"职位", orDash(p.GuildRank)},
		{"功德轮回", fmt.Sprintf("%d / %d", p.Merit, p.Lives)},
		{"丹毒", fmt.Sprintf("%d（%s）", p.Toxin, orDash(p.ToxinNote))},
	}
	for i, kvp := range extra {
		if by+i >= y+h-1 {
			break
		}
		kv(sc, x+half, by+i, 12, kvp[0], truncateWidth(kvp[1], half-13), fg(colText))
	}
}

// tableHeader 画表头：列与列之间用两格空白，宽度由调用方给。
func tableHeader(sc *screen, x, y int, cols []string, widths []int) {
	cx := x
	for i, c := range cols {
		sc.put(cx, y, padWidth(c, widths[i]), fg(colFaint))
		cx += widths[i] + 2
	}
}

func tableRow(sc *screen, x, y int, cells []string, widths []int, st style) {
	cx := x
	for i, c := range cells {
		sc.put(cx, y, padWidth(truncateWidth(c, widths[i]), widths[i]), st)
		cx += widths[i] + 2
	}
}

func (a *app) drawBag(sc *screen, x, y, w, h int) {
	p := a.st.player
	if p == nil {
		return
	}
	sc.put(x, y, fmt.Sprintf("%d 件   灵石 %s", len(p.Bag), thousands(p.Stones)), fg(colDim))
	if len(p.Bag) == 0 {
		sc.put(x, y+2, "行囊空空。", fg(colFaint))
		return
	}
	rows := h - 2
	if rows < 1 {
		return
	}
	widths := []int{18, 6, 8, 6}
	tableHeader(sc, x, y+1, []string{"名称", "数量", "类别", "售价"}, widths)

	sel := clampInt(a.ui.selOf(pageBag), 0, len(p.Bag)-1)
	start := listWindow(sel, len(p.Bag), rows)
	for i := 0; i < rows; i++ {
		idx := start + i
		if idx >= len(p.Bag) {
			break
		}
		it := p.Bag[idx]
		// 无色的终端里没有反色可用，用 ▸ 标光标、· 标已装备。
		name := it.Name
		switch {
		case idx == sel:
			name = "▸ " + name
		case it.Equipped:
			name = "· " + name
		}
		st := fg(colText)
		if it.Equipped {
			st = fg(colAccent)
		}
		ry := y + 2 + i
		if idx == sel {
			row(sc, x, ry, w, fgBg(colBG, colAccent))
			st = fgBg(colBG, colAccent)
		}
		tableRow(sc, x, ry, []string{name, itoa64(it.N), kindName(it.Kind), itoa64(it.Sell)}, widths, st)
		sc.addHit(x, ry, w, 1, hitAction{hitBagItem, idx})
	}
}

func (a *app) drawSkills(sc *screen, x, y, w, h int) {
	p := a.st.player
	if p == nil {
		return
	}
	main := "—"
	if p.MainName != "" {
		main = p.MainName
	}
	equipped := len(equippedSkillIDs(p))
	sc.put(x, y, fmt.Sprintf("主修 %s   功法 %d 门   技能槽 %d/%d",
		main, len(p.Manuals), equipped, skillSlotCap(p.Stage)), fg(colDim))
	if len(p.SkillBook) == 0 {
		sc.put(x, y+2, "还没学艺在身。", fg(colFaint))
		return
	}
	rows := h - 2
	if rows < 1 {
		return
	}
	widths := []int{16, 6, 6, 8}
	tableHeader(sc, x, y+1, []string{"技能", "真气", "威力", "状态"}, widths)

	sel := clampInt(a.ui.selOf(pageSkills), 0, len(p.SkillBook)-1)
	start := listWindow(sel, len(p.SkillBook), rows)
	for i := 0; i < rows; i++ {
		idx := start + i
		if idx >= len(p.SkillBook) {
			break
		}
		sk := p.SkillBook[idx]
		state := "未装配"
		st := fg(colText)
		if sk.Equipped {
			st, state = fg(colAccent), "已装配"
		}
		ry := y + 2 + i
		if idx == sel {
			row(sc, x, ry, w, fgBg(colBG, colAccent))
			st = fgBg(colBG, colAccent)
		}
		name := sk.Name
		if idx == sel {
			name = "▸ " + name
		}
		tableRow(sc, x, ry, []string{name, itoa64(sk.MP), fmt.Sprintf("%.1f", sk.Power), state}, widths, st)
		sc.addHit(x, ry, w, 1, hitAction{hitSkillItem, idx})
	}
}

// drawMarket 坊市：服务端 ready.catalog.shop 就是货架，价格与门槛都按它来。
func (a *app) drawMarket(sc *screen, x, y, w, h int) {
	p := a.st.player
	items := a.shopList()
	if p != nil {
		sc.put(x, y, fmt.Sprintf("灵石 %s   数量 %d（+/- 调整）", thousands(p.Stones), a.ui.shopQty), fg(colDim))
	}
	if len(items) == 0 {
		sc.put(x, y+2, "货架是空的（服务端没下发 catalog）。", fg(colFaint))
		return
	}
	rows := h - 3
	if rows < 1 {
		rows = 1
	}
	widths := []int{18, 8, 8, 6}
	tableHeader(sc, x, y+1, []string{"货物", "类别", "价格", "需阶"}, widths)

	sel := clampInt(a.ui.selOf(pageMarket), 0, len(items)-1)
	start := listWindow(sel, len(items), rows)
	for i := 0; i < rows; i++ {
		idx := start + i
		if idx >= len(items) {
			break
		}
		it := items[idx]
		st := fg(colText)
		if p != nil && p.Stage < it.Min {
			st = fg(colFaint)
		}
		ry := y + 2 + i
		if idx == sel {
			row(sc, x, ry, w, fgBg(colBG, colAccent))
			st = fgBg(colBG, colAccent)
		}
		name := it.Name
		if idx == sel {
			name = "▸ " + name
		}
		tableRow(sc, x, ry, []string{
			name, kindName(it.Kind), itoa64(it.Price), itoa64(int64(it.Min)),
		}, widths, st)
		sc.addHit(x, ry, w, 1, hitAction{hitShopItem, idx})
	}
	it := items[sel]
	desc := it.Desc
	if desc == "" {
		desc = "—"
	}
	sc.put(x, y+h-1, truncateWidth(fmt.Sprintf("%s：%s（卖价 %d）", it.Name, desc, it.Sell), w), fg(colFaint))
}

func (a *app) drawSect(sc *screen, x, y, w, h int) {
	p := a.st.player
	if p == nil {
		return
	}
	guild := p.GuildName
	if guild == "" {
		guild = "散修"
	}
	kv(sc, x, y, 14, "玩家宗门", guild, fg(colText))
	kv(sc, x, y+1, 14, "职位 / 贡献", fmt.Sprintf("%s / %d", orDash(p.GuildRank), p.GuildContrib), fg(colText))
	kv(sc, x, y+3, 14, "NPC 势力", orDash(p.FactionName), fg(colText))
	kv(sc, x, y+4, 14, "声望", fmt.Sprintf("%d（%s）", p.Fame, orDash(p.FameTier)), fg(colText))
	kv(sc, x, y+5, 14, "势力贡献", itoa64(p.Contrib), fg(colText))
	sc.put(x, y+7, "建宗、捐献、宗门战在 M4 落地。", fg(colFaint))
}

func (a *app) drawSettings(sc *screen, x, y, w, h int) {
	hasChar := "无"
	if a.st.hasChar {
		hasChar = "已载入"
	}
	kv(sc, x, y, 10, "服务器", a.cfg.Server, fg(colText))
	kv(sc, x, y+1, 10, "协议", fmt.Sprintf("v%d   角色 %s   日志 %d 行", protoVersion, hasChar, a.st.logs.len()), fg(colText))
	kv(sc, x, y+2, 10, "连接", a.st.conn.badgeText(), fg(colText))
	kv(sc, x, y+3, 10, "配置", configPath(), fg(colText))
	mode := "无边框（只留游戏界面）"
	if !a.cfg.borderless() {
		mode = "带边框（可拖动与缩放）"
	}
	hint := "Space 切换"
	if a.ui.desktop {
		hint = "Space / 点击切换"
	}
	kv(sc, x, y+4, 10, "窗口", mode+"   "+hint, fg(colText))
	hitW := 58
	if hitW > w {
		hitW = w
	}
	if hitW > 0 {
		sc.addHit(x, y+4, hitW, 1, hitAction{kind: hitWindowToggle})
	}
	sc.put(x, y+6, "退出登录：Esc 回主界面后按 o（只退出账号，不退出客户端）", fg(colFaint))
}

// drawEncounter 奇遇三选一：居中弹层，数字键选。
func drawEncounter(sc *screen, e *eventView) {
	h := len(e.Options) + 6
	w := 46
	if w > sc.w-4 {
		w = sc.w - 4
	}
	if h > sc.h-2 {
		h = sc.h - 2
	}
	x := (sc.w - w) / 2
	y := (sc.h - h) / 2
	if x < 1 {
		x = 1
	}
	if y < 1 {
		y = 1
	}
	// 奇遇要停下来做选择：保留标题栏，其余整屏铺底，避免弹层边框与底下的盒子
	// 边框在同一行"错位拼接"。
	rowFill(sc, 0, 1, sc.w, sc.h-1)
	box(sc, x, y, w, h, "奇遇", true)
	ix, iw := boxInner(x, w)
	for i, opt := range e.Options {
		if y+2+i >= y+h-2 {
			break
		}
		line := fmt.Sprintf("%d  %s", i+1, opt.Text)
		sc.put(ix, y+2+i, truncateWidth(line, iw), fg(colText))
		sc.addHit(ix, y+2+i, iw, 1, hitAction{hitEncounter, i})
	}
	sc.put(ix, y+h-2, "按 1–"+itoa(len(e.Options))+" 选择", fg(colFaint))
}

// drawQuitMenu 离开菜单：q 打开，退出游戏 / 返回登录窗口 / 取消。
func drawQuitMenu(sc *screen, sel int, hasLauncher bool) {
	w, h := 42, 8
	if w > sc.w-4 {
		w = sc.w - 4
	}
	if h > sc.h-2 {
		h = sc.h - 2
	}
	if w < 20 || h < 6 {
		return
	}
	rowFill(sc, 0, 1, sc.w, sc.h-1)
	sc.addHit(0, 0, sc.w, sc.h, hitAction{hitQuitCancel, 0})
	x := (sc.w - w) / 2
	y := (sc.h - h) / 2
	if x < 1 {
		x = 1
	}
	if y < 1 {
		y = 1
	}
	box(sc, x, y, w, h, "离开游戏", true)
	ix, iw := boxInner(x, w)
	login := "返回登录界面"
	if hasLauncher {
		login = "返回登录窗口"
	}
	options := []string{"退出游戏", login, "取消"}
	for i, label := range options {
		ry := y + 2 + i
		line := fmt.Sprintf("%d  %s", i+1, label)
		st := fg(colText)
		if i == sel {
			row(sc, ix, ry, iw, fgBg(colBG, colAccent))
			st = fgBg(colBG, colAccent)
		}
		sc.put(ix, ry, truncateWidth(line, iw), st)
		sc.addHit(ix, ry, iw, 1, hitAction{hitQuitOption, i})
	}
	sc.put(ix, y+h-2, "↑↓ 选择   Enter 确认   Esc 返回", fg(colFaint))
}

// drawTalk 交谈弹层：列出当前节点的人物，↑↓ 选、数字直选。
func drawTalk(sc *screen, m *mapView, sel int) {
	if m == nil || len(m.NPCs) == 0 {
		return
	}
	list := m.NPCs
	h := len(list) + 5
	w := 50
	if w > sc.w-4 {
		w = sc.w - 4
	}
	if h > sc.h-2 {
		h = sc.h - 2
	}
	if w < 24 || h < 5 {
		return
	}
	rowFill(sc, 0, 1, sc.w, sc.h-1)
	x := (sc.w - w) / 2
	y := (sc.h - h) / 2
	if x < 1 {
		x = 1
	}
	if y < 1 {
		y = 1
	}
	box(sc, x, y, w, h, "交谈", true)
	ix, iw := boxInner(x, w)
	for i, n := range list {
		ry := y + 1 + i
		if ry >= y+h-2 {
			break
		}
		line := fmt.Sprintf("%d  %s", i+1, n.Name)
		if n.Title != "" {
			line += " · " + n.Title
		}
		st := fg(colText)
		if i == sel {
			row(sc, ix, ry, iw, fgBg(colBG, colAccent))
			st = fgBg(colBG, colAccent)
		}
		sc.put(ix, ry, truncateWidth(line, iw), st)
		sc.addHit(ix, ry, iw, 1, hitAction{hitTalk, i})
	}
	sc.put(ix, y+h-2, "↑↓ 选   Enter 说话   Esc 不说", fg(colFaint))
}

// drawHelp 帮助浮层：一屏看完所有键位，按 ? 或 Esc 收起。
func drawHelp(sc *screen) {
	lines := [][2]string{
		{"行走", "↑↓ / j k 选路    Enter 前往    l 看四周"},
		{"修行", "Space 历练    c 打坐    b 突破"},
		{"交谈", "/ 说话    Tab 换频道    /w 道号 内容 私语    /r 内容 回复    t 与人物交谈"},
		{"指令", "/任务  /排行  /学 功法  /加点 ti  /炼 丹药  /势力    （服务端文本指令）"},
		{"界面", "F1–F8 数据页    [ ] 前后翻    PgUp/PgDn 见闻    End 回最新"},
		{"背包", "↑↓ 选择    Enter 使用/装备    e 装备    s 出售"},
		{"技能", "↑↓ 选择    Enter 装配/卸下"},
		{"坊市", "↑↓ 选择    +/- 数量    Enter 买入"},
		{"战斗", "↑↓ / 1–9 选目标    a 普攻    s 技能    i 丹药    f 逃"},
		{"其他", "o 退出登录（两次）    q 离开菜单（退出/回登录）    ? 收起帮助"},
	}
	w := 76
	if w > sc.w-4 {
		w = sc.w - 4
	}
	if w < 30 || sc.h < 8 {
		return
	}
	h := len(lines) + 3
	if h > sc.h-2 {
		h = sc.h - 2
	}
	// 整屏铺底：帮助是"停下来看"的浮层，不该和底下的盒子边框交错。
	rowFill(sc, 0, 1, sc.w, sc.h-1)
	x := (sc.w - w) / 2
	y := (sc.h - h) / 2
	if x < 1 {
		x = 1
	}
	if y < 1 {
		y = 1
	}
	box(sc, x, y, w, h, "帮助", true)
	ix, iw := boxInner(x, w)
	for i, ln := range lines {
		ry := y + 1 + i
		if ry >= y+h-1 {
			break
		}
		lx := sc.put(ix, ry, padWidth(ln[0], 6), fg(colGold))
		sc.put(lx, ry, truncateWidth(ln[1], iw-6), fg(colText))
	}
	if y+h-2 > y+1 {
		sc.put(ix, y+h-2, "? 或 Esc 收起", fg(colFaint))
	}
}

// rowFill 用面板底色铺一块矩形，避免弹层底下透出内容。
func rowFill(sc *screen, x, y, w, h int) {
	st := style{fg: colText, bg: colBG, hasBG: true}
	for j := y; j < y+h; j++ {
		row(sc, x, j, w, st)
	}
}

// ---------------- toast ----------------

func drawToast(sc *screen, ui *uiState) {
	t, ok := ui.activeToast()
	if !ok || sc.h < 3 {
		return
	}
	w := min(sc.w-2, textWidth(t.text)+6)
	if w < 8 {
		return
	}
	// 放在目标行右端：不压盒子边框，也不挡按键表。
	// 数据页没有目标行，往上挪一行，否则会盖住整屏盒子的下边框。
	y := sc.h - 2
	if ui.view == viewMain && ui.page != pageGame {
		y = sc.h - 3
	}
	if y < 1 {
		y = 1
	}
	x0 := sc.w - w - 1
	st := style{fg: colBG, bg: colAccent, hasBG: true, bold: true}
	if t.bad {
		st = style{fg: colText, bg: colBad, hasBG: true, bold: true}
	}
	row(sc, x0, y, w, st)
	sc.put(x0+1, y, "! "+truncateWidth(t.text, w-3), st)
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
