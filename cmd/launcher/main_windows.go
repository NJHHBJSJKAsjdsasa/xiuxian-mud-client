//go:build windows

// 九州问道 · 登录启动器。
//
// 独立于游戏主窗口：原生 Win32 控件负责登录/注册，成功后按
// data/manifest.json 的 game 路径拉起宿主，并用 XIUXIAN_TOKEN 交接令牌。
// 密码不进入命令行，也不写盘。
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	u32 = windows.NewLazySystemDLL("user32.dll")
	g32 = windows.NewLazySystemDLL("gdi32.dll")
	k32 = windows.NewLazySystemDLL("kernel32.dll")

	pRegisterClassExW       = u32.NewProc("RegisterClassExW")
	pCreateWindowExW        = u32.NewProc("CreateWindowExW")
	pDefWindowProcW         = u32.NewProc("DefWindowProcW")
	pShowWindow             = u32.NewProc("ShowWindow")
	pUpdateWindow           = u32.NewProc("UpdateWindow")
	pGetMessageW            = u32.NewProc("GetMessageW")
	pTranslateMessage       = u32.NewProc("TranslateMessage")
	pDispatchMessageW       = u32.NewProc("DispatchMessageW")
	pIsDialogMessageW       = u32.NewProc("IsDialogMessageW")
	pPostQuitMessage        = u32.NewProc("PostQuitMessage")
	pPostMessageW           = u32.NewProc("PostMessageW")
	pDestroyWindow          = u32.NewProc("DestroyWindow")
	pSendMessageW           = u32.NewProc("SendMessageW")
	pSetWindowTextW         = u32.NewProc("SetWindowTextW")
	pGetWindowTextW         = u32.NewProc("GetWindowTextW")
	pGetWindowTextLenW      = u32.NewProc("GetWindowTextLengthW")
	pEnableWindow           = u32.NewProc("EnableWindow")
	pSetFocus               = u32.NewProc("SetFocus")
	pGetDC                  = u32.NewProc("GetDC")
	pReleaseDC              = u32.NewProc("ReleaseDC")
	pGetClientRect          = u32.NewProc("GetClientRect")
	pGetWindowRect          = u32.NewProc("GetWindowRect")
	pSetWindowPos           = u32.NewProc("SetWindowPos")
	pGetSystemMetrics       = u32.NewProc("GetSystemMetrics")
	pAdjustWindowRect       = u32.NewProc("AdjustWindowRectEx")
	pLoadCursorW            = u32.NewProc("LoadCursorW")
	pSetProcessDPIAware     = u32.NewProc("SetProcessDPIAware")
	pFillRect               = u32.NewProc("FillRect")
	pSetBkMode              = g32.NewProc("SetBkMode")
	pSetTextColor           = g32.NewProc("SetTextColor")
	pSetBkColor             = g32.NewProc("SetBkColor")
	pCreateSolidBrush       = g32.NewProc("CreateSolidBrush")
	pDeleteObject           = g32.NewProc("DeleteObject")
	pCreateFontW            = g32.NewProc("CreateFontW")
	pGetDeviceCaps          = g32.NewProc("GetDeviceCaps")
	pGetModuleHandleW       = k32.NewProc("GetModuleHandleW")
	pEnumWindows            = u32.NewProc("EnumWindows")
	pGetWindowThreadProcID  = u32.NewProc("GetWindowThreadProcessId")
	pIsWindowVisible        = u32.NewProc("IsWindowVisible")
	pGetClassNameW          = u32.NewProc("GetClassNameW")
	pMessageBoxW            = u32.NewProc("MessageBoxW")
	pLoadImageW             = u32.NewProc("LoadImageW")
	pCreateCompatibleDC     = g32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = g32.NewProc("CreateCompatibleBitmap")
	pSelectObject           = g32.NewProc("SelectObject")
	pStretchBlt             = g32.NewProc("StretchBlt")
	pDeleteDC               = g32.NewProc("DeleteDC")
	pGetStockObject         = g32.NewProc("GetStockObject")
)

