//! 网络层：REST 登录 + WebSocket 长连接。
//!
//! 只认协议，不懂业务语义。UI 通过 `NetCmd` 下命令，通过 `NetEvent` 收结果。

pub mod http;
pub mod ws;

use serde_json::Value;

use crate::protocol::msg::{
    BattleView, ChatMsg, LogLine, MapView, PlayerView, ReadyView, VitalView,
};

#[derive(Debug, Clone, PartialEq)]
pub enum ConnStatus {
    Idle,
    LoggingIn,
    Connecting,
    Online,
    /// 退避中：`secs` 后重试，`attempt` 是第几次。
    Backoff {
        secs: u64,
        attempt: u32,
    },
    /// 正常断开（主动退出 / 被顶号 / token 失效）。
    Offline(String),
    /// 不可恢复：版本不兼容等，界面只留退出。
    Fatal(String),
}

#[derive(Debug)]
pub enum NetCmd {
    Login {
        server: String,
        name: String,
        pass: String,
        register: bool,
    },
    Act {
        a: String,
        d: Value,
    },
    Chat {
        ch: String,
        to: Option<String>,
        text: String,
    },
    Logout,
    Shutdown,
}

#[derive(Debug)]
#[allow(dead_code)] // `n`/`extra` 保留给 M2 的动作对账与错误细化。
pub enum NetEvent {
    Status(ConnStatus),
    Ready(Box<ReadyView>),
    State(Box<PlayerView>),
    Vital(Box<VitalView>),
    Log(Vec<LogLine>),
    Map(Box<MapView>),
    Chat(Box<ChatMsg>),
    Presence(i64),
    Battle(Box<BattleView>),
    Ack {
        n: Option<u64>,
    },
    Err {
        n: Option<u64>,
        code: String,
        msg: String,
        extra: Value,
    },
    Announce(String),
    /// 服务端要求清 token 回登录（4003）。
    TokenExpired,
}

pub fn emit(tx: &crossbeam_channel::Sender<NetEvent>, ctx: &egui::Context, ev: NetEvent) {
    if tx.send(ev).is_ok() {
        ctx.request_repaint();
    }
}
