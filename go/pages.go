package main

import (
	"fmt"
	"math"
	"strings"
)

// 数据页：整屏覆盖，Esc 回主界面。一页一件事，不做分屏。

func (a *app) drawPage(sc *screen) {
	p := a.ui.page
	sc.put(1, 0, p.label(), fgBold(colText))
	sc.put(1+textWidth(p.label()), 0, "   Esc 返回", fg(colFaint))

	top := 1
	body := sc.h - 2
	if body < 1 {
		body = 1
	}
	switch p {
	case pageCharacter:
		a.drawCharacter(sc, top, body)
	case pageBag:
		a.drawBag(sc, top, body)
	case pageSkills:
		a.drawSkills(sc, top, body)
	case pageSect:
		a.drawSect(sc, top, body)
	case pageMap:
		a.drawMap(sc, top, body)
	case pageSettings:
		a.drawSettings(sc, top, body)
	case pageSocial, pageMarket:
		name, desc, milestone := "江湖", "好友 · 师徒 · 道侣 · 仇敌 · 盟约 · 队伍", "M3"
		if p == pageMarket {
			name, desc, milestone = "坊市", "挂单 · 我的 · 成交 · 曲线 · 拍卖 · 书信", "M3"
		}
		sc.put(2, top+1, name, fg(colGold))
		sc.put(2, top+2, desc, fg(colDim))
		sc.put(2, top+4, "此处在 "+milestone+" 开张", fg(colFaint))
	}
	sc.put(1, sc.h-1, " Esc 返回    PgUp/PgDn 翻页", fg(colFaint))
}

func kvRow(sc *screen, y int, k, v string) {
	x := sc.put(2, y, padWidth(k, 12), fg(colFaint))
	sc.put(x, y, v, fg(colText))
}

func (a *app) drawCharacter(sc *screen, top, body int) {
	p := a.st.player
	if p == nil {
		return
	}
	root := "—"
	if p.Root != nil && p.Root.Name != "" {
		root = p.Root.Name
	}
	y := top + 1
	kvRow(sc, y, "境界", p.Realm)
	y++
	kvRow(sc, y, "灵根", root)
	y++
	kvRow(sc, y, "攻击 / 防御", fmt.Sprintf("%d / %d", p.Atk, p.Def))
	y++
	kvRow(sc, y, "身法 / 会心", fmt.Sprintf("%d / %.0f%%", p.Spd, p.Crit*100))
	y++
	kvRow(sc, y, "战力", fmt.Sprintf("%d", p.Power))
	y += 2
	kvRow(sc, y, "根骨", fmt.Sprintf("体魄 %d  灵性 %d  身法 %d  护体 %d  神识 %d  未分配 %d",
		p.Attrs.Ti, p.Attrs.Ling, p.Attrs.Feng, p.Attrs.Hu, p.Attrs.Shen, p.Free))
	y += 2
	kvRow(sc, y, "名分", fmt.Sprintf("势力 %s  宗门 %s  职位 %s",
		orDash(p.FactionName), orDash(p.GuildName), orDash(p.GuildRank)))
	y++
	kvRow(sc, y, "功德 / 轮回", fmt.Sprintf("%d / %d", p.Merit, p.Lives))
	y++
	kvRow(sc, y, "丹毒", fmt.Sprintf("%d（%s）", p.Toxin, orDash(p.ToxinNote)))
}

func (a *app) drawBag(sc *screen, top, body int) {
	p := a.st.player
	if p == nil {
		return
	}
	y := top
	sc.put(2, y, fmt.Sprintf("%d 件   灵石 %s", len(p.Bag), thousands(p.Stones)), fg(colDim))
	y++
	sc.put(2, y, fmt.Sprintf("  %s%s%s%s", padWidth("名称", 18), padWidth("数量", 8), padWidth("类别", 10), "售价"), fg(colFaint))
	y++
	if len(p.Bag) == 0 {
		sc.put(2, y, "行囊空空。", fg(colDim))
		return
	}
	for _, it := range p.Bag {
		if y >= top+body {
			break
		}
		name := truncateWidth(it.Name, 16)
		st := fg(colText)
		if it.Equipped {
			st = fg(colAccent)
		}
		sc.put(2, y, "  "+padWidth(name, 18)+padWidth(fmt.Sprintf("%d", it.N), 8)+
			padWidth(kindName(it.Kind), 10)+fmt.Sprintf("%d", it.Sell), st)
		y++
	}
}

