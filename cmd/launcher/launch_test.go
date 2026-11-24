//go:build windows

package main

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestGameProcessAttrsDoesNotHideWindow(t *testing.T) {
	attrs := gameProcessAttrs()
	if attrs.HideWindow {
		t.Fatal("HideWindow 会把游戏窗口的第一次 ShowWindow 强制成 SW_HIDE")
	}
	if attrs.CreationFlags&uint32(windows.CREATE_NO_WINDOW) == 0 {
		t.Fatal("缺少 CREATE_NO_WINDOW，宿主可能继承控制台")
	}
}
