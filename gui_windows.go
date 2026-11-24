//go:build windows

package main

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 桌面窗口：把同一块字符屏用 GDI 画到原生窗口里。
//
// 为什么不引 GUI 框架：界面本来就是字符网格，GDI 的 FillRect + ExtTextOut
// 足够；窗口、输入、DPI 都在自己手里，依赖仍只有 x/sys。
// 桌面版比终端多两件事：原生窗口边框与鼠标拖舆图。

var (
	guiUser32 = windows.NewLazySystemDLL("user32.dll")
	guiGDI32  = windows.NewLazySystemDLL("gdi32.dll")
	guiK32    = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW    = guiUser32.NewProc("RegisterClassExW")
	procCreateWindowExW     = guiUser32.NewProc("CreateWindowExW")
	procDefWindowProcW      = guiUser32.NewProc("DefWindowProcW")
	procShowWindow          = guiUser32.NewProc("ShowWindow")
	procUpdateWindow        = guiUser32.NewProc("UpdateWindow")
	procGetMessageW         = guiUser32.NewProc("GetMessageW")
	procTranslateMessage    = guiUser32.NewProc("TranslateMessage")
	procDispatchMessageW    = guiUser32.NewProc("DispatchMessageW")
	procPostQuitMessage     = guiUser32.NewProc("PostQuitMessage")
	procDestroyWindow       = guiUser32.NewProc("DestroyWindow")
	procGetClientRect       = guiUser32.NewProc("GetClientRect")
	procAdjustWindowRectEx  = guiUser32.NewProc("AdjustWindowRectEx")
	procSetTimer            = guiUser32.NewProc("SetTimer")
	procKillTimer           = guiUser32.NewProc("KillTimer")
	procInvalidateRect      = guiUser32.NewProc("InvalidateRect")
	procBeginPaint          = guiUser32.NewProc("BeginPaint")
	procEndPaint            = guiUser32.NewProc("EndPaint")
	procGetDC               = guiUser32.NewProc("GetDC")
	procReleaseDC           = guiUser32.NewProc("ReleaseDC")
	procLoadCursorW         = guiUser32.NewProc("LoadCursorW")
	procSetCapture          = guiUser32.NewProc("SetCapture")
	procReleaseCapture      = guiUser32.NewProc("ReleaseCapture")
	procGetKeyState         = guiUser32.NewProc("GetKeyState")
	procSetProcessDPIAware  = guiUser32.NewProc("SetProcessDPIAware")
	procGetSystemMetrics    = guiUser32.NewProc("GetSystemMetrics")
	procGetWindowRect       = guiUser32.NewProc("GetWindowRect")
	procSetForegroundWindow = guiUser32.NewProc("SetForegroundWindow")
	procFillRect            = guiUser32.NewProc("FillRect")

	procCreateCompatibleDC     = guiGDI32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = guiGDI32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = guiGDI32.NewProc("SelectObject")
	procDeleteObject           = guiGDI32.NewProc("DeleteObject")
	procDeleteDC               = guiGDI32.NewProc("DeleteDC")
	procCreateFontW            = guiGDI32.NewProc("CreateFontW")
	procSetTextColor           = guiGDI32.NewProc("SetTextColor")
	procSetBkMode              = guiGDI32.NewProc("SetBkMode")
	procExtTextOutW            = guiGDI32.NewProc("ExtTextOutW")

	procCreateSolidBrush      = guiGDI32.NewProc("CreateSolidBrush")
	procBitBlt                = guiGDI32.NewProc("BitBlt")
	procGetTextExtentPoint32W = guiGDI32.NewProc("GetTextExtentPoint32W")
	procGetDeviceCaps         = guiGDI32.NewProc("GetDeviceCaps")

	procGetModuleHandleW = guiK32.NewProc("GetModuleHandleW")
)