const (
	wmDestroy        = 0x0002
	wmClose          = 0x0010
	wmEraseBkgnd     = 0x0014
	wmCommand        = 0x0111
	wmCtlColorEdit   = 0x0133
	wmCtlColorStatic = 0x0138
	wmSetFont        = 0x0030
	wmAppDone        = 0x8001
	wmGameDone       = 0x8002

	wsChild           = 0x40000000
	wsVisible         = 0x10000000
	wsBorder          = 0x00800000
	wsTabStop         = 0x00010000
	wsCaption         = 0x00C00000
	wsSysMenu         = 0x00080000
	wsMinimize        = 0x00020000
	wsGroup           = 0x00020000
	wsExControlParent = 0x00010000

	esAutoHScroll = 0x0080
	esPassword    = 0x0020
	bsPush        = 0x00000000
	bsDefPush     = 0x00000001
	ssLeft        = 0x00000000
	ssBitmap      = 0x0000000E
	ssCenterImage = 0x00000200

	imageBitmap   = 0
	imageIcon     = 1
	lrLoadFile    = 0x00000010
	lrCreatedib   = 0x00002000
	lrDefaultSize = 0x00000040
	srcCopy       = 0x00CC0020
	stmSetImage   = 0x0172
	wmSetIcon     = 0x0080

	swShow       = 5
	cwUseDefault = uintptr(0x80000000)
	idLogin      = 1
	idRegister   = 1001
	idServer     = 1002
	idName       = 1003
	idPass       = 1004
	idStatus     = 1005
	idTitle      = 1006
	idSub        = 1007
	idLabServer  = 1008
	idLabName    = 1009
	idLabPass    = 1010

	transparent    = 1
	defaultCharset = 1
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }
type wMsg struct {
	HWnd    windows.HWND
	Message uint32
	_       uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	Private uint32
}
type wClassEx struct {
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

type config struct {
	Server     string `json:"server"`
	LastName   string `json:"lastName"`
	Color      string `json:"color"`
	WindowMode string `json:"windowMode,omitempty"`
}

type manifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Game    string `json:"game"`
	Server  string `json:"server"`
}

const defaultServer = "ws://127.0.0.1:8080"

func loadManifest() manifest {
	m := manifest{Name: "九州问道", Version: "0.1.0", Game: "game/playcli.exe", Server: defaultServer}
	for _, p := range candidatePaths(filepath.Join("data", "manifest.json")) {
		text, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var got manifest
		if json.Unmarshal(text, &got) != nil {
			continue
		}
		if got.Name == "" {
			got.Name = m.Name
		}
		if got.Version == "" {
			got.Version = m.Version
		}
		if got.Game == "" {
			got.Game = m.Game
		}
		return got
	}
	return m
}

// candidatePaths 先找 exe 同目录，再找 bin/ 的上一级（开发期 bin/launcher.exe）。
func candidatePaths(rel string) []string {
	exe, err := os.Executable()
	if err != nil {
		return []string{rel}
	}
	dir := filepath.Dir(exe)
	return []string{
		filepath.Join(dir, rel),
		filepath.Join(dir, "..", rel),
		rel,
	}
}

func configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "xiuxian-client", "playcli.json")
}

func loadConfig() config {
	c := config{Server: defaultServer, Color: "true"}
	text, err := os.ReadFile(configPath())
	if err != nil || json.Unmarshal(text, &c) != nil || c.Server == "" {
		c.Server = defaultServer
		c.Color = "true"
	}
	return c
}

func saveConfig(c config) {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if text, err := json.MarshalIndent(c, "", "  "); err == nil {
		_ = os.WriteFile(path, text, 0o644)
	}
}

type authOutcome struct {
	token   string
	hasChar bool
	err     error
}

type launcher struct {
	hwnd       windows.HWND
	inst       windows.Handle
	font       windows.Handle
	fontBold   windows.Handle
	fontSmall  windows.Handle
	bgBrush    windows.Handle
	panelBrush windows.Handle
	scale      float64

	server, name, pass    windows.HWND
	loginBtn, registerBtn windows.HWND
	status, title, sub    windows.HWND
	labels                [3]windows.HWND

	cfg       config
	manifest  manifest
	version   windows.HWND
	banner    windows.HWND
	bannerBmp windows.Handle
	icon      windows.Handle
	busy      bool
	statusBad bool
	done      chan authOutcome
	gameDone  chan error
}

var appUI *launcher
var launcherClass = mustUTF16("JiuzhouLauncher")
var launcherProc = windows.NewCallback(launcherWndProc)

