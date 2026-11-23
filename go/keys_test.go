package main

import "testing"

func TestParseArrowKeys(t *testing.T) {
	ks := parseKeys([]byte("\x1b[A\x1b[B\x1b[C\x1b[D"))
	want := []string{"up", "down", "right", "left"}
	if len(ks) != len(want) {
		t.Fatalf("切出 %d 个键，期望 %d：%+v", len(ks), len(want), ks)
	}
	for i, w := range want {
		if !ks[i].is(w) {
			t.Errorf("第 %d 个键是 %+v，期望 %s", i, ks[i], w)
		}
	}
}

func TestParseSingleEscIsEsc(t *testing.T) {
	ks := parseKeys([]byte{0x1b})
	if len(ks) != 1 || !ks[0].is("esc") {
		t.Fatalf("单独 ESC 应识别为 esc：%+v", ks)
	}
}

func TestParseFKeys(t *testing.T) {
	cases := map[string]string{
		"\x1bOP":   "f1",
		"\x1b[12~": "f2",
		"\x1b[15~": "f5",
		"\x1b[19~": "f8",
		"\x1b[24~": "f12",
	}
	for seq, want := range cases {
		ks := parseKeys([]byte(seq))
		if len(ks) != 1 || !ks[0].is(want) {
			t.Errorf("%q 应识别为 %s，实为 %+v", seq, want, ks)
		}
	}
}

func TestParsePagingAndHomeEnd(t *testing.T) {
	ks := parseKeys([]byte("\x1b[5~\x1b[6~\x1b[H\x1b[F"))
	want := []string{"pgup", "pgdn", "home", "end"}
	if len(ks) != len(want) {
		t.Fatalf("切出 %d 个键：%+v", len(ks), ks)
	}
	for i, w := range want {
		if !ks[i].is(w) {
			t.Errorf("第 %d 个是 %+v，期望 %s", i, ks[i], w)
		}
	}
}

func TestParseUTF8AndControl(t *testing.T) {
	ks := parseKeys([]byte("青\x12\x03\x7f"))
	if len(ks) != 4 {
		t.Fatalf("切出 %d 个键：%+v", len(ks), ks)
	}
	if !ks[0].isRune('青') {
		t.Errorf("第一个应是汉字青：%+v", ks[0])
	}
	if !(ks[1].kind == keySpecial && ks[1].ctrl && ks[1].name == "r") {
		t.Errorf("Ctrl+R 解析错：%+v", ks[1])
	}
	if !(ks[2].kind == keySpecial && ks[2].ctrl && ks[2].name == "c") {
		t.Errorf("Ctrl+C 解析错：%+v", ks[2])
	}
	if !ks[3].is("backspace") {
		t.Errorf("0x7f 应是退格：%+v", ks[3])
	}
}

func TestPageFromFnKey(t *testing.T) {
	if p, ok := pageFromFn("f1"); !ok || p != pageCharacter {
		t.Fatalf("F1 应到角色页，实为 %v %v", p, ok)
	}
	if p, ok := pageFromFn("f8"); !ok || p != pageSettings {
		t.Fatalf("F8 应到设置页，实为 %v %v", p, ok)
	}
	if _, ok := pageFromFn("f12"); ok {
		t.Fatal("F12 不该映射到页面")
	}
}

func TestUnknownCSISwallowed(t *testing.T) {
	// Ctrl+↑ 在多数终端是 ESC[1;5A：认不出就整段吞掉，不能漏成 '1' ';' '5' 'A'。
	ks := parseKeys([]byte("\x1b[1;5A你"))
	if len(ks) != 2 {
		t.Fatalf("应只剩下 ESC 与一个汉字，实为 %+v", ks)
	}
	if !ks[0].is("esc") || !ks[1].isRune('你') {
		t.Fatalf("解析结果不对：%+v", ks)
	}
}
