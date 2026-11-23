package main

import "testing"

func TestTextWidth(t *testing.T) {
	cases := []struct {
		s string
		w int
	}{
		{"abc", 3},
		{"青云", 4},
		{"青云 abc", 8},
		{"", 0},
		{"·", 1},
		{"…", 1},
	}
	for _, c := range cases {
		if got := textWidth(c.s); got != c.w {
			t.Errorf("textWidth(%q)=%d, 期望 %d", c.s, got, c.w)
		}
	}
}

func TestTruncateWidthNeverOverflows(t *testing.T) {
	for max := 0; max < 14; max++ {
		got := truncateWidth("青云·望云台 12s", max)
		if w := textWidth(got); w > max && max >= 1 {
			t.Errorf("max=%d 时截断结果宽 %d：%q", max, w, got)
		}
	}
	if got := truncateWidth("短", 4); got != "短" {
		t.Errorf("不该截断，得到 %q", got)
	}
}

func TestTruncateChars(t *testing.T) {
	if got := truncateChars("青云·望云台", 3); got != "青云…" {
		t.Errorf("得到 %q", got)
	}
	if got := truncateChars("短", 3); got != "短" {
		t.Errorf("得到 %q", got)
	}
}

func TestPadWidth(t *testing.T) {
	if got := padWidth("青云", 6); got != "青云  " {
		t.Errorf("得到 %q", got)
	}
	if w := textWidth(padWidth("青云", 6)); w != 6 {
		t.Errorf("补齐后宽 %d", w)
	}
}

func TestThousands(t *testing.T) {
	cases := map[int64]string{
		0: "0", 999: "999", 1000: "1,000", 12340: "12,340", 1234567: "1,234,567", -4200: "-4,200",
	}
	for n, want := range cases {
		if got := thousands(n); got != want {
			t.Errorf("thousands(%d)=%q, 期望 %q", n, got, want)
		}
	}
}

func TestCountdown(t *testing.T) {
	cases := map[int64]string{3400: "00:04", 0: "00:00", -5: "00:00", 61000: "01:01"}
	for ms, want := range cases {
		if got := fmtCountdown(ms); got != want {
			t.Errorf("fmtCountdown(%d)=%q, 期望 %q", ms, got, want)
		}
	}
}

func TestGauge(t *testing.T) {
	if got := gauge(50, 100, 4); got != "██░░" {
		t.Errorf("得到 %q", got)
	}
	if got := gauge(0, 0, 3); got != "░░░" {
		t.Errorf("max=0 时应全空，得到 %q", got)
	}
	if got := gauge(200, 200, 3); got != "███" {
		t.Errorf("满血应全实心，得到 %q", got)
	}
}
