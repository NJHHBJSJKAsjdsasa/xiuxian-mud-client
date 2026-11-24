package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

// testApp 构造一个不联网的应用：状态全在内存里，渲染可重复。
func testApp() *app {
	a := &app{
		st:   newAppState(),
		ui:   newUIState("ws://127.0.0.1:8080"),
		cfg:  config{Server: "ws://127.0.0.1:8080"},
		cmds: make(chan netCmd, 8),
		evs:  make(chan netEvent, 8),
	}
	a.st.conn = connStatus{kind: connOnline}
	a.st.hasChar = true
	a.st.presence = 1
	a.st.player = &playerView{
		Name: "沈砚", Realm: "凡人", HP: 50, MaxHP: 50, MP: 20, MaxMP: 20,
		Exp: 90, ExpNeed: 120, Stamina: 200, StaminaMax: 200, Stones: 180,
		Atk: 6, Def: 4, Spd: 5, Crit: 0.05, Power: 42, Loc: "qingyun_trail",
		Attrs: attrsView{Ti: 2, Ling: 3, Feng: 1, Hu: 2, Shen: 1}, Free: 2,
		Bag:       []bagItem{{ID: "herb", N: 3, Name: "止血草", Kind: "herb", Sell: 4}},
		SkillBook: []skillItem{{ID: "s1", Name: "引气诀", MP: 4, Power: 1.2, Equipped: true}},
	}
	a.st.vitals = vitalView{HP: 50, MP: 20, Exp: 90, ExpNeed: 120, Stamina: 200}
	a.st.world = &mapView{
		Loc: "qingyun_trail", Name: "青云·樵径", RegionName: "青云郡",
		Terrain: "plain", Explored: 5, X: intp(12), Y: intp(34),
		Season: strp("spring"), Weather: strp("rain"), Daypart: strp("day"),
		Exits: []exitView{
			{To: "a", Name: strp("中州·落凡台"), Terrain: "town", CostSec: 10, OK: true, X: intp(12), Y: intp(35)},
			{To: "b", Name: strp("青云·浅滩"), Terrain: "river", CostSec: 4, OK: true, X: intp(11), Y: intp(33)},
			{To: "c", Name: strp("青云观"), Terrain: "gate", CostSec: 5, OK: false, X: intp(13), Y: intp(34)},
		},
	}
	a.st.graph.observe(a.st.world)
	a.st.pushLogs([]logLine{
		{K: "sys", S: "你来到青云·樵径。"},
		{K: "nar", S: "【旁白】坊市·掌柜：布告栏上每天换三条，跑得动就接。"},
		{K: "good", S: "采到止血草 ×1。"},
	})
	a.ui.view = viewMain
	a.ui.page = pageGame
	return a
}

func assertFrameWidth(t *testing.T, a *app, w, h int) []string {
	t.Helper()
	frame := a.render(w, h).frame()
	if len(frame) != h {
		t.Fatalf("行数 %d，应为 %d", len(frame), h)
	}
	for i, line := range frame {
		if got := textWidth(line); got != w {
			t.Fatalf("第 %d 行宽 %d，应为 %d：%q", i, got, w, line)
		}
	}
	return frame
}

func TestRenderGameLayout(t *testing.T) {
	a := testApp()
	frame := assertFrameWidth(t, a, 100, 30)
	all := strings.Join(frame, "\n")

	if !strings.Contains(frame[0], "九州问道") || !strings.Contains(frame[0], "青云·樵径") {
		t.Errorf("标题栏缺品牌或地名：%q", frame[0])
	}
	if !strings.Contains(frame[0], "在线") || !strings.Contains(frame[0], "沈砚") {
		t.Errorf("标题栏缺连接状态或道号：%q", frame[0])
	}
	if strings.Count(all, "┌") < 2 || strings.Count(all, "┘") < 2 {
		t.Error("至少应有状态/去处/见闻三个盒子")
	}
	if !strings.Contains(all, "气血") || !strings.Contains(all, "█") || !strings.Contains(all, "░") {
		t.Error("状态盒缺量表")
	}
	if !strings.Contains(all, "中州·落凡台") || !strings.Contains(all, "10s") {
		t.Error("去处盒缺地名或耗时")
	}
	if !strings.Contains(all, "▸") {
		t.Error("去处列表缺选中标记")
	}
	if !strings.Contains(all, "布告栏上每天换三条") {
		t.Error("见闻盒缺日志")
	}
	if !strings.Contains(all, "寻个去处") || !strings.Contains(all, "F1–F8 页面") {
		t.Error("底部缺目标行或按键行")
	}
}

