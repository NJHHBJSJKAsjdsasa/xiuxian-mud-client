//go:build windows && dll

package main

/*
#include <stdlib.h>
*/
import "C"

import "os"

func runFromEnv(server, token string) int {
	if token != "" {
		_ = os.Setenv("XIUXIAN_TOKEN", token)
	}
	args := []string{"playcli", "--gui"}
	if server != "" {
		args = []string{"playcli", "--server", server, "--gui"}
	}
	os.Args = args
	runMain()
	return 0
}

// RunGame 是 DLL 入口：gamehost.dll 把服务器与 token 传进来，游戏以 GUI 模式运行。
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
	return C.int(runFromEnv(s, t))
}
