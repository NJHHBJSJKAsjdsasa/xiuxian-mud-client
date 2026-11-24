//go:build windows

package main

import (
	"path/filepath"
	"strings"
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

func TestGameCommandHostModes(t *testing.T) {
	cmd, err := gameCommand(`C:\game\gamehost.dll`, "ws://127.0.0.1:8080")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(strings.ToLower(cmd.Path)) != "rundll32.exe" {
		t.Fatalf("DLL 宿主应通过 rundll32 启动，实际 %q", cmd.Path)
	}
	if len(cmd.Args) < 2 || cmd.Args[1] != "gamehost.dll,Launch" {
		t.Fatalf("rundll32 参数错误：%v", cmd.Args)
	}
	exe, err := gameCommand(`C:\game\playcli.exe`, "ws://127.0.0.1:8080")
	if err != nil {
		t.Fatal(err)
	}
	if exe.Args[0] != `C:\game\playcli.exe` || len(exe.Args) != 4 {
		t.Fatalf("exe 宿主参数错误：%v", exe.Args)
	}
}
