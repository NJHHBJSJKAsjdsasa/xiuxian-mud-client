package main

import "math"

// ASCII 场景：山脊、地形、地面、人影、天气。
//
// 纯函数式生成，不加载资源、不逐帧抖动——随机位置用 sin 哈希，
// 同一尺寸下每次画出来都一样。只用 ASCII，保证任何等宽字体都对齐。

type pose int

const (
	poseStanding pose = iota
	poseSitting
	poseWalking
)

type sceneSpec struct {
	terrain string
	weather string
	daypart string
	pose    pose
}

// drawScene 把场景画进 scr 的 (x0,y0,w,h) 区域。
func drawScene(scr *screen, x0, y0, w, h int, s sceneSpec) {
	if w < 8 {
		w = 8
	}
	if h < 4 {
		h = 4
	}
	if x0+w > scr.w {
		w = scr.w - x0
	}
	if y0+h > scr.h {
		h = scr.h - y0
	}
	if w < 8 || h < 4 {
		return
	}

	// 天空：顶部最黑，靠近地平线略亮；黎明偏暖、夜偏蓝。
	skyBot := rgb{10, 14, 18}
	switch s.daypart {
	case "dawn":
		skyBot = rgb{28, 22, 18}
	case "dusk":
		skyBot = rgb{30, 20, 24}
	case "night":
		skyBot = rgb{6, 8, 12}
	}
	for r := 0; r < h; r++ {
		t := float64(r) / float64(h)
		st := style{fg: colFaint, bg: colBG.mix(skyBot, t), hasBG: true}
		for c := 0; c < w; c++ {
			scr.set(x0+c, y0+r, ' ', st)
		}
	}

	// 远、中、近三层山脊：峰谷用 |sin| 叠出来，脊线只画上升/下降/持平三种字符，
	// 脊下补两行 ':' 做厚度，再往下用背景色铺实，近山才能遮住远山。
	ridges := []struct {
		baseR float64
		ampR  float64
		freq  float64
		seed  float64
		col   rgb
	}{
		{0.40, 0.22, 2.1, 0.7, colFaint},
		{0.54, 0.17, 3.4, 2.1, colFaint.mix(colDim, 0.4)},
		{0.68, 0.12, 5.2, 4.3, colDim.mix(colText, 0.25)},
	}
	for _, rg := range ridges {
		base := int(float64(h) * rg.baseR)
		prevTop := 0
		first := true
		for c := 0; c < w; c++ {
			t := float64(c) / float64(w)
			wave := math.Abs(math.Sin(t*rg.freq*math.Pi + rg.seed))
			hh := wave * rg.ampR * float64(h)
			top := base - int(hh)
			if top >= 0 && top < h {
				// 字符按"与上一列的真实高度差"选：升 /、降 \、平 _。
				// 这样山脊和地形用同一套格子，不会出现看不见的抖动。
				ch := '_'
				if !first {
					switch {
					case top < prevTop:
						ch = '/'
					case top > prevTop:
						ch = '\\'
					}
				}
				first = false
				scr.set(x0+c, y0+top, ch, fg(rg.col))
				// 山体厚度：脊下两行 ':'，再往下铺背景，形成实心山。
				for r := top + 1; r < h; r++ {
					if r >= base {
						break
					}
					if r <= top+2 {
						scr.set(x0+c, y0+r, ':', fg(rg.col))
					} else {
						scr.set(x0+c, y0+r, ' ', style{fg: colFaint})
					}
				}
			}
			prevTop = top
		}
	}

	// 地势：水、林、镇各有各的存在感。
	ground := int(float64(h) * 0.82)
	if ground >= h {
		ground = h - 1
	}
	switch s.terrain {
	case "river", "lake":
		water := colBG.mix(colAccent, 0.35)
		for r := ground; r < h; r++ {
			for c := 0; c < w; c++ {
				if (r+c)%3 == 0 {
					scr.set(x0+c, y0+r, '~', fg(water))
				}
			}
		}
	case "woods", "forest", "bamboo":
		tree := colDim.mix(colGood, 0.35)
		for i, c := 2, 0; c < w-2; c, i = c+5, i+1 {
			top := ground - (2 + i%3)
			if top < 0 || top >= h {
				continue
			}
			scr.set(x0+c, y0+top, '^', fg(tree))
			if top+1 < h {
				scr.set(x0+c, y0+top+1, '|', fg(colFaint))
			}
		}
	case "town", "gate":
		for i, c := 3, 0; c < w-3; c, i = c+7, i+1 {
			top := ground - (3 + i%2)
			for d, ch := range []rune{'_', '|', '|'} {
				if top+d >= 0 && top+d < h {
					scr.set(x0+c, y0+top+d, ch, fg(colGold))
				}
			}
		}
	}

	// 地面与脚下小路。
	soil := colBG.mix(colGold, 0.22)
	path := colBG.mix(colText, 0.42)
	for c := 0; c < w; c++ {
		scr.set(x0+c, y0+ground, '=', fg(soil))
		for r := ground + 1; r < h; r++ {
			scr.set(x0+c, y0+r, '.', fg(soil))
		}
	}
	cx := w / 2
	for r := ground; r < h; r++ {
		spread := 1 + (r-ground)/2
		lo, hi := cx-spread, cx+spread
		if lo < 0 {
			lo = 0
		}
		if hi > w-1 {
			hi = w - 1
		}
		for c := lo; c <= hi; c++ {
			cur := scr.buf[(y0+r)*scr.w+(x0+c)]
			if cur.ch == '.' || cur.ch == ' ' {
				scr.set(x0+c, y0+r, '.', fg(path))
			}
		}
	}

	// 人影三行：站 / 坐 / 走。
	body := [3]string{" o ", "/|\\", "/ \\"}
	switch s.pose {
	case poseSitting:
		body = [3]string{" o ", "/|\\", "==="}
	case poseWalking:
		body = [3]string{" o ", "/|\\", " |\\"}
	}
	gy := ground - 3
	if gy < 0 {
		gy = 0
	}
	for i, line := range body {
		r := gy + i
		if r >= h {
			continue
		}
		for j, ch := range line {
			if ch == ' ' {
				continue
			}
			c := cx - 1 + j
			if c >= 0 && c < w {
				scr.set(x0+c, y0+r, ch, fg(colText))
			}
		}
	}

	// 天气：只落在上半部分，不盖地面。
	wch := rune(0)
	wcol := colBG
	switch s.weather {
	case "rain", "storm":
		wch, wcol = '|', colBG.mix(colAccent, 0.45)
	case "snow", "frost", "hail":
		wch, wcol = '*', colBG.mix(colText, 0.55)
	case "sandstorm":
		wch, wcol = '~', colBG.mix(colGold, 0.4)
	case "fog":
		wch, wcol = '~', colBG.mix(colDim, 0.35)
	}
	if wch != 0 {
		count := w / 2
		if s.weather == "fog" {
			count = w / 5
		}
		topRows := int(float64(h) * 0.78)
		if topRows < 1 {
			topRows = 1
		}
		for i := 0; i < count; i++ {
			c := int(rnd(uint32(i), 11)*float64(w)) % w
			r := int(rnd(uint32(i), 23)*float64(topRows)) % topRows
			cur := scr.buf[(y0+r)*scr.w+(x0+c)]
			if cur.ch == ' ' || cur.ch == ':' {
				scr.set(x0+c, y0+r, wch, fg(wcol))
			}
		}
	}
}

// rnd 是确定性的伪随机：同一个 i 永远落在同一个位置，画面不会抖。
func rnd(i, salt uint32) float64 {
	x := math.Sin(float64(i+1)*12.9898+float64(salt)*78.233) * 43758.545
	return math.Abs(x - math.Floor(x))
}
