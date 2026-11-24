package main

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// 字符屏：一屏就是 w×h 个格子，整屏重绘。
//
// 不引任何 TUI 库：写字、着色、光标定位全靠 ANSI 转义。
// 这么做的代价是所有细节都要自己算对——宽度、续格、残影、脏行——
// 好处是二进制零依赖、SSH 上去就能跑、行为完全可预测。

type style struct {
	fg    rgb
	bg    rgb
	hasBG bool
	bold  bool
}

func fg(c rgb) style      { return style{fg: c} }
func fgBold(c rgb) style  { return style{fg: c, bold: true} }
func fgBg(f, b rgb) style { return style{fg: f, bg: b, hasBG: true} }
func (s style) withBG(b rgb) style {
	s.bg, s.hasBG = b, true
	return s
}
func (s style) withBold() style { s.bold = true; return s }

type cell struct {
	ch rune
	st style
}

type hitKind int

const (
	hitExit hitKind = iota
	hitBagItem
	hitSkillItem
	hitShopItem
	hitFoe
	hitLoginField
	hitLoginButton
	hitRegisterToggle
	hitCreateField
	hitHelpClose
	hitEncounter
	hitTalk
)

type hitAction struct {
	kind  hitKind
	index int
}

type hitRegion struct {
	x, y, w, h int
	act        hitAction
}

type screen struct {
	w, h  int
	buf   []cell
	prev  []uint64 // 上一帧每行内容签名：没变的行不重写
	prevW int
	prevH int
	hits  []hitRegion
}

func newScreen(w, h int) *screen {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	s := &screen{w: w, h: h, buf: make([]cell, w*h)}
	base := fg(colText).withBG(colBG)
	for i := range s.buf {
		s.buf[i] = cell{ch: ' ', st: base}
	}
	return s
}

// reset 清空画面但保留上一帧签名：复用同一个屏时，没变的行依然不会被重写。
func (s *screen) reset() {
	st := fg(colText).withBG(colBG)
	for i := range s.buf {
		s.buf[i] = cell{ch: ' ', st: st}
	}
	s.hits = s.hits[:0]
}

// addHit 登记一个可点区域（字符格坐标）；后登记的优先命中。
func (s *screen) addHit(x, y, w, h int, act hitAction) {
	if w <= 0 || h <= 0 {
		return
	}
	s.hits = append(s.hits, hitRegion{x: x, y: y, w: w, h: h, act: act})
}

func (s *screen) hitAt(x, y int) (hitAction, bool) {
	for i := len(s.hits) - 1; i >= 0; i-- {
		r := s.hits[i]
		if x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h {
			return r.act, true
		}
	}
	return hitAction{}, false
}

// bgAt 取某格当前的底色，用于"只写前景色时沿用面板色"。
func (s *screen) bgAt(i int) (rgb, bool) {
	if i < 0 || i >= len(s.buf) {
		return rgb{}, false
	}
	return s.buf[i].st.bg, s.buf[i].st.hasBG
}

func (s *screen) set(x, y int, ch rune, st style) {
	if x < 0 || y < 0 || x >= s.w || y >= s.h {
		return
	}
	w := runeWidth(ch)
	if w == 0 {
		return
	}
	i := y*s.w + x
	// 只给前景色的写法（fg/put/kv…）沿用落笔处已有的底色：
	// 盒子先铺面板色，里面写字不该再把面板戳成黑窟窿。
	if !st.hasBG {
		if bg, ok := s.bgAt(i); ok {
			st = st.withBG(bg)
		}
	}
	s.buf[i] = cell{ch: ch, st: st}
	if w == 2 && x+1 < s.w {
		// 续格：终端已经把下一个位置让给了这个宽字，渲染时跳过。
		cont := st
		if bg, ok := s.bgAt(i + 1); ok {
			cont = cont.withBG(bg)
		}
		s.buf[i+1] = cell{ch: 0, st: cont}
	}
}

// put 在 (x,y) 写一段文本，返回写完后的列坐标。
func (s *screen) put(x, y int, text string, st style) int {
	for _, r := range text {
		w := runeWidth(r)
		if w == 0 {
			continue
		}
		if x+w > s.w {
			break
		}
		s.set(x, y, r, st)
		x += w
	}
	return x
}

func (s *screen) fill(x, y, w, h int, ch rune, st style) {
	for j := y; j < y+h; j++ {
		for i := x; i < x+w; i++ {
			s.set(i, j, ch, st)
		}
	}
}

func (s *screen) clearLine(y int) {
	for x := 0; x < s.w; x++ {
		s.set(x, y, ' ', fg(colText))
	}
}

