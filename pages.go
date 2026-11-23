package main

import (
	"fmt"
	"math"
	"strings"
)

// 舆图页：把已探索的节点画在盒子内部。
//
// 符号：◎ 当前、● 到过、○ 只是望见；连线 · 可走、x 断路。
// 节点名只写短地名（"青云·浅滩" → "浅滩"），放不下就不写——
// 宁可少一个标签，也不要糊成一串。

func (a *app) drawMap(sc *screen, x, y, w, h int) {
	if w < 12 || h < 6 {
		return
	}
	m := a.st.world
	if m == nil || a.st.graph.empty() {
		sc.put(x, y+1, "还没有走过路，舆图上一片空白。", fg(colFaint))
		return
	}
	g := a.st.graph
	layout := g.layout(m.Loc)
	maxr := 1.0
	for _, p := range layout {
		if r := math.Hypot(p[0], p[1]); r > maxr {
			maxr = r
		}
	}
	// 字符宽高比 1:2，纵向压一半才不会把地图拉成长条。
	scale := math.Min(float64(w), float64(h)*2) * 0.38 / maxr
	if scale < 0.02 {
		scale = 0.02
	}
	cx, cy := x+w/2, y+h/2
	pos := func(px, py float64) (int, int) {
		return cx + int(math.Round(px*scale)), cy + int(math.Round(py*scale*0.5))
	}

	for k, e := range g.edges {
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
			px := x0 + int(math.Round(float64(x1-x0)*t))
			py := y0 + int(math.Round(float64(y1-y0)*t))
			if px <= x || px >= x+w-1 || py <= y || py >= y+h-2 {
				continue
			}
			if c := sc.buf[py*sc.w+px]; c.ch == ' ' {
				ch, st := '·', fg(colFaint)
				if !e.ok {
					ch, st = 'x', fg(colBad)
				}
				sc.set(px, py, ch, st)
			}
		}
	}

	for id, p := range layout {
		gx, gy := pos(p[0], p[1])
		if gx <= x || gx >= x+w-1 || gy <= y || gy >= y+h-2 {
			continue
		}
		n := g.nodes[id]
		ch, st := '○', fg(colFaint)
		switch {
		case id == m.Loc:
			ch, st = '◎', fgBold(colAccent)
		case n != nil && n.visited:
			ch, st = '●', fg(colDim)
		}
		sc.set(gx, gy, ch, st)
		if n != nil && n.name != "" {
			placeLabel(sc, truncateChars(mapLabel(n.name), 5), gx, gy, st, y, h)
		}
	}

	legend := fmt.Sprintf("◎ 当前   ● 到过   ○ 只是望见    已记 %d 处", len(g.nodes))
	sc.put(x, y+h-1, truncateWidth(legend, w), fg(colFaint))
}

// placeLabel 给节点找一块能写字的地方：右边 → 左边 → 上下。
func placeLabel(sc *screen, label string, gx, gy int, st style, top, h int) bool {
	lw := textWidth(label)
	if lw == 0 {
		return false
	}
	for _, c := range [][2]int{{gx + 2, gy}, {gx - 2 - lw, gy}, {gx - lw/2, gy + 1}, {gx - lw/2, gy - 1}} {
		x, y := c[0], c[1]
		if y <= top || y >= top+h-1 || x <= 0 || x+lw >= sc.w-1 {
			continue
		}
		ok := true
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

func (c connStatus) badgeText() string {
	_, text := c.badge()
	switch c.kind {
	case connBackoff:
		return fmt.Sprintf("%s（第 %d 秒）", text, c.secs)
	default:
		return text
	}
}
