//go:build !windows

package main

import "errors"

// runGUI 在非 Windows 上没有实现：Win32 窗口与 GDI 只在 Windows 上可用。
func runGUI(a *app, cfg config) error {
	return errors.New("桌面窗口只支持 Windows")
}

func consoleOwned() bool { return false }