const (
	wmDestroy     = 0x0002
	wmSize        = 0x0005
	wmPaint       = 0x000F
	wmClose       = 0x0010
	wmEraseBkgnd  = 0x0014
	wmKeyDown     = 0x0100
	wmChar        = 0x0102
	wmTimer       = 0x0113
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmLButtonDbl  = 0x0203

	wsOverlappedWindow = 0x00CF0000
	swHide             = 0
	swShow             = 5
	cwUseDefault       = uintptr(0x80000000)

	srcCopy = 0x00CC0020

	transparentBk  = 1
	fixedPitch     = 0x31 // FIXED_PITCH | FF_MODERN
	defaultCharset = 1
)

type winPoint struct{ X, Y int32 }

type winMsg struct {
	HWnd    windows.HWND
	Message uint32
	_       uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      winPoint
	Private uint32
}

type winClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type paintStruct struct {
	Hdc       windows.Handle
	Erase     int32
	Paint     windows.Rect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

type winSize struct{ CX, CY int32 }

type guiApp struct {
	app *app

	hwnd     windows.HWND
	hdcMem   windows.Handle
	hbmp     windows.Handle
	stockBmp windows.Handle
	bmpW     int
	bmpH     int

	cols  int
	rows  int
	cellW int
	cellH int
	minW  int32
	minH  int32

	font     windows.Handle
	fontBold windows.Handle

	lastHash uint64
	lastW    int
	lastH    int

	dragging  bool
	dragX     int
	dragY     int
	dragPanX  int
	dragPanY  int
	resizing  bool
	skipSpace bool
	highSurf  uint16
}

var (
	gui      *guiApp
	guiClass = mustUTF16("JiuzhouWenDao")
	guiProc  = windows.NewCallback(guiWndProc)
)

func mustUTF16(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		panic(err)
	}
	return p
}

// consoleOwned 报告控制台是不是本进程带起来的（双击 exe）。
func consoleOwned() bool { return ownsConsole() }

func runGUI(a *app, _ config) error {
	// Windows 的窗口、消息队列、GDI 句柄都绑在创建它的 OS 线程上。
	// Go 的 goroutine 会在系统调用后换线程，不锁线程就会出现
	// "窗口能创建、但消息循环在另一个线程上永远等不到消息"。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	gui = &guiApp{app: a}
	if err := gui.initFonts(); err != nil {
		return err
	}

	hInst, _, _ := procGetModuleHandleW.Call(0)
	wc := winClassEx{
		Size:      uint32(unsafe.Sizeof(winClassEx{})),
		Style:     0x000B, // CS_VREDRAW | CS_HREDRAW | CS_DBLCLKS
		WndProc:   guiProc,
		Instance:  windows.Handle(hInst),
		Cursor:    loadCursor(),
		ClassName: guiClass,
	}
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("注册窗口类失败：%v", err)
	}

	var rc windows.Rect
	rc.Right = int32(110 * gui.cellW)
	rc.Bottom = int32(34 * gui.cellH)
	procAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&rc)),
		uintptr(wsOverlappedWindow), 0, 0)

	hwnd, _, err := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(guiClass)),
		uintptr(unsafe.Pointer(mustUTF16("九州问道"))),
		uintptr(wsOverlappedWindow),
		cwUseDefault, cwUseDefault,
		uintptr(rc.Right-rc.Left), uintptr(rc.Bottom-rc.Top),
		0, 0, hInst, 0)
	if hwnd == 0 {
		return fmt.Errorf("创建窗口失败：%v", err)
	}
	gui.hwnd = windows.HWND(hwnd)
	gui.centerWindow()

	// 定时器先建：失败时窗口直接销毁并回退 TUI，此时控制台还没藏。
	if r, _, _ := procSetTimer.Call(hwnd, 1, 250, 0); r == 0 {
		procDestroyWindow.Call(hwnd)
		return errors.New("创建窗口定时器失败")
	}

	// 双击 exe 时 Windows 会先给一个黑控制台；GUI 起来了就把它藏掉。
	// 从终端带 --gui 启动时控制台是用户的，不能动。
	if ownsConsole() {
		if ch, _, _ := procGetConsoleWindow.Call(); ch != 0 {
			procShowWindow.Call(ch, swHide)
		}
	}

	// 只有窗口真的起来了才标记桌面模式：初始化失败会回退 TUI，
	// 那时舆图提示不能写着"拖动平移"。
	a.ui.desktop = true
	procShowWindow.Call(hwnd, swShow)
	procSetForegroundWindow.Call(hwnd)
	procUpdateWindow.Call(hwnd)

	var m winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	a.shutdown()
	return nil
}

