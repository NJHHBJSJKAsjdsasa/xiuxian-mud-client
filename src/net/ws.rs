//! WebSocket 会话：连接、hello、心跳、收发、断线与重连。
//!
//! 命令方向用 tokio 的 unbounded 通道（UI 侧同步 send、网络侧异步 recv，避免在 async 里阻塞）；
//! 事件方向用 crossbeam（UI 侧每帧 try_recv）。

use std::time::{Duration, Instant};

use crossbeam_channel::Sender;
use futures_util::{SinkExt, StreamExt};
use serde_json::{Value, json};
use tokio::sync::mpsc::UnboundedReceiver;
use tokio::time::{interval, sleep};
use tokio_tungstenite::tungstenite::Message;

use super::{ConnStatus, NetCmd, NetEvent, emit, http};
use crate::protocol::err::classify;
use crate::protocol::frame::{self, InFrame};
use crate::protocol::msg::{
    BattleView, ChatMsg, LogLine, MapView, PlayerView, ReadyView, VitalView,
};

struct Creds {
    server: String,
    token: String,
}

enum SessionEnd {
    Shutdown,
    Logout,
    Fatal(String),
    Closed { code: u16, reason: String },
}

pub async fn run(mut cmds: UnboundedReceiver<NetCmd>, ev: Sender<NetEvent>) {
    loop {
        // 等登录命令
        let (server, name, pass, register) = loop {
            match cmds.recv().await {
                Some(NetCmd::Login {
                    server,
                    name,
                    pass,
                    register,
                }) => break (server, name, pass, register),
                Some(NetCmd::Shutdown) | None => return,
                Some(_) => continue,
            }
        };
        emit(&ev, NetEvent::Status(ConnStatus::LoggingIn));

        let auth = if register {
            http::register(&server, &name, &pass).await
        } else {
            http::login(&server, &name, &pass).await
        };
        let auth = match auth {
            Ok(a) if !a.token.is_empty() => a,
            Ok(_) => {
                emit(
                    &ev,
                    NetEvent::Err {
                        n: None,
                        code: "E-AUTH".into(),
                        msg: "服务端没有返回 token".into(),
                        extra: Value::Null,
                    },
                );
                emit(&ev, NetEvent::Status(ConnStatus::Idle));
                continue;
            }
            Err(e) => {
                emit(
                    &ev,
                    NetEvent::Err {
                        n: None,
                        code: "E-AUTH".into(),
                        msg: format!("{e}"),
                        extra: Value::Null,
                    },
                );
                emit(&ev, NetEvent::Status(ConnStatus::Idle));
                continue;
            }
        };

        let creds = Creds {
            server,
            token: auth.token,
        };
        let mut attempt = 0u32;

        loop {
            emit(&ev, NetEvent::Status(ConnStatus::Connecting));
            match session(&ev, &mut cmds, &creds).await {
                SessionEnd::Shutdown => return,
                SessionEnd::Logout => {
                    let _ = http::logout(&creds.server, &creds.token).await;
                    emit(
                        &ev,
                        NetEvent::Status(ConnStatus::Offline("已退出登录".into())),
                    );
                    break;
                }
                SessionEnd::Fatal(msg) => {
                    emit(&ev, NetEvent::Status(ConnStatus::Fatal(msg)));
                    return;
                }
                SessionEnd::Closed { code, reason } => {
                    let kind = classify(code);
                    if !kind.retryable() {
                        let msg = if reason.is_empty() {
                            kind.user_message().to_string()
                        } else {
                            format!("{}（{code} {reason}）", kind.user_message())
                        };
                        emit(&ev, NetEvent::Status(ConnStatus::Offline(msg)));
                        break;
                    }
                    attempt += 1;
                    let secs = kind.backoff_secs(attempt);
                    emit(&ev, NetEvent::Status(ConnStatus::Backoff { secs, attempt }));
                    if !wait_backoff(&mut cmds, secs).await {
                        return;
                    }
                }
            }
        }
    }
}

/// 退避等待，期间可被 Shutdown / Logout / 新 Login 打断。
/// 返回 false 表示应当退出整个网络任务。
async fn wait_backoff(cmds: &mut UnboundedReceiver<NetCmd>, secs: u64) -> bool {
    let deadline = Instant::now() + Duration::from_secs(secs);
    loop {
        let left = deadline.saturating_duration_since(Instant::now());
        if left.is_zero() {
            return true;
        }
        tokio::select! {
            _ = sleep(left) => return true,
            cmd = cmds.recv() => match cmd {
                None | Some(NetCmd::Shutdown) => return false,
                Some(NetCmd::Logout) => return false,
                // 退避期间的新登录：交给外层重新走登录（当前实现直接放弃本次会话）
                Some(NetCmd::Login { .. }) => return false,
                Some(_) => continue,
            },
        }
    }
}