func mustUTF16(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		panic(err)
	}
	return p
}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pSetProcessDPIAware.Call()

	cfg := loadConfig()
	mf := loadManifest()
	if cfg.Server == defaultServer && mf.Server != "" {
		cfg.Server = mf.Server
	}
	l := &launcher{
		cfg:      cfg,
		manifest: mf,
		done:     make(chan authOutcome, 1),
		gameDone: make(chan error, 1),
	}
	l.scale = dpiScale()
	appUI = l

	hInst, _, _ := pGetModuleHandleW.Call(0)
	l.inst = windows.Handle(hInst)
	l.bgBrush = brush(16, 20, 26)
	l.panelBrush = brush(30, 37, 46)
	l.font = makeFont(16, 400)
	l.fontBold = makeFont(22, 700)
	l.fontSmall = makeFont(13, 400)

	wc := wClassEx{
		Size: uint32(unsafe.Sizeof(wClassEx{})), Style: 0x000B,
		WndProc: launcherProc, Instance: l.inst,
		Cursor: loadCursor(), Background: l.bgBrush, ClassName: launcherClass,
	}
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		messageBox("注册窗口类失败：" + err.Error())
		return
	}

	rc := rect{Right: int32(l.px(520)), Bottom: int32(l.px(432))}
	pAdjustWindowRect.Call(uintptr(unsafe.Pointer(&rc)), wsCaption|wsSysMenu|wsMinimize, 0, 0)
	hwnd, _, err := pCreateWindowExW.Call(
		wsExControlParent, uintptr(unsafe.Pointer(launcherClass)),
		uintptr(unsafe.Pointer(mustUTF16(mf.Name+" · 登录"))),
		wsCaption|wsSysMenu|wsMinimize,
		cwUseDefault, cwUseDefault,
		uintptr(rc.Right-rc.Left), uintptr(rc.Bottom-rc.Top),
		0, 0, hInst, 0)
	if hwnd == 0 {
		messageBox("创建窗口失败：" + err.Error())
		return
	}
	l.hwnd = windows.HWND(hwnd)
	l.center()
	l.loadIcon()
	l.buildControls()
	pShowWindow.Call(hwnd, swShow)
	pUpdateWindow.Call(hwnd)

	var msg wMsg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if r, _, _ := pIsDialogMessageW.Call(hwnd, uintptr(unsafe.Pointer(&msg))); r != 0 {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func dpiScale() float64 {
	hdc, _, _ := pGetDC.Call(0)
	if hdc == 0 {
		return 1
	}
	defer pReleaseDC.Call(0, hdc)
	dpi, _, _ := pGetDeviceCaps.Call(hdc, 90)
	if dpi < 96 {
		dpi = 96
	}
	return float64(dpi) / 96.0
}

func (l *launcher) px(v int) int { return int(float64(v) * l.scale) }

func (l *launcher) buildControls() {
	add := func(class, text string, style, ex uintptr, x, y, w, h int, id int) windows.HWND {
		r, _, _ := pCreateWindowExW.Call(
			ex, uintptr(unsafe.Pointer(mustUTF16(class))),
			uintptr(unsafe.Pointer(mustUTF16(text))), style,
			uintptr(l.px(x)), uintptr(l.px(y)), uintptr(l.px(w)), uintptr(l.px(h)),
			uintptr(l.hwnd), uintptr(id), uintptr(l.inst), 0)
		return windows.HWND(r)
	}
	child := uintptr(wsChild | wsVisible)
	l.banner = add("STATIC", "", child|ssBitmap|ssCenterImage, 0, 20, 16, 480, 150, 1011)
	l.bannerBmp = l.scaledBanner(l.px(480), l.px(150))
	if l.bannerBmp != 0 {
		pSendMessageW.Call(uintptr(l.banner), stmSetImage, uintptr(imageBitmap), uintptr(l.bannerBmp))
	} else {
		l.title = add("STATIC", l.manifest.Name, child|ssLeft, 0, 28, 28, 300, 44, idTitle)
		l.sub = add("STATIC", "登录或注册后打开游戏窗口", child|ssLeft, 0, 30, 76, 380, 24, idSub)
	}
	l.version = add("STATIC", "v"+l.manifest.Version+" · 原生启动器", child|ssLeft, 0, 24, 176, 300, 22, 1012)

	l.labels[0] = add("STATIC", "服务器", child|ssLeft, 0, 30, 207, 70, 26, idLabServer)
	l.labels[1] = add("STATIC", "道号", child|ssLeft, 0, 30, 247, 70, 26, idLabName)
	l.labels[2] = add("STATIC", "密码", child|ssLeft, 0, 30, 287, 70, 26, idLabPass)
	edit := child | wsBorder | wsTabStop | esAutoHScroll
	l.server = add("EDIT", l.cfg.Server, edit, 0, 112, 205, 380, 30, idServer)
	l.name = add("EDIT", l.cfg.LastName, edit, 0, 112, 245, 380, 30, idName)
	l.pass = add("EDIT", "", edit|esPassword, 0, 112, 285, 380, 30, idPass)
	btn := child | wsTabStop
	l.loginBtn = add("BUTTON", "登录", btn|bsDefPush, 0, 112, 332, 146, 38, idLogin)
	l.registerBtn = add("BUTTON", "注册", btn|bsPush, 0, 346, 332, 146, 38, idRegister)
	l.status = add("STATIC", "输入账号密码后登录，或直接注册。", child|ssLeft, 0, 30, 386, 450, 26, idStatus)

	for _, h := range []windows.HWND{l.title, l.sub, l.version, l.labels[0], l.labels[1], l.labels[2], l.status} {
		if h != 0 {
			pSendMessageW.Call(uintptr(h), wmSetFont, uintptr(l.font), 1)
		}
	}
	if l.title != 0 {
		pSendMessageW.Call(uintptr(l.title), wmSetFont, uintptr(l.fontBold), 1)
	}
	if l.sub != 0 {
		pSendMessageW.Call(uintptr(l.sub), wmSetFont, uintptr(l.fontSmall), 1)
	}
	pSendMessageW.Call(uintptr(l.version), wmSetFont, uintptr(l.fontSmall), 1)
	pSendMessageW.Call(uintptr(l.status), wmSetFont, uintptr(l.fontSmall), 1)
	for _, h := range []windows.HWND{l.server, l.name, l.pass} {
		pSendMessageW.Call(uintptr(h), wmSetFont, uintptr(l.font), 1)
	}
	for _, h := range []windows.HWND{l.loginBtn, l.registerBtn} {
		pSendMessageW.Call(uintptr(h), wmSetFont, uintptr(l.font), 1)
	}
	if l.cfg.LastName != "" {
		pSetFocus.Call(uintptr(l.pass))
	} else {
		pSetFocus.Call(uintptr(l.name))
	}
}

// scaledBanner 把 assets/banner.bmp 缩放到当前 DPI 的控件尺寸，返回可直接给 STATIC 的 HBITMAP。
func (l *launcher) scaledBanner(w, h int) windows.Handle {
	path := ""
	for _, p := range candidatePaths(filepath.Join("assets", "banner.bmp")) {
		if _, err := os.Stat(p); err == nil {
			path = p
			break
		}
	}
	if path == "" || w <= 0 || h <= 0 {
		return 0
	}
	src, _, _ := pLoadImageW.Call(0, uintptr(unsafe.Pointer(mustUTF16(path))),
		imageBitmap, 0, 0, lrLoadFile|lrCreatedib)
	if src == 0 {
		return 0
	}
	screenDC, _, _ := pGetDC.Call(0)
	srcDC, _, _ := pCreateCompatibleDC.Call(screenDC)
	dstDC, _, _ := pCreateCompatibleDC.Call(screenDC)
	dst, _, _ := pCreateCompatibleBitmap.Call(screenDC, uintptr(w), uintptr(h))
	if srcDC == 0 || dstDC == 0 || dst == 0 {
		if srcDC != 0 {
			pDeleteDC.Call(srcDC)
		}
		if dstDC != 0 {
			pDeleteDC.Call(dstDC)
		}
		if dst != 0 {
			pDeleteObject.Call(dst)
		}
		pDeleteObject.Call(src)
		pReleaseDC.Call(0, screenDC)
		return 0
	}
	oldSrc, _, _ := pSelectObject.Call(srcDC, src)
	oldDst, _, _ := pSelectObject.Call(dstDC, dst)
	pStretchBlt.Call(dstDC, 0, 0, uintptr(w), uintptr(h), srcDC, 0, 0, 960, 300, srcCopy)
	pSelectObject.Call(srcDC, oldSrc)
	pSelectObject.Call(dstDC, oldDst)
	pDeleteDC.Call(srcDC)
	pDeleteDC.Call(dstDC)
	pDeleteObject.Call(src)
	pReleaseDC.Call(0, screenDC)
	return windows.Handle(dst)
}

func (l *launcher) loadIcon() {
	path := ""
	for _, p := range candidatePaths(filepath.Join("assets", "icon.ico")) {
		if _, err := os.Stat(p); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		return
	}
	h, _, _ := pLoadImageW.Call(0, uintptr(unsafe.Pointer(mustUTF16(path))),
		imageIcon, 0, 0, lrLoadFile|lrDefaultSize)
	if h == 0 {
		return
	}
	l.icon = windows.Handle(h)
	pSendMessageW.Call(uintptr(l.hwnd), wmSetIcon, 1, h)
	pSendMessageW.Call(uintptr(l.hwnd), wmSetIcon, 0, h)
}

func (l *launcher) center() {
	sw, _, _ := pGetSystemMetrics.Call(0)
	sh, _, _ := pGetSystemMetrics.Call(1)
	var r rect
	pGetWindowRect.Call(uintptr(l.hwnd), uintptr(unsafe.Pointer(&r)))
	w, h := int(r.Right-r.Left), int(r.Bottom-r.Top)
	x, y := (int(sw)-w)/2, (int(sh)-h)/2
	const noSize, noZ, noAct = 0x0001, 0x0004, 0x0010
	pSetWindowPos.Call(uintptr(l.hwnd), 0, uintptr(x), uintptr(y), 0, 0, noSize|noZ|noAct)
}

func launcherWndProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	l := appUI
	if l == nil {
		r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
		return r
	}
	switch msg {
	case wmEraseBkgnd:
		var r rect
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		pFillRect.Call(wparam, uintptr(unsafe.Pointer(&r)), uintptr(l.bgBrush))
		return 1
	case wmCtlColorStatic:
		if l.banner != 0 && windows.HWND(lparam) == l.banner {
			// 位图 Static 不能铺实心背景，否则会盖住横幅。
			nullBrush, _, _ := pGetStockObject.Call(5) // NULL_BRUSH
			return nullBrush
		}
		pSetBkMode.Call(wparam, transparent)
		if l.statusBad && windows.HWND(lparam) == l.status {
			pSetTextColor.Call(wparam, uintptr(color(232, 120, 110)))
		} else {
			pSetTextColor.Call(wparam, uintptr(color(226, 226, 214)))
		}
		pSetBkColor.Call(wparam, uintptr(color(16, 20, 26)))
		return uintptr(l.bgBrush)
	case wmCtlColorEdit:
		pSetBkMode.Call(wparam, transparent)
		pSetTextColor.Call(wparam, uintptr(color(226, 226, 214)))
		pSetBkColor.Call(wparam, uintptr(color(30, 37, 46)))
		return uintptr(l.panelBrush)
	case wmCommand:
		id := int(wparam & 0xFFFF)
		switch id {
		case idLogin:
			l.startAuth(false)
		case idRegister:
			l.startAuth(true)
		case 2: // IDCANCEL：Esc
			pDestroyWindow.Call(hwnd)
		}
		return 0
	case wmAppDone:
		select {
		case res := <-l.done:
			l.finishAuth(res)
		default:
		}
		return 0
	case wmGameDone:
		select {
		case err := <-l.gameDone:
			l.finishGameLaunch(err)
		default:
		}
		return 0
	case wmClose:
		pDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		if l.bannerBmp != 0 {
			pDeleteObject.Call(uintptr(l.bannerBmp))
			l.bannerBmp = 0
		}
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
	return r
}

func (l *launcher) startAuth(register bool) {
	if l.busy {
		return
	}
	server := strings.TrimSpace(getText(l.server))
	name := strings.TrimSpace(getText(l.name))
	pass := getText(l.pass)
	if server == "" || name == "" {
		l.setStatus("服务器和道号都要填。", true)
		return
	}
	if len([]rune(name)) < 2 || len([]rune(name)) > 8 {
		l.setStatus("道号长度需在 2–8 个字符之间。", true)
		return
	}
	if len([]rune(pass)) < 6 {
		l.setStatus("密码至少 6 位。", true)
		return
	}
	l.busy = true
	l.setStatus(map[bool]string{false: "登录中…", true: "注册中…"}[register], false)
	l.setInputsEnabled(false)
	go func() {
		res := auth(server, name, pass, register)
		l.done <- res
		pPostMessageW.Call(uintptr(l.hwnd), wmAppDone, 0, 0)
	}()
}

func (l *launcher) finishAuth(res authOutcome) {
	l.busy = false
	l.setInputsEnabled(true)
	if res.err != nil {
		l.setStatus("失败："+res.err.Error(), true)
		pSetFocus.Call(uintptr(l.pass))
		return
	}
	server := strings.TrimSpace(getText(l.server))
	name := strings.TrimSpace(getText(l.name))
	cfg := l.cfg
	cfg.Server, cfg.LastName = server, name
	saveConfig(cfg)

	l.busy = true
	l.setInputsEnabled(false)
	l.setStatus("登录成功，正在打开游戏…", false)
	if err := l.startGame(res.token, server); err != nil {
		l.busy = false
		l.setInputsEnabled(true)
		l.setStatus("启动游戏失败："+err.Error(), true)
		pSetFocus.Call(uintptr(l.pass))
	}
}

// startGame 拉起宿主后先等游戏窗口真正可见，再让启动器退出。
// 启动器立刻退出会把“游戏没起来”伪装成“窗口消失”，这里改用窗口就绪确认。
func (l *launcher) startGame(token, server string) error {
	game, err := gamePath(l.manifest.Game)
	if err != nil {
		return err
	}
	cmd := exec.Command(game, "--server", server, "--gui")
	cmd.SysProcAttr = gameProcessAttrs()
	env := make([]string, 0, len(os.Environ())+2)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "XIUXIAN_TOKEN=") && !strings.HasPrefix(kv, "XIUXIAN_SERVER=") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "XIUXIAN_TOKEN="+token, "XIUXIAN_SERVER="+server)
	cmd.Dir = filepath.Dir(game)
	if err := cmd.Start(); err != nil {
		return err
	}
	pid := uint32(cmd.Process.Pid)
	exited := make(chan error, 1)
	go func() {
		exited <- cmd.Wait()
	}()
	go l.watchGame(pid, cmd, exited)
	return nil
}