func loadCursor() windows.Handle {
	r, _, _ := procLoadCursorW.Call(0, 32512) // IDC_ARROW
	return windows.Handle(r)
}

func (g *guiApp) enforceMinSize() {
	if g.resizing || g.minW <= 0 || g.hwnd == 0 {
		return
	}
	var rc windows.Rect
	procGetClientRect.Call(uintptr(g.hwnd), uintptr(unsafe.Pointer(&rc)))
	if int(rc.Right) >= 80*g.cellW && int(rc.Bottom) >= 24*g.cellH {
		return
	}
	g.resizing = true
	procSetWindowPos.Call(uintptr(g.hwnd), 0, 0, 0,
		uintptr(g.minW), uintptr(g.minH),
		uintptr(swpNoMove|swpNoZOrder|swpNoActivate))
	g.resizing = false
}

func (g *guiApp) initFonts() error {
	procSetProcessDPIAware.Call()
	hdc, _, _ := procGetDC.Call(0)
	if hdc == 0 {
		return errors.New("取屏幕 DC 失败")
	}
	defer procReleaseDC.Call(0, hdc)

	dpi, _, _ := procGetDeviceCaps.Call(hdc, 90) // LOGPIXELSY
	scale := 1.0
	if dpi > 0 {
		scale = float64(dpi) / 96.0
	}
	fontH := int(math.Round(15 * scale))
	if fontH < 12 {
		fontH = 12
	}
	g.font = g.makeFont(fontH, false)
	g.fontBold = g.makeFont(fontH, true)
	if g.font == 0 {
		return errors.New("创建字体失败")
	}

	procSelectObject.Call(hdc, uintptr(g.font))
	var sa, sc winSize
	procGetTextExtentPoint32W.Call(hdc, uintptr(unsafe.Pointer(mustUTF16("A"))), 1,
		uintptr(unsafe.Pointer(&sa)))
	procGetTextExtentPoint32W.Call(hdc, uintptr(unsafe.Pointer(mustUTF16("国"))), 1,
		uintptr(unsafe.Pointer(&sc)))

	g.cellW = int(sa.CX)
	// 中文字宽取"国"的一半：只按 ASCII 估 2×cellW 会让中文挤在一起。
	if half := (int(sc.CX) + 1) / 2; half > g.cellW {
		g.cellW = half
	}
	if g.cellW < 4 {
		g.cellW = int(math.Round(8 * scale))
	}
	g.cellH = int(sc.CY) + 2
	if g.cellH < fontH+2 {
		g.cellH = fontH + 2
	}
	if g.cellW < 4 || g.cellH < 10 {
		return errors.New("字体度量异常")
	}
	// 窗口最少 80×24 格（C-05 的最小终端），否则右侧与见闻会被裁掉。
	var minRc windows.Rect
	minRc.Right = int32(80 * g.cellW)
	minRc.Bottom = int32(24 * g.cellH)
	procAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&minRc)),
		uintptr(wsOverlappedWindow), 0, 0)
	g.minW = minRc.Right - minRc.Left
	g.minH = minRc.Bottom - minRc.Top
	return nil
}

func (g *guiApp) makeFont(height int, bold bool) windows.Handle {
	weight := 400
	if bold {
		weight = 700
	}
	r, _, _ := procCreateFontW.Call(
		uintptr(int32(-height)), 0, 0, 0, uintptr(weight),
		0, 0, 0,
		defaultCharset, 0, 0,
		0,          // quality：默认
		fixedPitch, // pitch and family：等宽 + 现代
		uintptr(unsafe.Pointer(mustUTF16("SimSun"))))
	return windows.Handle(r)
}

