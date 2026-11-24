package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// 本地配置：记住服务器地址与上次道号。只存这两样，密码不落盘。

const defaultServer = "ws://127.0.0.1:8080"

const (
	windowModeBorderless = "borderless"
	windowModeWindowed   = "windowed"
)

type config struct {
	Server     string `json:"server"`
	LastName   string `json:"lastName"`
	Color      string `json:"color"`
	WindowMode string `json:"windowMode,omitempty"`
}

func (c config) borderless() bool {
	return c.WindowMode != windowModeWindowed
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
	if err != nil {
		return c
	}
	if json.Unmarshal(text, &c) != nil || c.Server == "" {
		return config{Server: defaultServer, Color: "true"}
	}
	return c
}

func (c config) save() {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if text, err := json.MarshalIndent(c, "", "  "); err == nil {
		_ = os.WriteFile(path, text, 0o644)
	}
}
