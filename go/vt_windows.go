//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
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