func (g *guiApp) centerWindow() {
	sw, _, _ := procGetSystemMetrics.Call(0)
	sh, _, _ := procGetSystemMetrics.Call(1)
	var rc windows.Rect
	// GetWindowRect 不在 vt_windows 里，单独取：窗口已创建，直接量。
	procGetWindowRect.Call(uintptr(g.hwnd), uintptr(unsafe.Pointer(&rc)))
	w := int(rc.Right - rc.Left)
	h := int(rc.Bottom - rc.Top)
	x := (int(sw) - w) / 2
	y := (int(sh) - h) / 2
	procSetWindowPos.Call(uintptr(g.hwnd), 0, uintptr(x), uintptr(y), 0, 0,
		uintptr(swpNoSize|swpNoZOrder|swpNoActivate))
}

func guiWndProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	if gui == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
		return r
	}
	switch msg {
	case wmPaint:
		gui.onPaint()
		return 0
	case wmEraseBkgnd:
		return 1
	case wmSize:
		if wparam != 1 { // SIZE_MINIMIZED：最小化时不要回弹
			gui.enforceMinSize()
		}
		procInvalidateRect.Call(hwnd, 0, 0)
		return 0
	case wmTimer:
		gui.onTimer()
		return 0
	case wmKeyDown:
		gui.onKeyDown(uint32(wparam))
		return 0
	case wmChar:
		gui.onChar(uint32(wparam))
		return 0
	case wmMouseMove:
		gui.onMouseMove(lparam)
		return 0
	case wmLButtonDown:
		gui.onLButtonDown(lparam)
		return 0
	case wmLButtonUp:
		gui.onLButtonUp()
		return 0
	case wmLButtonDbl:
		gui.onDblClick()
		return 0
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procKillTimer.Call(hwnd, 1)
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
	return r
}

func (g *guiApp) onTimer() {
	g.app.drain()
	g.app.tick()
	if g.app.quit {
		procDestroyWindow.Call(uintptr(g.hwnd))
		return
	}
	procInvalidateRect.Call(uintptr(g.hwnd), 0, 0)
}

func (g *guiApp) onPaint() {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(uintptr(g.hwnd), uintptr(unsafe.Pointer(&ps)))
	defer procEndPaint.Call(uintptr(g.hwnd), uintptr(unsafe.Pointer(&ps)))
	g.paint(windows.Handle(hdc))
}

func (g *guiApp) paint(hdc windows.Handle) {
	var rc windows.Rect
	procGetClientRect.Call(uintptr(g.hwnd), uintptr(unsafe.Pointer(&rc)))
	w, h := int(rc.Right), int(rc.Bottom)
	if w <= 0 || h <= 0 {
		return
	}
	g.ensureBackBuffer(w, h)

	cols := w / g.cellW
	if cols < 40 {
		cols = 40
	}
	rows := h / g.cellH
	if rows < 12 {
		rows = 12
	}
	sc := g.app.render(cols, rows)
	hash := screenHash(sc)
	if hash != g.lastHash || cols != g.cols || rows != g.rows || w != g.lastW || h != g.lastH {
		g.drawScreen(g.hdcMem, sc)
		g.lastHash, g.cols, g.rows, g.lastW, g.lastH = hash, cols, rows, w, h
	}
	procBitBlt.Call(uintptr(hdc), 0, 0, uintptr(w), uintptr(h),
		uintptr(g.hdcMem), 0, 0, srcCopy)
}

func (g *guiApp) ensureBackBuffer(w, h int) {
	if g.hdcMem != 0 && g.bmpW == w && g.bmpH == h {
		return
	}
	if g.hdcMem != 0 {
		procSelectObject.Call(uintptr(g.hdcMem), uintptr(g.stockBmp))
		procDeleteObject.Call(uintptr(g.hbmp))
		procDeleteDC.Call(uintptr(g.hdcMem))
	}
	screenDC, _, _ := procGetDC.Call(0)
	g.hdcMem = windows.Handle(callHandle(procCreateCompatibleDC, screenDC))
	g.hbmp = windows.Handle(callHandle(procCreateCompatibleBitmap, screenDC, uintptr(w), uintptr(h)))
	g.stockBmp = windows.Handle(callHandle(procSelectObject, uintptr(g.hdcMem), uintptr(g.hbmp)))
	procReleaseDC.Call(0, screenDC)
	g.bmpW, g.bmpH = w, h
	g.lastHash = 0
}