func (a *app) drawSkills(sc *screen, top, body int) {
	p := a.st.player
	if p == nil {
		return
	}
	y := top
	main := "—"
	if p.MainName != "" {
		main = p.MainName
	}
	sc.put(2, y, fmt.Sprintf("主修 %s   功法 %d 门   技能槽 %d", main, len(p.Manuals), len(p.Slots)), fg(colDim))
	y++
	sc.put(2, y, fmt.Sprintf("  %s%s%s%s", padWidth("技能", 16), padWidth("真气", 8), padWidth("威力", 8), "状态"), fg(colFaint))
	y++
	if len(p.SkillBook) == 0 {
		sc.put(2, y, "还没学艺在身。", fg(colDim))
		return
	}
	for _, s := range p.SkillBook {
		if y >= top+body {
			break
		}
		st := fg(colText)
		state := "未装配"
		if s.Equipped {
			st, state = fg(colAccent), "已装配"
		}
		sc.put(2, y, "  "+padWidth(truncateWidth(s.Name, 14), 16)+
			padWidth(fmt.Sprintf("%d", s.MP), 8)+
			padWidth(fmt.Sprintf("%.1f", s.Power), 8)+state, st)
		y++
	}
}

func (a *app) drawSect(sc *screen, top, body int) {
	p := a.st.player
	if p == nil {
		return
	}
	y := top + 1
	guild := p.GuildName
	if guild == "" {
		guild = "散修"
	}
	kvRow(sc, y, "玩家宗门", guild)
	y++
	kvRow(sc, y, "职位 / 贡献", fmt.Sprintf("%s / %d", orDash(p.GuildRank), p.GuildContrib))
	y += 2
	kvRow(sc, y, "NPC 势力", orDash(p.FactionName))
	y++
	kvRow(sc, y, "声望", fmt.Sprintf("%d（%s）", p.Fame, orDash(p.FameTier)))
	y++
	kvRow(sc, y, "势力贡献", fmt.Sprintf("%d", p.Contrib))
	y += 2
	sc.put(2, y, "建宗、捐献、宗门战在 M4 落地。", fg(colFaint))
}

func (a *app) drawSettings(sc *screen, top, body int) {
	y := top + 1
	kvRow(sc, y, "服务器", a.cfg.Server)
	y++
	hasChar := "无"
	if a.st.hasChar {
		hasChar = "已载入"
	}
	kvRow(sc, y, "协议", fmt.Sprintf("v%d   角色 %s   日志 %d 行", protoVersion, hasChar, a.st.logs.len()))
	y++
	kvRow(sc, y, "配置", configPath())
	y++
	kvRow(sc, y, "连接", a.st.conn.badgeText())
	y += 2
	sc.put(2, y, "退出登录：Esc 回主界面后按 o（只退出账号，不退出客户端）", fg(colFaint))
}

