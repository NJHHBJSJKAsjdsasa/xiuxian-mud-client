package main

import (
	"math"
	"strings"
)

// 终端配色：与 client/docs/05-UI.md 同一套色值，改这里就是改全局。
// 终端没有"主题切换"，深色底是唯一形态。

type rgb struct{ r, g, b uint8 }

func (c rgb) mix(o rgb, t float64) rgb {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	f := func(a, b uint8) uint8 {
		return uint8(math.Round(float64(a) + (float64(b)-float64(a))*t))
	}
	return rgb{f(c.r, o.r), f(c.g, o.g), f(c.b, o.b)}
}

var (
	// 屏幕底色与面板底色差一档：盒子内部铺面板色，看起来像卡片而不是
	// 黑色空洞；纯黑（5,7,9）在多数显示器上与窗框糊成一片。
	colBG     = rgb{16, 20, 26}
	colPanel  = rgb{30, 37, 46}
	colText   = rgb{226, 226, 214}
	colDim    = rgb{120, 130, 138}
	colFaint  = rgb{70, 78, 86}
	colAccent = rgb{127, 209, 193}
	colGold   = rgb{217, 179, 106}
	colBad    = rgb{196, 85, 61}
	colGood   = rgb{111, 191, 115}
	colWarn   = rgb{201, 162, 39}
	colPVP    = rgb{169, 139, 200}
	colNar    = rgb{201, 184, 150}
)

// tone 把日志类型映射成颜色。
func tone(k string) rgb {
	switch k {
	case "sys":
		return colDim
	case "nar":
		return colNar
	case "combat":
		return colText
	case "good":
		return colGood
	case "bad":
		return colBad
	case "lvl":
		return colGold
	case "chat":
		return colAccent
	case "pvp":
		return colPVP
	default:
		return colText
	}
}

// bearingArrow 八向箭头：入参是数学坐标（x 向东、y 向北），
// 与屏幕的"上为北"一致，所以直接算角度，不用再取反。
func bearingArrow(dx, dy int) string {
	if dx == 0 && dy == 0 {
		return "·"
	}
	arrows := [8]string{"→", "↗", "↑", "↖", "←", "↙", "↓", "↘"}
	// 屏幕坐标：右为 +x，上为 +y。
	ang := math.Atan2(float64(dy), float64(dx))
	idx := int(math.Round(ang / (math.Pi / 4)))
	idx = ((idx % 8) + 8) % 8
	return arrows[idx]
}

// bar 字符进度条：[####------]。
func bar(cur, max int64, cells int) string {
	if cells <= 0 {
		return "[]"
	}
	filled := int(math.Round(percent(cur, max) * float64(cells)))
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", cells-filled) + "]"
}
