//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "登录启动器只支持 Windows；其他平台直接运行 playcli。")
	os.Exit(1)
}
