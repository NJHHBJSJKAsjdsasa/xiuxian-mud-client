package main

import "testing"

func strp(s string) *string { return &s }
func intp(n int) *int       { return &n }

func exit(to, name string, cost int64, ok bool) exitView {
	e := exitView{To: to, Terrain: "plain", CostSec: cost, OK: ok}
	if name != "" {
		e.Name = strp(name)
	}
	return e
}

func TestLogBufCapsAndDrops(t *testing.T) {
	b := newLogBuf()
	for i := 0; i < logCap+5; i++ {
		b.push(logLine{K: "sys", S: itoa(i)})
	}
	if b.len() != logCap {
		t.Fatalf("长度 %d，应为 %d", b.len(), logCap)
	}
	if b.dropped != 5 {
		t.Fatalf("丢弃计数 %d，应为 5", b.dropped)
	}
	first, ok := b.get(0)
	if !ok || first.line.S != "5" {
		t.Fatalf("队首应是 5，实为 %+v", first)
	}
}

func TestApplyMapKeepsExitsWhenLookReturnsBareNode(t *testing.T) {
	st := newAppState()
	full := &mapView{Loc: "a", Name: "甲", Exits: []exitView{exit("b", "乙", 3, true)}}
	st.applyMap(full)
	look := &mapView{Loc: "a", Name: "甲", Desc: "一条小路"}
	st.applyMap(look)
	if len(st.world.Exits) != 1 {
		t.Fatalf("look 把出口覆盖没了：%+v", st.world.Exits)
	}
	if st.world.Desc != "一条小路" {
		t.Fatalf("look 的描述没合进来")
	}
}

func TestApplyStateIsFullSnapshot(t *testing.T) {
	st := newAppState()
	st.applyState(&playerView{Name: "甲", HP: 10, MaxHP: 50, Cultivating: true})
	st.applyState(&playerView{Name: "甲", HP: 30, MaxHP: 50})
	if st.vitals.Cultivating {
		t.Fatal("state 是全量快照，cultivating 应被覆盖为 false")
	}
	if st.vitals.HP != 30 {
		t.Fatalf("HP 应为 30，实为 %d", st.vitals.HP)
	}
}

func TestApplyVitalPatchesPlayer(t *testing.T) {
	st := newAppState()
	st.applyState(&playerView{Name: "甲", HP: 10, MaxHP: 50})
	st.applyVital(&vitalView{HP: 44, MP: 7, Stamina: 88})
	if st.player.HP != 44 || st.vitals.Stamina != 88 {
		t.Fatalf("vital 未回写：player.HP=%d vitals.Stamina=%d", st.player.HP, st.vitals.Stamina)
	}
}

func TestGraphObserveAndLayout(t *testing.T) {
	g := newMapGraph()
	m := &mapView{
		Loc: "a", Name: "甲", Terrain: "town", X: intp(10), Y: intp(10),
		Exits: []exitView{
			{To: "b", Name: strp("乙"), Terrain: "plain", CostSec: 3, OK: true, X: intp(12), Y: intp(10)},
			{To: "c", Name: strp("丙"), Terrain: "woods", CostSec: 5, OK: true, X: intp(10), Y: intp(13)},
		},
	}
	g.observe(m)
	if len(g.nodes) != 3 || len(g.edges) != 2 {
		t.Fatalf("节点 %d 边 %d", len(g.nodes), len(g.edges))
	}
	if !g.nodes["a"].visited || g.nodes["b"].visited {
		t.Fatal("到过/望见的标记错了")
	}
	layout := g.layout("a")
	if len(layout) != 3 {
		t.Fatalf("布局缺节点：%v", layout)
	}
	if p := layout["a"]; p[0] != 0 || p[1] != 0 {
		t.Fatalf("当前节点应在原点：%v", p)
	}
	// 乙在正东：屏幕 x 为正、y 不变；丙在正北：屏幕 y 为负。
	if p := layout["b"]; p[0] <= 0 || p[1] != 0 {
		t.Fatalf("乙的方位不对：%v", p)
	}
	if p := layout["c"]; p[1] >= 0 {
		t.Fatalf("丙在北方，屏幕 y 应为负：%v", p)
	}
}

func TestGraphLayoutWithoutGeoFallsBackToLayers(t *testing.T) {
	g := newMapGraph()
	g.observe(&mapView{Loc: "a", Name: "甲", Exits: []exitView{exit("b", "乙", 3, true)}})
	g.observe(&mapView{Loc: "b", Name: "乙", Exits: []exitView{exit("a", "甲", 3, true), exit("c", "丙", 4, true)}})
	layout := g.layout("a")
	if len(layout) != 3 {
		t.Fatalf("布局缺节点：%v", layout)
	}
	if layout["a"][0] != 0 || layout["a"][1] != 0 {
		t.Fatal("无坐标时当前节点也应在原点")
	}
	if layout["b"][0] == 0 && layout["b"][1] == 0 {
		t.Fatal("邻居不该与当前节点重叠")
	}
}

func TestPositionalDownstream(t *testing.T) {
	// 中文注释里的方位：bearingArrow 的 dx 是东，dy 是北。
	if got := bearingArrow(1, 0); got != "→" {
		t.Errorf("正东应为 →，实为 %s", got)
	}
	if got := bearingArrow(0, 1); got != "↑" {
		t.Errorf("正北应为 ↑，实为 %s", got)
	}
	if got := bearingArrow(-1, 0); got != "←" {
		t.Errorf("正西应为 ←，实为 %s", got)
	}
}