async fn session(
    ev: &Sender<NetEvent>,
    cmds: &mut UnboundedReceiver<NetCmd>,
    creds: &Creds,
) -> SessionEnd {
    let url = http::ws_url(&creds.server);
    let ws = match tokio::time::timeout(
        Duration::from_secs(8),
        tokio_tungstenite::connect_async(url.as_str()),
    )
    .await
    {
        Ok(Ok((ws, _resp))) => ws,
        Ok(Err(e)) => {
            return SessionEnd::Closed {
                code: 1001,
                reason: format!("连接失败：{e}"),
            };
        }
        Err(_) => {
            return SessionEnd::Closed {
                code: 1001,
                reason: "连接超时".into(),
            };
        }
    };

    let (mut write, mut read) = ws.split();

    // hello 先于任何业务帧；服务端 5 秒内等首帧。
    if write
        .send(Message::Text(frame::hello(&creds.token).into()))
        .await
        .is_err()
    {
        return SessionEnd::Closed {
            code: 1001,
            reason: "发送 hello 失败".into(),
        };
    }

    let mut seq: u64 = 0;
    let mut ping = interval(Duration::from_secs(25));
    ping.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
    let mut last_rx = Instant::now();
    let mut ready_seen = false;

    loop {
        tokio::select! {
            incoming = read.next() => {
                match incoming {
                    Some(Ok(Message::Text(txt))) => {
                        last_rx = Instant::now();
                        if let Some(end) = handle_text(ev, txt.as_str(), &mut ready_seen) {
                            return end;
                        }
                    }
                    Some(Ok(Message::Binary(_))) => { /* 协议只有文本帧，忽略 */ }
                    Some(Ok(Message::Ping(p))) => {
                        let _ = write.send(Message::Pong(p)).await;
                    }
                    Some(Ok(Message::Pong(_))) => { last_rx = Instant::now(); }
                    Some(Ok(Message::Frame(_))) => {}
                    Some(Ok(Message::Close(frame))) => {
                        let (code, reason) = match frame {
                            Some(f) => (u16::from(f.code), f.reason.to_string()),
                            None => (1000, String::new()),
                        };
                        return SessionEnd::Closed { code, reason };
                    }
                    Some(Err(e)) => {
                        return SessionEnd::Closed { code: 1001, reason: format!("读取失败：{e}") };
                    }
                    None => {
                        return SessionEnd::Closed { code: 1001, reason: "连接被关闭".into() };
                    }
                }
            }
            cmd = cmds.recv() => {
                match cmd {
                    None | Some(NetCmd::Shutdown) => {
                        let _ = write.send(Message::Close(None)).await;
                        return SessionEnd::Shutdown;
                    }
                    Some(NetCmd::Logout) => {
                        let _ = write.send(Message::Text(frame::close().into())).await;
                        let _ = write.send(Message::Close(None)).await;
                        return SessionEnd::Logout;
                    }
                    Some(NetCmd::Login { .. }) => {
                        // 已在线又发登录：忽略（UI 只在离线时给登录入口）
                    }
                    Some(NetCmd::Act { a, d }) => {
                        seq += 1;
                        if let Err(e) = write.send(Message::Text(frame::act(seq, &a, d).into())).await {
                            return SessionEnd::Closed { code: 1001, reason: format!("发送失败：{e}") };
                        }
                    }
                    Some(NetCmd::Chat { ch, to, text }) => {
                        seq += 1;
                        let mut d = json!({"ch": ch, "text": text});
                        if let Some(to) = to { d["to"] = Value::String(to); }
                        if let Err(e) = write.send(Message::Text(frame::act(seq, "chat", d).into())).await {
                            return SessionEnd::Closed { code: 1001, reason: format!("发送失败：{e}") };
                        }
                    }
                }
            }
            _ = ping.tick() => {
                if last_rx.elapsed() > Duration::from_secs(60) {
                    return SessionEnd::Closed { code: 1001, reason: "心跳超时".into() };
                }
                if write.send(Message::Text(frame::ping().into())).await.is_err() {
                    return SessionEnd::Closed { code: 1001, reason: "心跳发送失败".into() };
                }
            }
        }
    }
}