// 只禁止新控制台；不能设 HideWindow，它会让宿主第一次 ShowWindow 变成 SW_HIDE，
// 游戏窗口会存在但不可见，表现为点登录后启动器消失、别处什么都没出现。
func gameProcessAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
}

func (l *launcher) watchGame(pid uint32, cmd *exec.Cmd, exited <-chan error) {
	const wait = 12 * time.Second
	deadline := time.Now().Add(wait)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if gameWindowVisible(pid) {
			l.notifyGameDone(nil)
			return
		}
		select {
		case err := <-exited:
			l.notifyGameDone(fmt.Errorf("游戏进程提前退出：%v", err))
			return
		case <-ticker.C:
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			l.notifyGameDone(errors.New("游戏窗口未出现（已等待 12 秒）"))
			return
		}
	}
}

func (l *launcher) notifyGameDone(err error) {
	l.gameDone <- err
	pPostMessageW.Call(uintptr(l.hwnd), wmGameDone, 0, 0)
}

func (l *launcher) finishGameLaunch(err error) {
	if err == nil {
		pDestroyWindow.Call(uintptr(l.hwnd))
		return
	}
	l.busy = false
	l.setInputsEnabled(true)
	l.setStatus("打开游戏失败："+err.Error(), true)
	pSetFocus.Call(uintptr(l.pass))
}

