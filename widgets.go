package main

import (
	"math"
	"strings"
)

// 小部件：盒子、分隔线、列表行、量表。
//
// 界面的骨架全在这里：四个盒子拼成一屏，所有内容都写在盒子内部。
// 只用制表符与方块字符，宽度都是 1 列，任何 UTF-8 等宽字体都能对齐。

// box 画一个边框盒，并在顶边嵌标题。accent=true 时边框更亮（当前焦点）。
func box(sc *screen, x, y, w, h int, title string, accent bool) {
	if w < 2 || h < 2 {
		return
	}
	line := fg(colFaint)
	if accent {
		line = fg(colDim)
	}
	sc.set(x, y, '┌', line)
	sc.set(x+w-1, y, '┐', line)
	sc.set(x, y+h-1, '└', line)
	sc.set(x+w-1, y+h-1, '┘', line)
	for i := 1; i < w-1; i++ {
		sc.set(x+i, y, '─', line)
		sc.set(x+i, y+h-1, '─', line)
	}
	for j := 1; j < h-1; j++ {
		sc.set(x, y+j, '│', line)
		sc.set(x+w-1, y+j, '│', line)
	}
	if title != "" {
		label := " " + title + " "
		if textWidth(label) <= w-4 {
			sc.put(x+2, y, label, fg(colText))
		}
	}
}

// 盒子内部：左右各让一格给边框与留白。
func boxInner(x, w int) (int, int) {
	return x + 2, w - 4
}

// row 把盒子内的一整行铺成同一样式（选中行用它铺反色底）。
func row(sc *screen, x, y, w int, st style) {
	for i := 0; i < w; i++ {
		sc.set(x+i, y, ' ', st)
	}
}

// listRow 画一行"左文右值"：选中时整行反色，右侧数值贴右边界。
func listRow(sc *screen, x, y, w int, left, right string, selected bool) {
	st := fg(colText)
	if selected {
		st = fgBg(colBG, colAccent)
	}
	row(sc, x, y, w, st)
	sc.put(x, y, truncateWidth(left, w), st)
	if right != "" {
		rw := textWidth(right)
		if rw < w {
			sc.put(x+w-rw, y, right, st)
		}
	}
}

// kv 一行"标签 + 值"，标签固定宽度，值从同一列起。
func kv(sc *screen, x, y, labelW int, label, value string, valueStyle style) {
	sc.put(x, y, padWidth(label, labelW), fg(colDim))
	sc.put(x+labelW, y, value, valueStyle)
}

// gauge 量表：实心块 + 空块，宽度按列算。
//
// 不用 █/░ 之外的花样：这两个字符宽度都是 1，中文与它混排不会错位。
func gauge(cur, max int64, width int) string {
	if width <= 0 {
		return ""
	}
	filled := int(math.Round(percent(cur, max) * float64(width)))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// gaugeRow 一行量表：标签对齐到 labelW 列，再画条与当前值。
func gaugeRow(sc *screen, x, y, labelW, barW int, label string, cur, max int64, color rgb) {
	sc.put(x, y, padWidth(label, labelW), fg(colDim))
	sc.put(x+labelW, y, gauge(cur, max, barW), fg(color))
	sc.put(x+labelW+barW+1, y, fmtPair(cur, max), fg(color))
}

func fmtPair(cur, max int64) string {
	return itoa64(cur) + "/" + itoa64(max)
}

// section 在盒子里画一条横向分隔（左右各留一格），用于分栏内的分组。
func section(sc *screen, x, y, w int) {
	for i := 1; i < w-1; i++ {
		sc.set(x+i, y, '─', fg(colFaint))
	}
	sc.set(x, y, '├', fg(colFaint))
	sc.set(x+w-1, y, '┤', fg(colFaint))
}

// divider 在两个并排盒子之间留一列空隙，不画竖线：两块边框各自完整更清爽。

// titleBar 整行标题条：反色底，左中右三段。
func titleBar(sc *screen, y int, left, center, right string) {
	st := style{fg: colText, bg: colPanel, hasBG: true}
	row(sc, 0, y, sc.w, st)
	x := sc.put(1, y, truncateWidth(left, sc.w-2), st)
	if center != "" {
		cw := textWidth(center)
		if x+cw+1 < sc.w {
			sc.put((sc.w-cw)/2, y, center, style{fg: colAccent, bg: colPanel, hasBG: true})
		}
	}
	if right != "" {
		rw := textWidth(right)
		if rw+1 < sc.w {
			sc.put(sc.w-rw-1, y, right, style{fg: colDim, bg: colPanel, hasBG: true})
		}
	}
}
