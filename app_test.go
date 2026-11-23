package main

import "testing"

// 这一组盯的是"选好的去处被无故打回第一项"。
// 触发过的事故：state 帧没有 loc，客户端以为地图与角色位置永远不一致，
// 每 600ms 补拉一次 worldMap，每次应答都把 selectedExit 清零。

func TestSameLocMapKeepsSelection(t *testing.T) {
	a := testApp()
	a.ui.selectedExit = 2

	// look 的应答：同一节点、没有出口。
	look := &mapView{Loc: a.st.world.Loc, Name: a.st.world.Name, Desc: "一条小路"}
	a.handleEvent(evMap{m: look})

	if a.ui.selectedExit != 2 {
		t.Fatalf("同一节点的 map 帧不该改选中项，实为 %d", a.ui.selectedExit)
	}
}

func TestNewLocMapResetsSelection(t *testing.T) {
	a := testApp()
	a.ui.selectedExit = 2
	moved := &mapView{Loc: "loc_other", Name: "别处", Exits: []exitView{exit("x", "某地", 3, true)}}
	a.handleEvent(evMap{m: moved})
	if a.ui.selectedExit != 0 {
		t.Fatalf("换节点后选中项应归零，实为 %d", a.ui.selectedExit)
	}
}

func TestDataPageIgnoresNavAndEnter(t *testing.T) {
	a := testApp()
	a.ui.page = pageMap
	a.ui.selectedExit = 1

	a.onKey(special("down"))
	if a.ui.selectedExit != 1 {
		t.Fatalf("数据页按 ↓ 不该改主界面选中项，实为 %d", a.ui.selectedExit)
	}

	a.onKey(special("enter"))
	select {
	case c := <-a.cmds:
		t.Fatalf("数据页按 Enter 不该发命令：%+v", c)
	default:
	}

	// Esc 仍然要能回主界面。
	a.onKey(special("esc"))
	if a.ui.page != pageGame {
		t.Fatal("Esc 应回到主界面")
	}
}

func TestScrollLogsStopsAtBounds(t *testing.T) {
	a := testApp()
	a.onKey(special("pgup"))
	if a.logBack == 0 {
		t.Fatal("PgUp 应向上翻")
	}
	before := a.logBack
	a.onKey(special("pgup"))
	a.onKey(special("pgup"))
	if a.logBack < before {
		t.Fatal("连续 PgUp 不该回退")
	}
	if a.logBack > a.st.logs.len()-1 {
		t.Fatalf("翻过界了：%d > %d", a.logBack, a.st.logs.len()-1)
	}
	a.onKey(special("end"))
	if a.logBack != 0 {
		t.Fatal("End 应回到最新")
	}
}

func TestRefreshStaleMapOnlyWhenLocDiffers(t *testing.T) {
	a := testApp()
	a.tick()
	select {
	case c := <-a.cmds:
		t.Fatalf("位置一致时不该补拉世界地图：%+v", c)
	default:
	}

	a.st.player.Loc = "loc_other"
	a.tick()
	select {
	case c := <-a.cmds:
		if c.action != actWorldMap {
			t.Fatalf("应发 worldMap，实为 %q", c.action)
		}
	default:
		t.Fatal("位置不一致时应补拉世界地图")
	}
}