func (l *launcher) setInputsEnabled(enabled bool) {
	flag := uintptr(0)
	if enabled {
		flag = 1
	}
	for _, h := range []windows.HWND{l.server, l.name, l.pass, l.loginBtn, l.registerBtn} {
		pEnableWindow.Call(uintptr(h), flag)
	}
}

var gameProbe struct {
	pid   uint32
	found bool
}

var gameProbeProc = windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
	var pid uint32
	pGetWindowThreadProcID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid != gameProbe.pid {
		return 1
	}
	if r, _, _ := pIsWindowVisible.Call(hwnd); r == 0 {
		return 1
	}
	var buf [64]uint16
	n, _, _ := pGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if int(n) > 0 && windows.UTF16ToString(buf[:int(n)]) == "JiuzhouWenDao" {
		gameProbe.found = true
		return 0
	}
	return 1
})

func gameWindowVisible(pid uint32) bool {
	gameProbe.pid = pid
	gameProbe.found = false
	pEnumWindows.Call(gameProbeProc, 0)
	return gameProbe.found
}

func gamePath(rel string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", errors.New("找不到启动器路径")
	}
	dir := filepath.Dir(exe)
	cands := make([]string, 0, 4)
	if rel != "" {
		cands = append(cands, filepath.Join(dir, filepath.FromSlash(rel)))
	}
	cands = append(cands, filepath.Join(dir, "game", "playcli.exe"), filepath.Join(dir, "playcli.exe"))
	for _, cand := range cands {
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}
	if p, err := exec.LookPath("playcli.exe"); err == nil {
		return p, nil
	}
	return "", errors.New("没找到游戏本体：检查 data/manifest.json 的 game 路径，或先 make dist")
}

