package main

import (
	"strings"
	"testing"
)

func TestInputCursorEditing(t *testing.T) {
	u := newUIState("ws://127.0.0.1:8080")
	u.view = viewLogin
	u.login.setFocus(1)
	u.login.name = "abcd"
	u.login.nameCur = 2

	u.appendRune('X')
	if u.login.name != "abXcd" || u.login.nameCur != 3 {
		t.Fatalf("中间插入应得 abXcd/3，实际 %q/%d", u.login.name, u.login.nameCur)
	}
	u.editBackspace()
	if u.login.name != "abcd" || u.login.nameCur != 2 {
		t.Fatalf("退格应得 abcd/2，实际 %q/%d", u.login.name, u.login.nameCur)
	}
	u.editDelete()
	if u.login.name != "abd" || u.login.nameCur != 2 {
		t.Fatalf("Delete 应删光标后一个字，实际 %q/%d", u.login.name, u.login.nameCur)
	}
	u.editHome()
	u.editDelete()
	u.editEnd()
	u.appendRune('!')
	if u.login.name != "bd!" || u.login.nameCur != 3 {
		t.Fatalf("Home/End 后应得 bd!/3，实际 %q/%d", u.login.name, u.login.nameCur)
	}
	u.editMove(-99)
	if u.login.nameCur != 0 {
		t.Fatalf("光标应夹到 0，实际 %d", u.login.nameCur)
	}
	u.editMove(99)
	if u.login.nameCur != 3 {
		t.Fatalf("光标应夹到末尾 3，实际 %d", u.login.nameCur)
	}
}

func TestLoginPrefillCursorAtEnd(t *testing.T) {
	u := newUIState("ws://x")
	u.login.name = "沈砚"
	u.login.syncCursors()
	if u.login.serverCur != len([]rune(u.login.server)) || u.login.nameCur != 2 {
		t.Fatalf("回填后光标应在末尾：server=%d name=%d", u.login.serverCur, u.login.nameCur)
	}
}

func TestHistoryKeepsDraft(t *testing.T) {
	u := newUIState("ws://127.0.0.1:8080")
	u.typing = true
	u.inputHist = []string{"one", "two"}
	u.inputAt = 2
	u.chatText, u.chatCur = "draft", 5

	u.histStep(-1)
	if u.chatText != "two" || u.chatCur != 3 {
		t.Fatalf("上翻应到 two/3，实际 %q/%d", u.chatText, u.chatCur)
	}
	u.histStep(-1)
	if u.chatText != "one" {
		t.Fatalf("再上翻应到 one，实际 %q", u.chatText)
	}
	u.histStep(1)
	u.histStep(1)
	if u.chatText != "draft" || u.chatCur != 5 {
		t.Fatalf("回到末尾应还原草稿 draft/5，实际 %q/%d", u.chatText, u.chatCur)
	}
}

func TestParseDeleteKey(t *testing.T) {
	ks := parseKeys([]byte("\x1b[3~"))
	if len(ks) != 1 || !ks[0].is("delete") {
		t.Fatalf("ESC[3~ 应识别为 delete：%+v", ks)
	}
}

func TestRenderInputCursor(t *testing.T) {
	a := testApp()
	a.ui.typing = true
	a.ui.chatText = "abcdef"
	a.ui.chatCur = 3
	frame := strings.Join(a.render(100, 30).frame(), "\n")
	if !strings.Contains(frame, "abc▌def") {
		t.Error("聊天输入行的光标应出现在第 3 个字之后")
	}

	a.ui.view = viewLogin
	a.ui.typing = false
	a.ui.login.setFocus(1)
	a.ui.login.name = "甲乙"
	a.ui.login.nameCur = 1
	frame = strings.Join(a.render(80, 24).frame(), "\n")
	t.Logf("LOGIN FRAME:\n%s", frame)
	if !strings.Contains(frame, "甲▌乙") {
		t.Error("登录道号字段的光标应出现在两个字之间")
	}
}