// row 返回某一行的纯文本（宽字符续格跳过）。
func (s *screen) row(y int) string {
	if y < 0 || y >= s.h {
		return ""
	}
	var b strings.Builder
	for x := 0; x < s.w; x++ {
		c := s.buf[y*s.w+x]
		if c.ch == 0 {
			continue
		}
		b.WriteRune(c.ch)
	}
	return b.String()
}

func (s *screen) frame() []string {
	out := make([]string, s.h)
	for y := 0; y < s.h; y++ {
		out[y] = s.row(y)
	}
	return out
}

// lineHash 给一行算签名，用于跳过多余重绘。
func (s *screen) lineHash(y int) uint64 {
	var h uint64 = 1469598103934665603
	for x := 0; x < s.w; x++ {
		c := s.buf[y*s.w+x]
		h ^= uint64(c.ch)
		h *= 1099511628211
		h ^= uint64(c.st.fg.r)<<16 | uint64(c.st.fg.g)<<8 | uint64(c.st.fg.b)
		h *= 1099511628211
		if c.st.hasBG {
			h ^= uint64(c.st.bg.r)<<16 | uint64(c.st.bg.g)<<8 | uint64(c.st.bg.b) | 1<<32
		}
		if c.st.bold {
			h ^= 1 << 40
		}
		h *= 1099511628211
	}
	return h
}

// render 把整屏打到终端。相同的行跳过，避免闪烁与无谓的字节。
func (s *screen) render(out *bufio.Writer, m colorMode) {
	full := s.prev == nil || s.prevW != s.w || s.prevH != s.h
	hashes := make([]uint64, s.h)
	for y := 0; y < s.h; y++ {
		hashes[y] = s.lineHash(y)
		if !full && hashes[y] == s.prev[y] {
			continue
		}
		s.renderLine(out, y, m)
	}
	s.prev, s.prevW, s.prevH = hashes, s.w, s.h
	_ = out.Flush()
}

// renderLine 画一行：定位 → 逐格着色 → 复位 → 清到行尾。
func (s *screen) renderLine(out *bufio.Writer, y int, m colorMode) {
	fmt.Fprintf(out, "\x1b[%d;1H", y+1)

	// 找出最后一个"非空白"格：后面的空格交给 EL 清行，不发字节。
	end := 0
	for x := s.w - 1; x >= 0; x-- {
		c := s.buf[y*s.w+x]
		if (c.ch != ' ' && c.ch != 0) || c.st.hasBG {
			end = x + 1
			break
		}
	}

	var cur style
	curOK := false
	for x := 0; x < end; x++ {
		c := s.buf[y*s.w+x]
		if c.ch == 0 {
			continue
		}
		if !curOK || c.st != cur {
			out.WriteString(sgr(c.st, m))
			cur, curOK = c.st, true
		}
		out.WriteRune(c.ch)
	}
	if curOK {
		out.WriteString("\x1b[0m")
	}
	out.WriteString("\x1b[K")
}

type colorMode int

const (
	colorTrue colorMode = iota
	color256
	colorOff
)

func parseColorMode(s string) colorMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "true", "truecolor", "24":
		return colorTrue
	case "256":
		return color256
	case "off", "none", "no", "0":
		return colorOff
	default:
		return colorTrue
	}
}

func sgr(st style, m colorMode) string {
	if m == colorOff {
		if st.bold {
			return "\x1b[0;1m"
		}
		return "\x1b[0m"
	}
	var b strings.Builder
	b.WriteString("\x1b[0")
	if st.bold {
		b.WriteString(";1")
	}
	if m == colorTrue {
		fmt.Fprintf(&b, ";38;2;%d;%d;%d", st.fg.r, st.fg.g, st.fg.b)
		if st.hasBG {
			fmt.Fprintf(&b, ";48;2;%d;%d;%d", st.bg.r, st.bg.g, st.bg.b)
		}
	} else {
		fmt.Fprintf(&b, ";38;5;%d", to256(st.fg))
		if st.hasBG {
			fmt.Fprintf(&b, ";48;5;%d", to256(st.bg))
		}
	}
	b.WriteString("m")
	return b.String()
}

// to256 把真彩近似到 xterm-256：灰阶走 232–255，其余走 6×6×6 色立方。
func to256(c rgb) int {
	r, g, b := int(c.r), int(c.g), int(c.b)
	if abs(r-g) < 8 && abs(g-b) < 8 && abs(r-b) < 8 {
		gray := (r + g + b) / 3
		if gray < 8 {
			return 16
		}
		if gray > 238 {
			return 231
		}
		return 232 + (gray-8)*24/230
	}
	level := func(v int) int {
		switch {
		case v < 48:
			return 0
		case v < 115:
			return 1
		default:
			return (v - 35) / 40
		}
	}
	return 16 + 36*level(r) + 6*level(g) + level(b)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func itoa(n int) string { return strconv.Itoa(n) }
