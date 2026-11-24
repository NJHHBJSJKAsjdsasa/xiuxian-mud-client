//go:build windows && dll

package main

/*
#include <stdlib.h>
*/
import "C"

import "os"

// RunGame 是 DLL 入口：启动器/宿主把服务器与 token 传进来，游戏以 GUI 模式运行。
//
//export RunGame
func RunGame(server *C.char, token *C.char) C.int {
	s := ""
	if server != nil {
		s = C.GoString(server)
	}
	t := ""
	if token != nil {
		t = C.GoString(token)
	}
	if t != "" {
		_ = os.Setenv("XIUXIAN_TOKEN", t)
	}
	args := []string{"playcli", "--gui"}
	if s != "" {
		args = []string{"playcli", "--server", s, "--gui"}
	}
	os.Args = args
	runMain()
	return 0
}
