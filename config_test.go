package main

import "testing"

func TestWindowModeDefaultsToBorderless(t *testing.T) {
	cases := []struct {
		mode string
		want bool
	}{
		{"", true},
		{windowModeBorderless, true},
		{windowModeWindowed, false},
	}
	for _, tc := range cases {
		got := (config{WindowMode: tc.mode}).borderless()
		if got != tc.want {
			t.Errorf("WindowMode=%q borderless=%v，想要 %v", tc.mode, got, tc.want)
		}
	}
}
