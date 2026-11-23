//go:build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")
	modUser32   = windows.NewLazySystemDLL("user32.dll")

	procGetConsoleProcessList  = modKernel32.NewProc("GetConsoleProcessList")
	procGetConsoleWindow       = modKernel32.NewProc("GetConsoleWindow")
	procSetConsoleTitleW       = modKernel32.NewProc("SetConsoleTitleW")
	procSetConsoleScreenBuffer = modKernel32.NewProc("SetConsoleScreenBufferSize")
	procGetWindowLongW         = modUser32.NewProc("GetWindowLongW")
	procSetWindowLongW         = modUser32.NewProc("SetWindowLongW")
	procSetWindowPos           = modUser32.NewProc("SetWindowPos")
)

const (
	wsVScroll      = 0x00200000
	wsHScroll      = 0x00100000
	gwlStyle       = ^uintptr(15) // -16，取窗口样式
	swpNoSize      = 0x0001
	swpNoMove      = 0x0002
	swpNoZOrder    = 0x0004
	swpNoActivate  = 0x0010
	swpFrameChange = 0x0020
)

// enableVT 打开 Windows 控制台的虚拟终端处理：
// 不开的话，写进去的 ANSI 转义会原样显示成 "[38;2;..."。
func enableVT() {
	h := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return
	}
	const enableVirtualTerminalProcessing = 0x0004
	if mode&enableVirtualTerminalProcessing == 0 {
		_ = windows.SetConsoleMode(h, mode|enableVirtualTerminalProcessing)
	}
}

// styleConsole 让双击打开的窗口更像一个应用，而不是一块黑控制台：
// 标题写游戏名，缓冲区收紧到窗口大小（去掉右侧滚动条）。
//
// 只在"本进程独占控制台"时动手。从 cmd / PowerShell / 终端里启动时，
// 那块窗口是用户的，改标题与缓冲区会干扰他。
func styleConsole() {
	if !ownsConsole() {
		return
	}
	if t, err := windows.UTF16PtrFromString("九州问道"); err == nil {
		_, _, _ = procSetConsoleTitleW.Call(uintptr(unsafe.Pointer(t)))
	}
	if os.Getenv("WT_SESSION") != "" {
		// Windows Terminal 自己管滚动条与窗口；动 ConPTY 缓冲区会让
		// 画面缩在标签页左上角，反而更难看。
		return
	}

	h := windows.Handle(os.Stdout.Fd())
	// 光标先归位：缓冲区缩小时若光标在窗口外，调用会失败。
	_ = windows.SetConsoleCursorPosition(h, windows.Coord{})
	var info windows.ConsoleScreenBufferInfo
	if windows.GetConsoleScreenBufferInfo(h, &info) != nil {
		return
	}
	w := int(info.Window.Right-info.Window.Left) + 1
	hgt := int(info.Window.Bottom-info.Window.Top) + 1
	if w < 20 || hgt < 8 {
		return
	}
	// 缓冲区 = 窗口：没有历史回滚，滚动条自然消失；界面本身有见闻翻页。
	_, _, _ = procSetConsoleScreenBuffer.Call(uintptr(h), coord(w, hgt))
	hideScrollbars()
}

// ownsConsole 判断控制台是不是本进程带起来的（双击 exe）。
// 返回 1 表示只有自己附着在这个控制台上。
func ownsConsole() bool {
	var list [4]uint32
	n, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&list[0])), uintptr(len(list)))
	return n == 1
}

func coord(x, y int) uintptr {
	return uintptr(uint16(x)) | uintptr(uint16(y))<<16
}

// hideScrollbars 去掉窗口的滚动条样式。缓冲区已经等于窗口，
// 这一步是双保险：某些系统上 conhost 会保留灰色的滚动条槽。
func hideScrollbars() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	style, _, _ := procGetWindowLongW.Call(hwnd, gwlStyle)
	style &^= wsVScroll | wsHScroll
	procSetWindowLongW.Call(hwnd, gwlStyle, style)
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		uintptr(swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChange))
}