func (l *launcher) setStatus(text string, bad bool) {
	l.statusBad = bad
	pSetWindowTextW.Call(uintptr(l.status), uintptr(unsafe.Pointer(mustUTF16(text))))
}

func getText(h windows.HWND) string {
	n, _, _ := pGetWindowTextLenW.Call(uintptr(h))
	buf := make([]uint16, int(n)+1)
	if len(buf) == 0 {
		return ""
	}
	pGetWindowTextW.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}

func auth(server, name, pass string, register bool) authOutcome {
	path := "/api/login"
	if register {
		path = "/api/register"
	}
	body, _ := json.Marshal(map[string]string{"name": name, "pass": pass})
	req, err := http.NewRequest(http.MethodPost, httpBase(server)+path, bytes.NewReader(body))
	if err != nil {
		return authOutcome{err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return authOutcome{err: fmt.Errorf("连不上服务端：%v", err)}
	}
	defer resp.Body.Close()
	text, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct{ Code, Msg string }
		if json.Unmarshal(text, &e) == nil && e.Msg != "" {
			return authOutcome{err: errors.New(e.Msg)}
		}
		return authOutcome{err: fmt.Errorf("服务端返回 %s", resp.Status)}
	}
	var out struct {
		Token   string `json:"token"`
		HasChar bool   `json:"hasChar"`
	}
	if json.Unmarshal(text, &out) != nil || out.Token == "" {
		return authOutcome{err: errors.New("登录响应没有 token")}
	}
	return authOutcome{token: out.Token, hasChar: out.HasChar}
}

