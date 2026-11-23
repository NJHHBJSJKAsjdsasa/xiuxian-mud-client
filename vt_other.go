//go:build !windows

package main

// enableVT 在 Unix 终端上无需额外处理：ANSI 是默认语言。
func enableVT() {}
