package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// 文本工具：显示宽度、截断、补齐。终端里"列"与"字符数"不是一回事，
// 中文与全角标点占两列，混排时必须按宽度算，否则右边界会错位。

// runeWidth 返回一个字符占几列。零宽组合符与不可见字符算 0。
func runeWidth(r rune) int {
	if r == 0 || r < 32 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	// 组合附加符：跟在别的字后面，不占列。
	if (r >= 0x0300 && r <= 0x036f) || (r >= 0x1ab0 && r <= 0x1aff) ||
		(r >= 0x1dc0 && r <= 0x1dff) || (r >= 0x20d0 && r <= 0x20ff) ||
		(r >= 0xfe20 && r <= 0xfe2f) {
		return 0
	}
	if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) || (r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1f64f) || (r >= 0x1f900 && r <= 0x1f9ff) ||
		(r >= 0x20000 && r <= 0x2fffd)) {
		return 2
	}
	return 1
}

// textWidth 返回字符串的显示宽度（列数）。
func textWidth(s string) int {
	n := 0
	for _, r := range s {
		n += runeWidth(r)
	}
	return n
}

// truncateWidth 按显示宽度截断，超出部分用省略号（占两列）收尾。
func truncateWidth(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if textWidth(s) <= max {
		return s
	}
	var b strings.Builder
	used := 0
	ell := runeWidth('…')
	if ell < 1 {
		ell = 1
	}
	for _, r := range s {
		w := runeWidth(r)
		if used+w+ell > max {
			break
		}
		b.WriteRune(r)
		used += w
	}
	b.WriteString("…")
	return b.String()
}

// truncateChars 按字符数截断（不是列），用于日志这类"数得清"的文本。
func truncateChars(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// padWidth 补空格到指定列宽；已超出则原样返回（由调用方决定是否截断）。
func padWidth(s string, w int) string {
	if n := w - textWidth(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

// thousands 给整数加千分位：12340 → 12,340。
func thousands(n int64) string {
	neg := n < 0
	s := strconv.FormatInt(abs64(n), 10)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func percent(cur, max int64) float64 {
	if max <= 0 {
		return 0
	}
	p := float64(cur) / float64(max)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

func nowMs() int64 { return time.Now().UnixMilli() }

// fmtCountdown 毫秒 → MM:SS，向上取整，负数按 0。
func fmtCountdown(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	total := (ms + 999) / 1000
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

// fmtHM 毫秒时间戳 → 东八区 HH:MM（见闻行首的时间）。
func fmtHM(ms int64) string {
	return time.UnixMilli(ms).In(time.FixedZone("CST", 8*3600)).Format("15:04")
}

// fmtHMS 毫秒时间戳 → 东八区 HH:MM:SS。
func fmtHMS(ms int64) string {
	return time.UnixMilli(ms).In(time.FixedZone("CST", 8*3600)).Format("15:04:05")
}
