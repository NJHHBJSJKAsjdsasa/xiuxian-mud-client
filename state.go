package main

import (
	"math"
	"sort"
)

// 客户端状态：服务端帧是唯一事实来源，这里只做归并与缓存。
// 不含任何渲染代码，单测可以直接构造帧喂进来。

const (
	logCap   = 5000
	chatKeep = 200
)

type logEntry struct {
	line logLine
	ts   int64
}

// logBuf 环形日志：超限从头部丢，摊还 O(1)。
type logBuf struct {
	lines   []logEntry
	dropped int
}

func newLogBuf() *logBuf { return &logBuf{lines: make([]logEntry, 0, 512)} }

func (b *logBuf) push(l logLine) {
	b.lines = append(b.lines, logEntry{line: l, ts: nowMs()})
	if len(b.lines) > logCap {
		drop := len(b.lines) - logCap
		b.lines = append(b.lines[:0], b.lines[drop:]...)
		b.dropped += drop
	}
}

func (b *logBuf) len() int { return len(b.lines) }

func (b *logBuf) get(i int) (logEntry, bool) {
	if i < 0 || i >= len(b.lines) {
		return logEntry{}, false
	}
	return b.lines[i], true
}

type mapNode struct {
	id      string
	name    string
	terrain string
	visited bool
	hasGeo  bool
	gx, gy  int
}

type mapEdge struct {
	ok      bool
	blocked bool
	costSec int64
}

// mapGraph 客户端累积的已探索地图：服务端只给"当前节点 + 相邻出口"，
// 全图是走出来的。
type mapGraph struct {
	nodes map[string]*mapNode
	edges map[[2]string]*mapEdge
}

func newMapGraph() *mapGraph {
	return &mapGraph{nodes: map[string]*mapNode{}, edges: map[[2]string]*mapEdge{}}
}

func edgeKey(a, b string) [2]string {
	if a <= b {
		return [2]string{a, b}
	}
	return [2]string{b, a}
}

func (g *mapGraph) observe(m *mapView) {
	if m == nil || m.Loc == "" {
		return
	}
	cur := g.node(m.Loc)
	if m.Name != "" {
		cur.name = m.Name
	}
	if m.Terrain != "" {
		cur.terrain = m.Terrain
	}
	if m.X != nil && m.Y != nil {
		cur.gx, cur.gy, cur.hasGeo = *m.X, *m.Y, true
	}
	cur.visited = true

	for _, ex := range m.Exits {
		if ex.To == "" {
			continue
		}
		n := g.node(ex.To)
		if ex.Name != nil && *ex.Name != "" {
			n.name = *ex.Name
		}
		if ex.Terrain != "" {
			n.terrain = ex.Terrain
		}
		if ex.X != nil && ex.Y != nil {
			n.gx, n.gy, n.hasGeo = *ex.X, *ex.Y, true
		}
		g.edges[edgeKey(m.Loc, ex.To)] = &mapEdge{
			ok:      ex.OK,
			blocked: ex.Blocked != nil && *ex.Blocked,
			costSec: ex.CostSec,
		}
	}
}

func (g *mapGraph) node(id string) *mapNode {
	if n, ok := g.nodes[id]; ok {
		return n
	}
	n := &mapNode{id: id}
	g.nodes[id] = n
	return n
}

func (g *mapGraph) empty() bool { return len(g.nodes) == 0 }

func (g *mapGraph) neighbors(id string) []string {
	var out []string
	for k := range g.edges {
		switch id {
		case k[0]:
			out = append(out, k[1])
		case k[1]:
			out = append(out, k[0])
		}
	}
	sort.Strings(out)
	return out
}