func TestRenderLoginPage(t *testing.T) {
	a := testApp()
	a.ui.view = viewLogin
	frame := assertFrameWidth(t, a, 80, 24)
	all := strings.Join(frame, "\n")
	for _, want := range []string{"登录", "九州问道", "服务器", "道号", "密码", "Ctrl+R"} {
		if !strings.Contains(all, want) {
			t.Errorf("登录页缺 %q", want)
		}
	}
	if !strings.Contains(all, "┌") {
		t.Error("登录页应是带边框的盒子")
	}
}

func TestRenderCreatePage(t *testing.T) {
	a := testApp()
	a.ui.view = viewCreate
	a.ui.createName = "沈砚"
	a.ui.createCur = len([]rune(a.ui.createName))
	all := strings.Join(assertFrameWidth(t, a, 80, 24), "\n")
	if !strings.Contains(all, "踏入凡界") || !strings.Contains(all, "沈砚") {
		t.Error("建号页缺标题或输入内容")
	}
}

func TestRenderAllPages(t *testing.T) {
	a := testApp()
	for _, p := range []page{pageCharacter, pageBag, pageSkills, pageSocial, pageMarket, pageSect, pageMap, pageSettings} {
		a.ui.page = p
		frame := assertFrameWidth(t, a, 100, 30)
		if !strings.Contains(frame[0], p.label()) {
			t.Errorf("%s 页标题不对：%q", p.label(), frame[0])
		}
	}
}

func TestRenderCharacterPageContent(t *testing.T) {
	a := testApp()
	a.ui.page = pageCharacter
	all := strings.Join(a.render(100, 30).frame(), "\n")
	for _, want := range []string{"境界", "灵根", "战力", "神识", "功德轮回", "丹毒"} {
		if !strings.Contains(all, want) {
			t.Errorf("角色页缺 %q", want)
		}
	}
}

func TestRenderBagPageContent(t *testing.T) {
	a := testApp()
	a.ui.page = pageBag
	all := strings.Join(a.render(100, 30).frame(), "\n")
	if !strings.Contains(all, "止血草") || !strings.Contains(all, "材料") {
		t.Error("背包页缺物品或类别")
	}
}

func TestRenderMapPageLegend(t *testing.T) {
	a := testApp()
	a.ui.page = pageMap
	all := strings.Join(a.render(100, 30).frame(), "\n")
	if !strings.Contains(all, "◎ 当前") || !strings.Contains(all, "只是望见") {
		t.Error("舆图缺图例")
	}
	if !strings.Contains(all, "◎") {
		t.Error("舆图应画出当前节点")
	}
}

func TestRenderTinyTerminalDoesNotPanic(t *testing.T) {
	a := testApp()
	assertFrameWidth(t, a, 40, 12)
	a.ui.typing = true
	a.ui.chatText = "你好"
	a.ui.chatCur = len([]rune(a.ui.chatText))
	assertFrameWidth(t, a, 40, 12)
}

func TestRenderNarrowUsesStatusLine(t *testing.T) {
	a := testApp()
	frame := assertFrameWidth(t, a, 66, 22)
	found := false
	for _, l := range frame {
		if strings.Contains(l, "气血") && strings.Contains(l, "真气") && strings.Contains(l, "修为") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("窄屏缺单行状态摘要：\n%s", strings.Join(frame, "\n"))
	}
}

func TestRenderTypingHint(t *testing.T) {
	a := testApp()
	a.ui.typing = true
	a.ui.chatText = "有人在吗"
	a.ui.chatCur = len([]rune(a.ui.chatText))
	all := strings.Join(a.render(100, 30).frame(), "\n")
	if !strings.Contains(all, "世界> 有人在吗▌") {
		t.Error("输入行未显示频道与内容")
	}
}

func TestRenderObjectiveTravelling(t *testing.T) {
	a := testApp()
	a.st.player.Travel = &travelView{To: "b", Name: "青云·浅滩", ArriveAt: a.st.serverNow() + 7000}
	all := strings.Join(a.render(100, 30).frame(), "\n")
	if !strings.Contains(all, "赶路中") || !strings.Contains(all, "前往 青云·浅滩") {
		t.Error("赶路时目标行未更新")
	}
	if !strings.Contains(all, "00:07") {
		t.Error("倒计时显示不对")
	}
}

func TestRenderOfflineBadge(t *testing.T) {
	a := testApp()
	a.st.conn = connStatus{kind: connOffline, detail: "已退出登录"}
	frame := assertFrameWidth(t, a, 100, 30)
	if !strings.Contains(frame[0], "已退出登录") {
		t.Errorf("标题栏应显示离线原因：%q", frame[0])
	}
}

func TestRenderToast(t *testing.T) {
	a := testApp()
	a.ui.toast("连接中断，正在重连…", true)
	all := strings.Join(a.render(100, 30).frame(), "\n")
	if !strings.Contains(all, "连接中断，正在重连…") {
		t.Error("toast 未画出")
	}
}