func TestBattleTargetSkipsDead(t *testing.T) {
	a := testApp()
	a.st.battle = &battleView{
		ID: "b1", State: "player",
		Foes: []foeView{{I: 1, Dead: true}, {I: 2, Dead: false}},
	}
	a.ui.target = 1
	a.keyBattle(key{kind: keyRune, r: '1'}, a.st.battle)
	if a.ui.target != 1 {
		t.Fatalf("点到已倒下的目标不应改选，实际 %d", a.ui.target)
	}
	a.keyBattle(key{kind: keyRune, r: '2'}, a.st.battle)
	if a.ui.target != 2 {
		t.Fatalf("点活着的 2 号应选中 2，实际 %d", a.ui.target)
	}
}

func TestBattleTargetResetsPerFight(t *testing.T) {
	a := testApp()
	a.ui.target = 3

	a.handleEvent(evBattle{b: &battleView{ID: "b1", Foes: []foeView{{I: 1}, {I: 2}}}})
	if a.ui.target != 1 {
		t.Fatalf("新一场战斗应把目标重置为 1，实际 %d", a.ui.target)
	}

	a.ui.target = 2
	a.handleEvent(evBattle{b: &battleView{ID: "b1", Foes: []foeView{{I: 1}, {I: 2, Dead: true}}}})
	if a.ui.target != 1 {
		t.Fatalf("当前目标倒下后应改选第一只活着的 1，实际 %d", a.ui.target)
	}
}

func TestBattleTargetCycleKeys(t *testing.T) {
	a := testApp()
	a.st.battle = &battleView{ID: "b1", Foes: []foeView{{I: 1}, {I: 2}}}
	a.ui.target = 1
	a.keyBattle(special("tab"), a.st.battle)
	if a.ui.target != 2 {
		t.Fatalf("Tab 应切到 2，实际 %d", a.ui.target)
	}
	a.keyBattle(special("left"), a.st.battle)
	if a.ui.target != 1 {
		t.Fatalf("← 应切回 1，实际 %d", a.ui.target)
	}
}

func TestChatOfflineKeepsDraft(t *testing.T) {
	a := testApp()
	a.st.conn = connStatus{kind: connOffline, detail: "断线"}
	a.ui.typing = true
	a.ui.chatText, a.ui.chatCur = "半条命令", 4
	a.sendChatInput()
	if a.ui.chatText != "半条命令" || a.ui.chatCur != 4 || !a.ui.typing {
		t.Fatalf("离线发送应保留草稿与输入态，实际 %q/%d/%v", a.ui.chatText, a.ui.chatCur, a.ui.typing)
	}
	if len(a.ui.toasts) == 0 {
		t.Fatal("离线发送应提示")
	}
}

func TestOfflineMoveAndBattleSpeak(t *testing.T) {
	a := testApp()
	a.st.conn = connStatus{kind: connOffline, detail: "断线"}
	a.enterExit()
	a.battleAct(&battleView{State: "player"}, map[string]any{"op": "attack", "target": 1})
	if len(a.ui.toasts) < 2 {
		t.Fatalf("离线赶路/出手都应有提示，实际 toast=%d", len(a.ui.toasts))
	}
	select {
	case c := <-a.cmds:
		t.Fatalf("离线时不应发动作：%+v", c)
	default:
	}
}

func TestFirstAliveFoeTarget(t *testing.T) {
	b := &battleView{Foes: []foeView{{I: 1, Dead: true}, {I: 3, Dead: false}}}
	if got := firstAliveFoeTarget(b); got != 3 {
		t.Fatalf("应选第一只活着的 3，实际 %d", got)
	}
	if got := firstAliveFoeTarget(&battleView{}); got != 1 {
		t.Fatalf("空战斗应回 1，实际 %d", got)
	}
}

func TestSubmitLoginIgnoredWhileLoading(t *testing.T) {
	a := testApp()
	a.ui.view = viewLogin
	a.ui.login.server, a.ui.login.name, a.ui.login.pass = "ws://x", "沈砚", "secret123"
	a.ui.login.loading = true
	a.submitLogin()
	select {
	case c := <-a.cmds:
		t.Fatalf("加载中不应再发登录命令：%+v", c)
	default:
	}
}

func joinFrame(lines []string) string {
	out := ""
	for _, line := range lines {
		out += line + "\n"
	}
	return out
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
