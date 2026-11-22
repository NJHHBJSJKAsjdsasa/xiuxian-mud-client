//! 帧编解码：`{t,n,a,d}` 信封。
//!
//! 出站用构造函数拼 JSON，入站先解析成宽松信封再按 `t` 转具体视图。

use serde::Deserialize;
use serde_json::{Value, json};

use super::PROTO;

/// 入站帧。未知 `t` 也能解析，交给上层忽略。
#[derive(Debug, Clone, Deserialize)]
pub struct InFrame {
    pub t: String,
    #[serde(default)]
    pub n: Option<u64>,
    #[serde(default)]
    pub a: Option<String>,
    #[serde(default)]
    pub d: Value,
}

pub fn hello(token: &str) -> String {
    json!({"t": "hello", "d": {"token": token, "proto": PROTO}}).to_string()
}

pub fn act(seq: u64, action: &str, data: Value) -> String {
    json!({"t": "act", "n": seq, "a": action, "d": data}).to_string()
}

pub fn ping() -> String {
    json!({"t": "ping"}).to_string()
}

pub fn close() -> String {
    json!({"t": "close"}).to_string()
}