func (a *app) drawMap(sc *screen, top, body int) {
	if body < 5 || sc.w < 10 {
		return
	}
	m := a.st.world
	if m == nil || a.st.graph.empty() {
		sc.put(2, top+1, "还没有走过路，舆图上一片空白。", fg(colDim))
		return
	}
	graph := a.st.graph
	layout := graph.layout(m.Loc)
	maxr := 1.0
	for _, p := range layout {
		if r := math.Hypot(p[0], p[1]); r > maxr {
			maxr = r
		}
	}
	scale := (math.Min(float64(sc.w), float64(body)*2) * 0.38 / maxr)
	if scale < 0.02 {
		scale = 0.02
	}
	cx, cy := sc.w/2, body/2
	pos := func(x, y float64) (int, int) {
		return cx + int(math.Round(x*scale)), top + cy + int(math.Round(y*scale*0.5))
	}

	// 先连线，再盖节点，最后写名字。
	for k, e := range graph.edges {
		a0, ok1 := layout[k[0]]
		b0, ok2 := layout[k[1]]
		if !ok1 || !ok2 {
			continue
		}
		x0, y0 := pos(a0[0], a0[1])
		x1, y1 := pos(b0[0], b0[1])
		steps := max(absInt(x1-x0)+absInt(y1-y0), 1)
		for i := 0; i <= steps; i++ {
			t := float64(i) / float64(steps)
			x := x0 + int(math.Round(float64(x1-x0)*t))
			y := y0 + int(math.Round(float64(y1-y0)*t))
			if x <= 0 || x >= sc.w-1 || y <= top || y >= sc.h-1 {
				continue
			}
			if c := sc.buf[y*sc.w+x]; c.ch == ' ' {
				ch, st := '·', fg(colFaint)
				if !e.ok {
					ch, st = 'x', fg(colBad)
				}
				sc.set(x, y, ch, st)
			}
		}
	}

	for id, p := range layout {
		gx, gy := pos(p[0], p[1])
		if gx <= 0 || gx >= sc.w-1 || gy <= top || gy >= sc.h-1 {
			continue
		}
		n := graph.nodes[id]
		ch, st := '○', fg(colFaint)
		switch {
		case id == m.Loc:
			ch, st = '◎', fg(colAccent)
		case n != nil && n.visited:
			ch, st = '●', fg(colDim)
		}
		sc.set(gx, gy, ch, st)

		name := ""
		if n != nil {
			name = n.name
		}
		if name == "" {
			continue
		}
		// 舆图上只写地名本身（"青云·樵径" → "樵径"）。
		label := truncateChars(mapLabel(name), 5)
		placeLabel(sc, label, gx, gy, st)
	}

	legend := fmt.Sprintf(" ◎ 当前  ● 到过  ○ 只是望见   已记 %d 处", len(graph.nodes))
	sc.put(1, sc.h-2, legend, fg(colFaint))
}

// placeLabel 给节点找一块能写字的地方：优先右边，其次左边，最后上下。
// 找不到就干脆不写——宁可少一个标签，也不要盖住别的节点或糊成一串。
func placeLabel(sc *screen, label string, gx, gy int, st style) bool {
	lw := textWidth(label)
	if lw == 0 {
		return false
	}
	cands := [][2]int{
		{gx + 2, gy},
		{gx - 2 - lw, gy},
		{gx - lw/2, gy + 1},
		{gx - lw/2, gy - 1},
	}
	for _, c := range cands {
		x, y := c[0], c[1]
		if y < 1 || y > sc.h-3 || x < 1 || x+lw > sc.w-1 {
			continue
		}
		ok := true
		// 左右各留一列空隙，避免和邻格内容粘在一起。
		for i := -1; i <= lw && ok; i++ {
			if !labelFree(sc, y, x+i) {
				ok = false
			}
		}
		if !ok {
			continue
		}
		for _, r := range label {
			sc.set(x, y, r, st)
			x += runeWidth(r)
		}
		return true
	}
	return false
}

// mapLabel 去掉区域前缀：舆图上地方小，写"浅滩"比"青云·浅滩"清楚。
func mapLabel(name string) string {
	if i := strings.LastIndex(name, "·"); i >= 0 && i+len("·") < len(name) {
		return name[i+len("·"):]
	}
	return name
}

func labelFree(sc *screen, y, x int) bool {
	if y < 0 || y >= sc.h || x < 0 || x >= sc.w {
		return false
	}
	c := sc.buf[y*sc.w+x]
	return c.ch == ' ' || c.ch == '·' || c.ch == 'x'
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// badgeText 给设置页用的一行文字版连接状态。
func (c connStatus) badgeText() string {
	_, text := c.badge()
	switch c.kind {
	case connOnline, connOffline:
		return text
	case connBackoff:
		return fmt.Sprintf("%s（第 %d 秒）", text, c.secs)
	default:
		return text
	}
}