func httpBase(s string) string {
	s = strings.TrimSuffix(strings.TrimSpace(s), "/")
	switch {
	case strings.HasPrefix(s, "wss://"):
		s = "https://" + strings.TrimPrefix(s, "wss://")
	case strings.HasPrefix(s, "ws://"):
		s = "http://" + strings.TrimPrefix(s, "ws://")
	case !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://"):
		s = "http://" + s
	}
	return strings.TrimSuffix(s, "/ws")
}

func brush(r, g, b byte) windows.Handle {
	h, _, _ := pCreateSolidBrush.Call(uintptr(color(r, g, b)))
	return windows.Handle(h)
}

func color(r, g, b byte) uint32 {
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

func makeFont(height int, weight uintptr) windows.Handle {
	h, _, _ := pCreateFontW.Call(
		uintptr(int32(-height)), 0, 0, 0, weight, 0, 0, 0,
		defaultCharset, 0, 0, 0, 0x31,
		uintptr(unsafe.Pointer(mustUTF16("Microsoft YaHei UI"))))
	return windows.Handle(h)
}

func loadCursor() windows.Handle {
	r, _, _ := pLoadCursorW.Call(0, 32512)
	return windows.Handle(r)
}

func messageBox(text string) {
	const mbOK, mbIconError = 0x00000000, 0x00000010
	pMessageBoxW.Call(0, uintptr(unsafe.Pointer(mustUTF16(text))),
		uintptr(unsafe.Pointer(mustUTF16("九州问道 · 登录"))), mbOK|mbIconError)
}