func callHandle(proc *windows.LazyProc, args ...uintptr) uintptr {
	r, _, _ := proc.Call(args...)
	return r
}

func (g *guiApp) drawScreen(hdc windows.Handle, sc *screen) {
	brush := windows.Handle(callHandle(procCreateSolidBrush, uintptr(colorRef(colBG))))
	fill(hdc, 0, 0, g.bmpW, g.bmpH, brush)
	procDeleteObject.Call(uintptr(brush))

	procSetBkMode.Call(uintptr(hdc), transparentBk)

	for y := 0; y < sc.h; y++ {
		x := 0
		for x < sc.w {
			c := sc.buf[y*sc.w+x]
			if c.ch == 0 {
				x++
				continue
			}
			st := c.st
			x0 := x
			var runes []rune
			var dx []int32
			for x < sc.w {
				cc := sc.buf[y*sc.w+x]
				if cc.ch == 0 {
					x++
					continue
				}
				if cc.st != st {
					break
				}
				runes = append(runes, gdiRune(cc.ch))
				adv := int32(g.cellW)
				if runeWidth(cc.ch) == 2 {
					adv = int32(g.cellW * 2)
				}
				dx = append(dx, adv)
				x++
			}
			if len(runes) == 0 {
				continue
			}
			if st.hasBG {
				b := windows.Handle(callHandle(procCreateSolidBrush, uintptr(colorRef(st.bg))))
				fill(hdc, x0*g.cellW, y*g.cellH, (x-x0)*g.cellW, g.cellH, b)
				procDeleteObject.Call(uintptr(b))
			}
			font := g.font
			if st.bold {
				font = g.fontBold
			}
			procSelectObject.Call(uintptr(hdc), uintptr(font))
			procSetTextColor.Call(uintptr(hdc), uintptr(colorRef(st.fg)))
			u := utf16.Encode(runes)
			procExtTextOutW.Call(uintptr(hdc),
				uintptr(x0*g.cellW), uintptr(y*g.cellH), 0, 0,
				uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)),
				uintptr(unsafe.Pointer(&dx[0])))
		}
	}
}

func fill(hdc windows.Handle, x, y, w, h int, brush windows.Handle) {
	r := windows.Rect{Left: int32(x), Top: int32(y), Right: int32(x + w), Bottom: int32(y + h)}
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&r)), uintptr(brush))
}

func colorRef(c rgb) uint32 {
	return uint32(c.r) | uint32(c.g)<<8 | uint32(c.b)<<16
}

// gdiRune 把 SimSun 没有字形的 UI 符号换成等宽替代：
// ▸（U+25B8）与 ░（U+2591）不在 GB2312 里，GDI 会画成方框。
func gdiRune(r rune) rune {
	switch r {
	case '▸':
		return '»'
	case '░':
		return '·'
	}
	return r
}

func screenHash(sc *screen) uint64 {
	var h uint64 = 1469598103934665603
	mix := func(v uint64) {
		h ^= v
		h *= 1099511628211
	}
	for _, c := range sc.buf {
		mix(uint64(c.ch))
		mix(uint64(c.st.fg.r)<<16 | uint64(c.st.fg.g)<<8 | uint64(c.st.fg.b))
		if c.st.hasBG {
			mix(1<<32 | uint64(c.st.bg.r)<<16 | uint64(c.st.bg.g)<<8 | uint64(c.st.bg.b))
		}
		if c.st.bold {
			mix(1 << 40)
		}
	}
	return h
}

