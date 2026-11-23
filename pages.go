package main

import (
	"fmt"
	"math"
	"sort"
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

	// 边与节点都从 map 里取。直接 range 的顺序每帧都不同：重叠处的字符
	// 与地名标签会互相抢位置，屏幕上看就是"地点闪来闪去"（4Hz 重绘）。
	// 这里全部排序后再画，同一份数据每帧必须画出同一屏。
	for _, ln := range sortedEdges(g) {
		a0, ok1 := layout[ln.key[0]]
		b0, ok2 := layout[ln.key[1]]
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
				if !ln.e.ok {
					ch, st = 'x', fg(colBad)
				}
				sc.set(px, py, ch, st)
			}
		}
	}

	// 先画所有节点，再统一放标签：节点之间的位置竞争按优先级收口
	// （望见 → 到过 → 当前），当前节点最后画，同格时一定留下 ◎。
	nodes := sortedNodes(g, layout, m.Loc, pos, x, y, w, h)
	for _, n := range nodes {
		sc.set(n.gx, n.gy, n.ch, n.st)
	}
	// 标签按"重要度 → 屏幕位置"排序：当前节点先占位，其次到过、望见；
	// 同重要度再按上→下、左→右。相邻标签谁先占位不再随机，位置也就不会跳。
	labels := append([]mapNodeDraw(nil), nodes...)
	sort.Slice(labels, func(i, j int) bool {
		if labels[i].prio != labels[j].prio {
			return labels[i].prio > labels[j].prio
		}
		if labels[i].gy != labels[j].gy {
			return labels[i].gy < labels[j].gy
		}
		if labels[i].gx != labels[j].gx {
			return labels[i].gx < labels[j].gx
		}
		return labels[i].id < labels[j].id
	})
	for _, n := range labels {
		if n.name != "" {
			placeLabel(sc, truncateChars(mapLabel(n.name), 5), n.gx, n.gy, n.st, y, h)
		}
	}

	legend := fmt.Sprintf("◎ 当前   ● 到过   ○ 只是望见    已记 %d 处", len(g.nodes))
	sc.put(x, y+h-1, truncateWidth(legend, w), fg(colFaint))
}

// mapEdgeLine 是排好序的一条边。断路排在通路前面：两者共用一格时
// 留 x 比留 · 信息更明确，也保证每帧一致。
type mapEdgeLine struct {
	key [2]string
	e   *mapEdge
}

func sortedEdges(g *mapGraph) []mapEdgeLine {
	out := make([]mapEdgeLine, 0, len(g.edges))
	for k, e := range g.edges {
		out = append(out, mapEdgeLine{key: k, e: e})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].e.ok != out[j].e.ok {
			return !out[i].e.ok
		}
		if out[i].key[0] != out[j].key[0] {
			return out[i].key[0] < out[j].key[0]
		}
		return out[i].key[1] < out[j].key[1]
	})
	return out
}

// mapNodeDraw 是一个已定位的节点：prio 越大越后画（当前节点盖住其它符号）。
type mapNodeDraw struct {
	id   string
	gx   int
	gy   int
	ch   rune
	st   style
	prio int
	name string
}

func sortedNodes(g *mapGraph, layout map[string][2]float64, cur string,
	pos func(float64, float64) (int, int), x, y, w, h int) []mapNodeDraw {

	ids := make([]string, 0, len(layout))
	for id := range layout {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]mapNodeDraw, 0, len(ids))
	for _, id := range ids {
		p := layout[id]
		gx, gy := pos(p[0], p[1])
		if gx <= x || gx >= x+w-1 || gy <= y || gy >= y+h-2 {
			continue
		}
		n := g.nodes[id]
		nd := mapNodeDraw{id: id, gx: gx, gy: gy, ch: '○', st: fg(colFaint), prio: 0}
		if n != nil {
			nd.name = n.name
			if n.visited {
				nd.ch, nd.st, nd.prio = '●', fg(colDim), 1
			}
		}
		if id == cur {
			nd.ch, nd.st, nd.prio = '◎', fgBold(colAccent), 2
		}
		out = append(out, nd)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].prio != out[j].prio {
			return out[i].prio < out[j].prio
		}
		return out[i].id < out[j].id
	})
	return out
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
