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
	all := strings.Join(a.render(100, 30).frame(), "\n")
	if !strings.Contains(all, "世界 > 有人在吗▌") {
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