func (g *guiApp) onKeyDown(vk uint32) {
	ctrl, _, _ := procGetKeyState.Call(0x11) // VK_CONTROL
	if vk == 0x43 && int16(ctrl) < 0 {       // Ctrl+C
		g.app.quit = true
		return
	}
	if vk == 0x20 {
		// 输入框里的空格要当字符（WM_CHAR 会送），主界面/战斗里的空格
		// 要当特殊键（历练/普攻）；后者把随后的 WM_CHAR 丢掉。
		if g.app.ui.typing {
			return
		}
		g.skipSpace = true
	}
	if k, ok := vkToKey(vk); ok {
		g.app.onKey(k)
		procInvalidateRect.Call(uintptr(g.hwnd), 0, 0)
	}
}

func (g *guiApp) onChar(ch uint32) {
	if ch == 0x20 && g.skipSpace {
		g.skipSpace = false
		return
	}
	if ch == 0x20 {
		// 输入框里的空格仍要能打出来。
		g.app.onKey(key{kind: keyRune, r: ' '})
		procInvalidateRect.Call(uintptr(g.hwnd), 0, 0)
		return
	}
	if ch < 0x20 {
		return
	}
	if ch >= 0xD800 && ch <= 0xDBFF {
		g.highSurf = uint16(ch)
		return
	}
	var r rune
	if ch >= 0xDC00 && ch <= 0xDFFF {
		if g.highSurf == 0 {
			return
		}
		r = utf16.DecodeRune(rune(g.highSurf), rune(ch))
		g.highSurf = 0
	} else {
		r = rune(ch)
	}
	g.app.onKey(key{kind: keyRune, r: r})
	procInvalidateRect.Call(uintptr(g.hwnd), 0, 0)
}

func vkToKey(vk uint32) (key, bool) {
	switch vk {
	case 0x25:
		return special("left"), true
	case 0x26:
		return special("up"), true
	case 0x27:
		return special("right"), true
	case 0x28:
		return special("down"), true
	case 0x0D:
		return special("enter"), true
	case 0x1B:
		return special("esc"), true
	case 0x09:
		return special("tab"), true
	case 0x08:
		return special("backspace"), true
	case 0x20:
		return special("space"), true
	case 0x21:
		return special("pgup"), true
	case 0x22:
		return special("pgdn"), true
	case 0x24:
		return special("home"), true
	case 0x23:
		return special("end"), true
	}
	if vk >= 0x70 && vk <= 0x7B {
		return special(fmt.Sprintf("f%d", int(vk-0x70)+1)), true
	}
	return key{}, false
}

func (g *guiApp) mapActive() bool {
	ui := &g.app.ui
	return ui.view == viewMain && ui.page == pageMap &&
		!ui.help && !ui.talkPick && !ui.typing && g.app.st.encounter == nil
}

func mouseXY(lparam uintptr) (int, int) {
	return int(int16(lparam & 0xFFFF)), int(int16((lparam >> 16) & 0xFFFF))
}

func (g *guiApp) onLButtonDown(lparam uintptr) {
	if !g.mapActive() {
		return
	}
	x, y := mouseXY(lparam)
	g.dragging = true
	g.dragX, g.dragY = x, y
	g.dragPanX, g.dragPanY = g.app.ui.mapPanX, g.app.ui.mapPanY
	procSetCapture.Call(uintptr(g.hwnd))
}

func (g *guiApp) onMouseMove(lparam uintptr) {
	if !g.dragging {
		return
	}
	x, y := mouseXY(lparam)
	g.app.ui.mapPanX = g.dragPanX + (x-g.dragX)/g.cellW
	g.app.ui.mapPanY = g.dragPanY + (y-g.dragY)/g.cellH
	procInvalidateRect.Call(uintptr(g.hwnd), 0, 0)
}

func (g *guiApp) onLButtonUp() {
	if !g.dragging {
		return
	}
	g.dragging = false
	procReleaseCapture.Call()
}

func (g *guiApp) onDblClick() {
	if !g.mapActive() {
		return
	}
	g.app.ui.mapPanX, g.app.ui.mapPanY = 0, 0
	procInvalidateRect.Call(uintptr(g.hwnd), 0, 0)
}