func TestSecondFrameSkipsUnchangedLines(t *testing.T) {
	a := testApp()
	var b1 bytes.Buffer
	w1 := bufio.NewWriter(&b1)
	a.render(100, 30).render(w1, colorOff)
	if b1.Len() == 0 {
		t.Fatal("首帧应有输出")
	}

	// 状态没变：第二帧不该再写一遍整屏。
	var b2 bytes.Buffer
	w2 := bufio.NewWriter(&b2)
	a.render(100, 30).render(w2, colorOff)
	if b2.Len() != 0 {
		t.Fatalf("无变化时第二帧仍写了 %d 字节：%q", b2.Len(), b2.String())
	}

	// 改一条日志：只重画受影响的行，而不是整屏。
	a.st.pushLogs([]logLine{{K: "good", S: "又采到一株。"}})
	var b3 bytes.Buffer
	w3 := bufio.NewWriter(&b3)
	a.render(100, 30).render(w3, colorOff)
	if b3.Len() == 0 || b3.Len() >= b1.Len() {
		t.Fatalf("局部更新应只写少量字节：首帧 %d，本帧 %d", b1.Len(), b3.Len())
	}
}

func TestRenderSettingsPageContent(t *testing.T) {
	a := testApp()
	a.ui.page = pageSettings
	a.ui.desktop = true
	sc := a.render(100, 30)
	all := strings.Join(sc.frame(), "\n")
	for _, want := range []string{"服务器", "协议", "连接", "配置", "playcli.json", "窗口", "无边框"} {
		if !strings.Contains(all, want) {
			t.Errorf("设置页缺 %q", want)
		}
	}
	for _, h := range sc.hits {
		if h.act.kind == hitWindowToggle {
			return
		}
	}
	t.Fatal("设置页没有窗口切换的点击区")
}

func TestRenderBattleBox(t *testing.T) {
	a := battleApp()
	frame := assertFrameWidth(t, a, 100, 30)
	all := strings.Join(frame, "\n")
	for _, want := range []string{"战斗 · 第 1 回合", "山兔精", "黑风狼", "气血", "白露"} {
		if !strings.Contains(all, want) {
			t.Errorf("战斗盒缺 %q", want)
		}
	}
	if !strings.Contains(all, "a 普攻") || !strings.Contains(all, "f 逃") {
		t.Error("战斗时按键行没换成战斗键位")
	}
	if strings.Contains(all, "去处") {
		t.Error("战斗时不该再画去处盒")
	}
}

func TestRenderEncounterOverlay(t *testing.T) {
	a := encounterApp()
	frame := assertFrameWidth(t, a, 100, 30)
	all := strings.Join(frame, "\n")
	for _, want := range []string{"奇遇", "静心打坐", "翻开石缝", "按 1–3 选择"} {
		if !strings.Contains(all, want) {
			t.Errorf("奇遇弹层缺 %q", want)
		}
	}
}

func TestRenderBagCursor(t *testing.T) {
	a := richApp()
	a.ui.page = pageBag
	a.ui.setSel(pageBag, 1)
	all := strings.Join(assertFrameWidth(t, a, 100, 22), "\n")
	if !strings.Contains(all, "▸ 青锋剑") {
		t.Errorf("选中行应有 ▸ 标记：\n%s", all)
	}
	if !strings.Contains(all, "↑↓ 选择") || !strings.Contains(all, "s 出售") {
		t.Error("背包底栏没写清可用按键")
	}
}

func TestRenderSkillsSlotCounter(t *testing.T) {
	a := richApp()
	a.ui.page = pageSkills
	all := strings.Join(assertFrameWidth(t, a, 100, 20), "\n")
	if !strings.Contains(all, "技能槽 1/2") {
		t.Errorf("技能页应显示技能槽占用：\n%s", all)
	}
	if !strings.Contains(all, "▸ 剑气诀") && !strings.Contains(all, "剑气诀") {
		t.Error("技能列表没出来")
	}
}

func TestRenderMarketPage(t *testing.T) {
	a := richApp()
	a.ui.page = pageMarket
	a.ui.setSel(pageMarket, 2)
	all := strings.Join(assertFrameWidth(t, a, 100, 20), "\n")
	for _, want := range []string{"货物", "回气丹", "凝神丹", "需阶", "数量 1", "修炼速度"} {
		if !strings.Contains(all, want) {
			t.Errorf("坊市页缺 %q：\n%s", want, all)
		}
	}
}