// layout 返回每个节点的平面坐标（当前节点在原点）。
// 服务端给了内容表坐标就按真实排布画；没给就按 BFS 层级摊开。
func (g *mapGraph) layout(current string) map[string][2]float64 {
	out := map[string][2]float64{}
	cur, ok := g.nodes[current]
	if ok && cur.hasGeo {
		const scale = 11.0
		for id, n := range g.nodes {
			if n.hasGeo {
				out[id] = [2]float64{float64(n.gx-cur.gx) * scale, -float64(n.gy-cur.gy) * scale}
			}
		}
		// 少数节点没坐标：贴到已定位邻居的平均位置，别丢。
		for id := range g.nodes {
			if _, done := out[id]; done {
				continue
			}
			sx, sy, n := 0.0, 0.0, 0.0
			for _, nb := range g.neighbors(id) {
				if p, ok := out[nb]; ok {
					sx += p[0]
					sy += p[1]
					n++
				}
			}
			ang := hash01(id) * 2 * math.Pi
			if n > 0 {
				out[id] = [2]float64{sx/n + math.Cos(ang)*26, sy/n + math.Sin(ang)*26}
			} else {
				out[id] = [2]float64{math.Cos(ang) * 40, math.Sin(ang) * 40}
			}
		}
		return out
	}

	// 无坐标：BFS 分层，第 d 层摆成半径 120*d 的环。
	level := map[string]int{current: 0}
	queue := []string{current}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, nb := range g.neighbors(id) {
			if _, seen := level[nb]; seen {
				continue
			}
			level[nb] = level[id] + 1
			queue = append(queue, nb)
		}
	}
	byLevel := map[int][]string{}
	maxLevel := 0
	for id := range g.nodes {
		d, ok := level[id]
		if !ok {
			d = 3
		}
		byLevel[d] = append(byLevel[d], id)
		if d > maxLevel {
			maxLevel = d
		}
	}
	for d := 0; d <= maxLevel; d++ {
		ids := byLevel[d]
		sort.Strings(ids)
		for i, id := range ids {
			if d == 0 {
				out[id] = [2]float64{0, 0}
				continue
			}
			ang := 2 * math.Pi * (float64(i)/float64(max(1, len(ids))) + hash01(id)*0.1)
			r := 120.0 * float64(d)
			out[id] = [2]float64{math.Cos(ang) * r, math.Sin(ang) * r}
		}
	}
	return out
}

func hash01(s string) float64 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return float64(h%10000) / 10000
}

type appState struct {
	conn        connStatus
	player      *playerView
	world       *mapView
	vitals      vitalView
	presence    int64
	logs        *logBuf
	chats       []chatMsg
	graph       *mapGraph
	battle      *battleView
	encounter   *eventView
	clockOffset int64
	season      string
	daypart     string
	hasChar     bool
}

func newAppState() *appState {
	return &appState{
		conn:  connStatus{kind: connIdle},
		logs:  newLogBuf(),
		graph: newMapGraph(),
	}
}

func (st *appState) serverNow() int64 { return nowMs() + st.clockOffset }

func (st *appState) applyReady(r *readyView) {
	if r.ServerNow > 0 {
		st.clockOffset = r.ServerNow - nowMs()
	}
	st.presence = r.onlineCount()
	st.hasChar = r.hasCharacter()
	st.season, st.daypart = r.Season, r.Daypart
	if r.Player != nil {
		st.applyState(r.Player)
	}
	for _, c := range r.ChatHist {
		st.pushChat(c)
	}
}

// applyState 是全量快照：整体替换，不做字段级合并。
func (st *appState) applyState(p *playerView) {
	st.player = p
	st.vitals.HP = p.HP
	st.vitals.MP = p.MP
	st.vitals.Exp = p.Exp
	st.vitals.ExpNeed = p.ExpNeed
	st.vitals.Stamina = p.Stamina
	st.vitals.Cultivating = p.Cultivating
	st.vitals.Buffs = p.Buffs
}

// applyVital 是增量：只覆盖它带的字段。
func (st *appState) applyVital(v *vitalView) {
	st.vitals = *v
	if p := st.player; p != nil {
		p.HP, p.MP, p.Exp, p.ExpNeed = v.HP, v.MP, v.Exp, v.ExpNeed
		p.Stamina, p.Cultivating, p.Buffs = v.Stamina, v.Cultivating, v.Buffs
	}
}

func (st *appState) applyMap(m *mapView) {
	st.graph.observe(m)
	// look 的应答是单节点视图（exits 为空），不能拿它覆盖当前节点的出口，
	// 否则"看一眼四周"就把路全看没了。
	if old := st.world; old != nil && len(m.Exits) == 0 && old.Loc != "" && old.Loc == m.Loc {
		merged := *m
		merged.Exits = old.Exits
		st.world = &merged
		return
	}
	st.world = m
}

func (st *appState) pushLogs(lines []logLine) {
	for _, l := range lines {
		st.logs.push(l)
	}
}

func (st *appState) pushChat(c chatMsg) {
	st.chats = append(st.chats, c)
	if len(st.chats) > chatKeep {
		st.chats = append(st.chats[:0], st.chats[len(st.chats)-chatKeep:]...)
	}
}

func (st *appState) clearSession() {
	clock := st.clockOffset
	logs := st.logs
	*st = *newAppState()
	st.clockOffset = clock
	st.logs = logs
}
