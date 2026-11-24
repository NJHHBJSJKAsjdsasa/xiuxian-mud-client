//go:build windows

package main

import "testing"

func TestVKToKey(t *testing.T) {
	cases := []struct {
		vk   uint32
		want string
	}{
		{0x25, "left"}, {0x26, "up"}, {0x27, "right"}, {0x28, "down"},
		{0x0D, "enter"}, {0x1B, "esc"}, {0x09, "tab"}, {0x08, "backspace"},
		{0x20, "space"}, {0x21, "pgup"}, {0x22, "pgdn"}, {0x24, "home"}, {0x23, "end"},
		{0x70, "f1"}, {0x7B, "f12"},
	}
	for _, c := range cases {
		k, ok := vkToKey(c.vk)
		if !ok || !k.is(c.want) {
			t.Errorf("vk 0x%02X 应映射为 %s，实际 ok=%v key=%+v", c.vk, c.want, ok, k)
		}
	}
	if _, ok := vkToKey(0x41); ok {
		t.Error("字母 A 应走 WM_CHAR，不应在 vkToKey 里映射")
	}
	if _, ok := vkToKey(0x30); ok {
		t.Error("数字 0 应走 WM_CHAR，不应在 vkToKey 里映射")
	}
}

func TestMouseXY(t *testing.T) {
	lp := uintptr(uint16(0xFFFF)) | uintptr(uint16(0xFFFE))<<16
	if x, y := mouseXY(lp); x != -1 || y != -2 {
		t.Errorf("负坐标应解出 (-1,-2)，实际 (%d,%d)", x, y)
	}
	lp = uintptr(3) | uintptr(4)<<16
	if x, y := mouseXY(lp); x != 3 || y != 4 {
		t.Errorf("正坐标应解出 (3,4)，实际 (%d,%d)", x, y)
	}
}

func TestGDIRuneFallback(t *testing.T) {
	cases := map[rune]rune{'▸': '»', '░': '·', '中': '中', '█': '█'}
	for in, want := range cases {
		if got := gdiRune(in); got != want {
			t.Errorf("gdiRune(%q) = %q，应 %q", in, got, want)
		}
	}
}

func TestScreenHashDetectsStyleChanges(t *testing.T) {
	sc := newScreen(2, 1)
	h1 := screenHash(sc)
	sc.buf[0].st.bold = true
	if h2 := screenHash(sc); h2 == h1 {
		t.Error("加粗变化应改变哈希")
	}
	sc.buf[0].st.bold = false
	sc.buf[0].st.hasBG = true
	sc.buf[0].st.bg = rgb{1, 2, 3}
	if h3 := screenHash(sc); h3 == h1 {
		t.Error("背景变化应改变哈希")
	}
}