func TestRenderHelpOverlay(t *testing.T) {
	a := richApp()
	a.ui.help = true
	all := strings.Join(assertFrameWidth(t, a, 100, 30), "\n")
	for _, want := range []string{"帮助", "行走", "私语", "装配/卸下", "i 丹药", "q 退出（按两次）"} {
		if !strings.Contains(all, want) {
			t.Errorf("帮助浮层缺 %q：\n%s", want, all)
		}
	}
}

func TestRenderBlockedExitMarked(t *testing.T) {
	a := richApp()
	a.ui.selectedExit = 0
	all := strings.Join(assertFrameWidth(t, a, 100, 30), "\n")
	if !strings.Contains(all, "× → 青云观") {
		t.Errorf("走不通的出口应标 ×：\n%s", all)
	}
}

func TestRenderLogScrollTitle(t *testing.T) {
	a := richApp()
	a.logBack = 3
	all := strings.Join(assertFrameWidth(t, a, 100, 30), "\n")
	if !strings.Contains(all, "上翻 3 行") {
		t.Errorf("上翻时见闻标题应提示：\n%s", all)
	}
}

func TestRenderToastOnDataPage(t *testing.T) {
	a := richApp()
	a.ui.page = pageBag
	a.ui.toast("这件不是装备，穿不上", true)
	all := strings.Join(assertFrameWidth(t, a, 90, 16), "\n")
	if !strings.Contains(all, "这件不是装备") {
		t.Errorf("数据页 toast 没画出来：\n%s", all)
	}
	if strings.Contains(all, "└─────") && strings.Count(all, "└") != 1 {
		t.Log("仅提示用")
	}
}

func TestRenderTalkOverlay(t *testing.T) {
	a := richApp()
	a.st.world.NPCs = []npcView{{ID: "npc_zhanggui", Name: "掌柜", Title: "坊市"}}
	a.ui.talkPick = true
	all := strings.Join(assertFrameWidth(t, a, 100, 26), "\n")
	for _, want := range []string{"交谈", "掌柜", "坊市", "Enter 说话"} {
		if !strings.Contains(all, want) {
			t.Errorf("交谈弹层缺 %q：\n%s", want, all)
		}
	}
}

func TestMapPageIsDeterministic(t *testing.T) {
	a := testApp()
	a.ui.page = pageMap
	first := strings.Join(assertFrameWidth(t, a, 100, 30), "\n")
	for i := 0; i < 12; i++ {
		got := strings.Join(assertFrameWidth(t, a, 100, 30), "\n")
		if got != first {
			t.Fatalf("第 %d 次重绘与首帧不同（舆图内容必须每帧一致）：\n%s", i+2, got)
		}
	}

	// 同一份数据重绘不该写出任何字节：脏行跳过依赖内容稳定。
	var b1, b2 bytes.Buffer
	w1 := bufio.NewWriter(&b1)
	a.render(100, 30).render(w1, colorOff)
	if b1.Len() == 0 {
		t.Fatal("首帧应有输出")
	}
	w2 := bufio.NewWriter(&b2)
	a.render(100, 30).render(w2, colorOff)
	if b2.Len() != 0 {
		t.Fatalf("舆图第二帧仍写了 %d 字节：%q", b2.Len(), b2.String())
	}
}

func TestSortedNodesCurrentWinsOnSameCell(t *testing.T) {
	g := newMapGraph()
	m := &mapView{
		Loc: "a", Name: "甲", X: intp(5), Y: intp(5),
		Exits: []exitView{{To: "b", Name: strp("乙"), OK: true, X: intp(5), Y: intp(5)}},
	}
	g.observe(m)
	layout := g.layout("a")
	pos := func(px, py float64) (int, int) { return int(px) + 10, int(py) + 10 }

	for i := 0; i < 12; i++ {
		nodes := sortedNodes(g, layout, "a", pos, 0, 0, 100, 30)
		if len(nodes) != 2 {
			t.Fatalf("应有 2 个节点，实为 %d", len(nodes))
		}
		if nodes[len(nodes)-1].id != "a" || nodes[len(nodes)-1].ch != '◎' {
			t.Fatalf("同格时当前节点应最后画：%+v", nodes)
		}
	}
}

func TestSortedEdgesBlockedFirst(t *testing.T) {
	g := newMapGraph()
	g.edges[[2]string{"a", "b"}] = &mapEdge{ok: true}
	g.edges[[2]string{"a", "c"}] = &mapEdge{ok: false}
	g.edges[[2]string{"a", "d"}] = &mapEdge{ok: true}
	lines := sortedEdges(g)
	if len(lines) != 3 || lines[0].e.ok {
		t.Fatalf("断路应排在通路前面：%+v", lines)
	}
	if lines[1].key[1] != "b" || lines[2].key[1] != "d" {
		t.Fatalf("通路按 id 排序不稳定：%+v", lines)
	}
}
