//! 本地配置：`dirs::config_dir()/xiuxian-client/config.toml`。

use std::path::PathBuf;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(default)]
pub struct Config {
    pub server: String,
    pub ui_scale: f32,
    pub show_timestamps: bool,
    pub theme: String,
    pub last_name: String,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            server: "ws://127.0.0.1:8080".into(),
            ui_scale: 1.0,
            show_timestamps: true,
            theme: "dark".into(),
            last_name: String::new(),
        }
    }
}

pub fn dir() -> PathBuf {
    dirs::config_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join("xiuxian-client")
}

impl Config {
    pub fn path() -> PathBuf {
        dir().join("config.toml")
    }

    pub fn load() -> Self {
        let path = Self::path();
        match std::fs::read_to_string(&path) {
            Ok(text) => toml::from_str(&text).unwrap_or_else(|e| {
                tracing::warn!("配置文件无法解析（{}），使用默认值", e);
                Config::default()
            }),
            Err(_) => Config::default(),
        }
    }

    pub fn save(&self) {
        let dir = dir();
        if let Err(e) = std::fs::create_dir_all(&dir) {
            tracing::warn!("无法创建配置目录：{e}");
            return;
        }
        match toml::to_string_pretty(self) {
            Ok(text) => {
                if let Err(e) = std::fs::write(Self::path(), text) {
                    tracing::warn!("写配置失败：{e}");
                }
            }
            Err(e) => tracing::warn!("序列化配置失败：{e}"),
        }
    }
}

/// 日志目录：`dirs::data_local_dir()/xiuxian-client/logs`。
pub fn log_dir() -> PathBuf {
    dirs::data_local_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join("xiuxian-client")
        .join("logs")
}