fn handle_text(ev: &Sender<NetEvent>, txt: &str, ready_seen: &mut bool) -> Option<SessionEnd> {
    let frame: InFrame = match serde_json::from_str(txt) {
        Ok(f) => f,
        Err(e) => {
            tracing::warn!("无法解析的帧：{e}；原文 {}", truncate(txt, 300));
            return None;
        }
    };

    match frame.t.as_str() {
        "ready" => {
            match serde_json::from_value::<ReadyView>(frame.d) {
                Ok(r) => {
                    *ready_seen = true;
                    emit(ev, NetEvent::Ready(Box::new(r)));
                    // ready 才是"握手 + 鉴权都过了"的标志；之前一律算连接中。
                    emit(ev, NetEvent::Status(ConnStatus::Online));
                }
                Err(e) => {
                    return Some(SessionEnd::Fatal(format!("ready 帧字段不兼容：{e}")));
                }
            }
        }
        "state" => match serde_json::from_value::<PlayerView>(frame.d) {
            Ok(v) => emit(ev, NetEvent::State(Box::new(v))),
            Err(e) => tracing::warn!("state 解析失败：{e}"),
        },
        "vital" => match serde_json::from_value::<VitalView>(frame.d) {
            Ok(v) => emit(ev, NetEvent::Vital(Box::new(v))),
            Err(e) => tracing::warn!("vital 解析失败：{e}"),
        },
        "log" => {
            let lines: Vec<LogLine> = frame
                .d
                .get("lines")
                .and_then(|l| serde_json::from_value(l.clone()).ok())
                .unwrap_or_default();
            if !lines.is_empty() {
                emit(ev, NetEvent::Log(lines));
            }
        }
        "map" => match serde_json::from_value::<MapView>(frame.d) {
            Ok(v) => emit(ev, NetEvent::Map(Box::new(v))),
            Err(e) => tracing::warn!("map 解析失败：{e}"),
        },
        "chat" => match serde_json::from_value::<ChatMsg>(frame.d) {
            Ok(v) => emit(ev, NetEvent::Chat(Box::new(v))),
            Err(e) => tracing::warn!("chat 解析失败：{e}"),
        },
        "presence" => {
            let count = frame.d.get("count").and_then(|c| c.as_i64()).unwrap_or(0);
            emit(ev, NetEvent::Presence(count));
        }
        "battle" => match serde_json::from_value::<BattleView>(frame.d) {
            Ok(v) => emit(ev, NetEvent::Battle(Box::new(v))),
            Err(e) => tracing::warn!("battle 解析失败：{e}"),
        },
        "ack" => emit(ev, NetEvent::Ack { n: frame.n }),
        "err" => {
            let code = frame
                .d
                .get("code")
                .and_then(|c| c.as_str())
                .unwrap_or("E-BADREQ")
                .to_string();
            let msg = frame
                .d
                .get("msg")
                .and_then(|m| m.as_str())
                .unwrap_or("操作失败")
                .to_string();
            let extra = frame.d.get("extra").cloned().unwrap_or(Value::Null);
            if code == "E-BADREQ"
                && let Some(e) = extra.get("error").and_then(|v| v.as_str())
                && e == "proto"
            {
                // 服务端会在 err 之后用 4004 关闭；这里只记录。
                tracing::warn!("协议版本不兼容：{msg}");
            }
            emit(
                ev,
                NetEvent::Err {
                    n: frame.n,
                    code,
                    msg,
                    extra,
                },
            );
        }
        "announce" => {
            let text = frame
                .d
                .get("text")
                .and_then(|t| t.as_str())
                .unwrap_or_default()
                .to_string();
            if !text.is_empty() {
                emit(ev, NetEvent::Announce(text));
            }
        }
        "evict" => {
            let reason = frame
                .d
                .get("reason")
                .and_then(|r| r.as_str())
                .unwrap_or("账号在别处登录")
                .to_string();
            emit(ev, NetEvent::Status(ConnStatus::Offline(reason)));
        }
        other => {
            tracing::debug!("忽略未知消息类型：{other}");
        }
    }
    None
}

fn truncate(s: &str, n: usize) -> String {
    if s.chars().count() <= n {
        s.to_string()
    } else {
        s.chars().take(n).collect::<String>() + "…"
    }
}
