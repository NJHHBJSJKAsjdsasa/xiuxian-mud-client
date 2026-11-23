package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestWideRuneTakesTwoCells(t *testing.T) {
	s := newScreen(10, 1)
	s.put(0, 0, "青云a", fg(colText))
	// "青" 占 2 格、"云" 占 2 格、"a" 占 1 格：第 5 格是 a。
	if got := s.buf[4].ch; got != 'a' {
		t.Fatalf("第 5 格应是 a，实为 %q", got)
	}
	if got := s.buf[5].ch; got != ' ' {
		t.Fatalf("a 之后应是空格，实为 %q", got)
	}
	// 续格必须标记为 0，否则渲染时会被当成空格多打一列。
	if s.buf[1].ch != 0 || s.buf[3].ch != 0 {
		t.Fatalf("宽字符续格未标记：%q %q", s.buf[1].ch, s.buf[3].ch)
	}
}

func TestRowsAreEvenWidth(t *testing.T) {
	s := newScreen(40, 6)
	s.put(0, 1, "青云问道 青云·樵径", fg(colGold))
	for i, line := range s.frame() {
		if w := textWidth(line); w != 40 {
			t.Errorf("第 %d 行宽 %d，应为 40：%q", i, w, line)
		}
	}
}

func TestTruncationDoesNotSplitWideRune(t *testing.T) {
	s := newScreen(5, 1)
	s.put(0, 0, "青云问道阁", fg(colText))
	// 宽度 5 只放得下两个汉字 + 一列空格；不能出现半个字。
	line := s.row(0)
	if w := textWidth(line); w != 5 {
		t.Fatalf("行宽 %d：%q", w, line)
	}
	if !strings.HasPrefix(line, "青云") {
		t.Fatalf("内容错位：%q", line)
	}
}

func TestRenderSkipsUnchangedLines(t *testing.T) {
	s := newScreen(10, 2)
	s.put(0, 0, "甲", fg(colText))
	s.put(0, 1, "乙", fg(colText))
	var first bytes.Buffer
	w := bufio.NewWriter(&first)
	s.render(w, colorOff)
	if strings.Count(first.String(), "\x1b[") < 2 {
		t.Fatalf("首帧应重画两行：%q", first.String())
	}

	// 只改第二行：第二帧不应再碰第一行。
	s.put(0, 1, "丙", fg(colText))
	var second bytes.Buffer
	w = bufio.NewWriter(&second)
	s.render(w, colorOff)
	if strings.Contains(second.String(), "\x1b[1;1H") {
		t.Fatalf("第一行没变却被重画：%q", second.String())
	}
	if !strings.Contains(second.String(), "\x1b[2;1H") {
		t.Fatalf("第二行变了却没重画：%q", second.String())
	}
}

func TestParseColorMode(t *testing.T) {
	if parseColorMode("off") != colorOff || parseColorMode("256") != color256 || parseColorMode("true") != colorTrue {
		t.Fatal("颜色模式解析错")
	}
	if parseColorMode("") != colorTrue {
		t.Fatal("缺省应真彩")
	}
}
