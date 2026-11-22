//! `eframe::App` 实现：每帧 drain 事件 → 更新 store → 画一帧 → 处理 Intent。

use crossbeam_channel::Receiver;
use eframe::egui;
use tokio::sync::mpsc::UnboundedSender;

use crate::config::Config;
use crate::net::{self, ConnStatus, NetCmd, NetEvent};
use crate::protocol::action;
use crate::protocol::msg::LogLine;
use crate::store::AppState;
use crate::ui::{self, Intent, Page, UiState, View, fonts, theme};
use crate::util::text::truncate_chars;
use crate::util::time;

pub struct ClientApp {
    state: AppState,
    ui: UiState,
    cfg: Config,
    tx: UnboundedSender<NetCmd>,
    rx: Receiver<NetEvent>,
    rt: Option<tokio::runtime::Runtime>,
    /// 登录/建号提交时刻，用于 12 秒无响应兜底。
    submit_at: i64,
    /// 待自动登录的凭据（`--login 道号:密码`）。
    pending_login: Option<(String, String)>,
    /// 待自动建号的道号（`--create`）。
    pending_create: Option<String>,
}

impl ClientApp {
    pub fn new(cc: &eframe::CreationContext<'_>) -> Self {
        theme::apply(&cc.egui_ctx);
        let font_source = fonts::install(&cc.egui_ctx);
        cc.egui_ctx.set_zoom_factor(1.0);

        let cfg = Config::load();
        let mut ui = UiState::default();
        ui.login.server = cfg.server.clone();
        ui.login.name = cfg.last_name.clone();
        ui.show_timestamps = cfg.show_timestamps;
        ui.ui_scale = if cfg.ui_scale > 0.0 {
            cfg.ui_scale
        } else {
            1.0
        };
        ui.font_source = font_source;
        cc.egui_ctx.set_zoom_factor(ui.ui_scale);

        let rt = tokio::runtime::Builder::new_multi_thread()
            .worker_threads(2)
            .enable_all()
            .thread_name("xiuxian-net")
            .build()
            .expect("tokio runtime 初始化失败");

        let (cmd_tx, cmd_rx) = tokio::sync::mpsc::unbounded_channel();
        let (ev_tx, ev_rx) = crossbeam_channel::bounded(2048);
        let ctx = cc.egui_ctx.clone();
        rt.spawn(async move {
            net::ws::run(ctx, cmd_rx, ev_tx).await;
        });

        Self {
            state: AppState::default(),
            ui,
            cfg,
            tx: cmd_tx,
            rx: ev_rx,
            rt: Some(rt),
            submit_at: 0,
            pending_login: None,
            pending_create: None,
        }
    }

    /// 开发/联调：启动后自动用给定凭据登录。
    pub fn auto_login(&mut self, name: String, pass: String) {
        self.ui.login.name = name.clone();
        self.ui.login.pass = pass.clone();
        self.pending_login = Some((name, pass));
    }

    /// 开发/联调：登录后如果还没有角色，自动建号。
    pub fn auto_create(&mut self, name: String) {
        self.pending_create = Some(name);
    }

    /// 命令行 `--server` 覆盖配置文件里的地址。
    pub fn apply_server_override(&mut self, server: &str) {
        if !server.is_empty() {
            self.ui.login.server = server.to_string();
        }
    }

    fn drain_events(&mut self) {
        let mut n = 0;
        while let Ok(ev) = self.rx.try_recv() {
            n += 1;
            self.handle_event(ev);
            if n > 512 {
                // 单帧最多处理 512 条，避免一帧卡死；剩下的下一帧继续。
                break;
            }
        }
    }

    fn handle_event(&mut self, ev: NetEvent) {
        match ev {
            NetEvent::Status(s) => {
                let prev = self.state.conn.clone();
                self.state.conn = s.clone();
                match &s {
                    ConnStatus::Online => {
                        self.ui.login.loading = false;
                        if self.ui.view != View::Fatal {
                            // ready 决定进主界面还是建号页；这里只在已有角色时兜底。
                            if self.ui.view == View::Login && self.state.has_char {
                                self.ui.view = View::Main;
                            }
                        }
                    }
                    ConnStatus::Offline(msg) => {
                        self.ui.login.loading = false;
                        self.ui.toast(msg.clone(), true);
                        if self.ui.view != View::Fatal {
                            self.ui.view = View::Login;
                        }
                        self.state.clear_session();
                    }
                    ConnStatus::Fatal(msg) => {
                        self.ui.login.loading = false;
                        self.ui.fatal = msg.clone();
                        self.ui.view = View::Fatal;
                    }
                    ConnStatus::Backoff { .. }
                        if self.ui.view == View::Main
                            && !matches!(prev, ConnStatus::Backoff { .. }) =>
                    {
                        self.ui.toast("连接中断，正在重连…", true);
                    }
                    _ => {}
                }
            }
            NetEvent::Ready(r) => {
                self.state.apply_ready(&r);
                if !r.has_character()
                    && let Some(name) = self.pending_create.take()
                {
                    let _ = self.tx.send(NetCmd::Act {
                        a: action::CREATE.into(),
                        d: serde_json::json!({ "name": name }),
                    });
                }
                self.ui.login.loading = false;
                self.ui.view = if r.has_character() {
                    View::Main
                } else {
                    View::Create
                };
                if r.has_character() {
                    self.ui.page = Page::Game;
                    self.ui.selected_exit = 0;
                    // ready 不带地图视图，主动拉一次当前节点。
                    let _ = self.tx.send(NetCmd::Act {
                        a: action::WORLD_MAP.into(),
                        d: serde_json::json!({}),
                    });
                }
            }
            NetEvent::State(p) => {
                // 到达/被传送后 loc 会变，但服务端只推 state + log：
                // 客户端要主动拉一次当前节点，否则界面停在旧地名上。
                let moved = self
                    .state
                    .player
                    .get()
                    .map(|old| old.loc != p.loc)
                    .unwrap_or(false);
                self.state.apply_state(&p);
                if moved {
                    let _ = self.tx.send(NetCmd::Act {
                        a: action::WORLD_MAP.into(),
                        d: serde_json::json!({}),
                    });
                }
            }
            NetEvent::Vital(v) => self.state.apply_vital(&v),
            NetEvent::Log(lines) => self.state.push_logs(lines),
            NetEvent::Map(m) => {
                self.state.apply_map(&m);
                self.ui.selected_exit = 0;
            }
            NetEvent::Chat(c) => {
                let head = match c.channel.as_str() {
                    "world" => "世界",
                    "region" => "区域",
                    "party" => "队伍",
                    "dm" => "私聊",
                    other => other,
                };
                let prefix = if c.npc { "【旁白】" } else { "" };
                let line = format!("[{head}] {prefix}{}：{}", c.from, c.text);
                self.state.push_logs(vec![LogLine {
                    k: "chat".into(),
                    s: truncate_chars(&line, 512),
                }]);
                self.state.push_chat(*c);
            }
            NetEvent::Presence(count) => self.state.presence = count,
            NetEvent::Battle(b) => {
                self.state.battle = Some(*b);
            }
            NetEvent::Ack { .. } => {}
            NetEvent::Err { code, msg, .. } => {
                self.ui.login.loading = false;
                self.ui.toast(format!("{msg}（{code}）"), true);
                if self.ui.view == View::Login {
                    self.ui.login.error = msg;
                }
            }
            NetEvent::Announce(text) => {
                self.state.push_logs(vec![LogLine {
                    k: "sys".into(),
                    s: format!("【公告】{text}"),
                }]);
            }
            NetEvent::TokenExpired => {
                self.ui.login.loading = false;
                self.ui.view = View::Login;
                self.ui.login.error = "登录已过期，请重新登录".into();
                self.state.clear_session();
            }
        }
    }

    fn handle_intent(&mut self, intent: Intent, ctx: &egui::Context) {
        match intent {
            Intent::Login { register } => {
                let server = self.ui.login.server.trim().to_string();
                let name = self.ui.login.name.trim().to_string();
                let pass = self.ui.login.pass.clone();
                if server.is_empty() || name.is_empty() || pass.len() < 6 {
                    self.ui.login.loading = false;
                    self.ui.login.error = "服务器、道号、密码（≥6 位）都要填".into();
                    return;
                }
                self.cfg.server = server.clone();
                self.cfg.last_name = name.clone();
                self.cfg.save();
                self.submit_at = time::now_ms();
                let _ = self.tx.send(NetCmd::Login {
                    server,
                    name,
                    pass,
                    register,
                });
            }
            Intent::Create(name) => {
                self.submit_at = time::now_ms();
                let _ = self.tx.send(NetCmd::Act {
                    a: action::CREATE.into(),
                    d: serde_json::json!({ "name": name }),
                });
            }
            Intent::Move(to) => {
                if !self.online() {
                    return;
                }
                let _ = self.tx.send(NetCmd::Act {
                    a: action::MOVE.into(),
                    d: serde_json::json!({ "to": to }),
                });
            }
            Intent::Explore => {
                if !self.online() {
                    return;
                }
                let region = self
                    .state
                    .world
                    .get()
                    .map(|m| m.region.clone())
                    .unwrap_or_default();
                let _ = self.tx.send(NetCmd::Act {
                    a: action::EXPLORE.into(),
                    d: serde_json::json!({ "region": region }),
                });
            }
            Intent::Cultivate(on) => {
                if !self.online() {
                    return;
                }
                let _ = self.tx.send(NetCmd::Act {
                    a: action::CULTIVATE.into(),
                    d: serde_json::json!({ "on": on }),
                });
            }
            Intent::Breakthrough => {
                if !self.online() {
                    return;
                }
                let _ = self.tx.send(NetCmd::Act {
                    a: action::BREAKTHROUGH.into(),
                    d: serde_json::json!({}),
                });
            }
            Intent::Look => {
                if !self.online() {
                    return;
                }
                let _ = self.tx.send(NetCmd::Act {
                    a: action::LOOK.into(),
                    d: serde_json::json!({}),
                });
            }
            Intent::Chat { ch, to, text } => {
                if !self.online() {
                    return;
                }
                let _ = self.tx.send(NetCmd::Chat { ch, to, text });
            }
            Intent::Logout => {
                let _ = self.tx.send(NetCmd::Logout);
                self.state.clear_session();
                self.ui.view = View::Login;
                self.ui.login.loading = false;
                self.ui.page = Page::Game;
            }
            Intent::Quit => {
                let _ = self.tx.send(NetCmd::Shutdown);
                ctx.send_viewport_cmd(egui::ViewportCommand::Close);
            }
            Intent::Local => {
                self.cfg.ui_scale = self.ui.ui_scale;
                self.cfg.show_timestamps = self.ui.show_timestamps;
                self.cfg.save();
            }
        }
    }

    fn online(&self) -> bool {
        matches!(self.state.conn, ConnStatus::Online)
    }

    fn shortcuts(&mut self, ctx: &egui::Context, out: &mut Vec<Intent>) {
        if self.ui.view != View::Main {
            return;
        }
        let typing = ctx.wants_keyboard_input();
        let key = |k: egui::Key| ctx.input(|i| i.key_pressed(k));

        if key(egui::Key::Escape) {
            if typing {
                ctx.memory_mut(|m| m.surrender_focus(egui::Id::NULL));
            } else {
                self.ui.page = Page::Game;
            }
        }
        if typing {
            return;
        }

        let pages = [
            (egui::Key::F1, Page::Character),
            (egui::Key::F2, Page::Bag),
            (egui::Key::F3, Page::Skills),
            (egui::Key::F4, Page::Social),
            (egui::Key::F5, Page::Market),
            (egui::Key::F6, Page::Sect),
            (egui::Key::F7, Page::Map),
            (egui::Key::F8, Page::Settings),
        ];
        for (k, p) in pages {
            if key(k) {
                self.ui.page = p;
            }
        }
        if self.ui.page != Page::Game {
            return;
        }
        if key(egui::Key::Space) {
            out.push(Intent::Explore);
        }
        if key(egui::Key::C) {
            out.push(Intent::Cultivate(!self.state.vitals.cultivating));
        }
        if key(egui::Key::B) && self.state.battle.is_some() {
            let _ = self.tx.send(NetCmd::Act {
                a: action::BATTLE.into(),
                d: serde_json::json!({ "op": "attack" }),
            });
        }
    }

    /// 需要下一帧重绘的依据：动画、倒计时、退避。
    fn repaint_after(&self) -> Option<std::time::Duration> {
        if !self.ui.toasts.is_empty() {
            return Some(std::time::Duration::from_millis(120));
        }
        if matches!(self.state.conn, ConnStatus::Backoff { .. }) {
            return Some(std::time::Duration::from_millis(500));
        }
        if let Some(t) = self.state.player.get().and_then(|p| p.travel.as_ref()) {
            let left = t.arrive_at - self.state.server_now();
            if left > 0 {
                return Some(std::time::Duration::from_millis(500));
            }
        }
        if self.state.vitals.cultivating {
            return Some(std::time::Duration::from_millis(1000));
        }
        None
    }
}

impl eframe::App for ClientApp {
    fn update(&mut self, ctx: &egui::Context, _frame: &mut eframe::Frame) {
        self.drain_events();

        // 提交后 12 秒没有任何回音，恢复按钮，避免"永远转圈"。
        if self.ui.login.loading && self.submit_at > 0 && time::now_ms() - self.submit_at > 12_000 {
            self.ui.login.loading = false;
            self.ui.login.error = "服务端没有响应，请检查地址与网络".into();
        }

        if let Some((name, pass)) = self.pending_login.take()
            && self.ui.view == View::Login
            && !self.ui.login.loading
        {
            {
                self.ui.login.loading = true;
                self.ui.login.error.clear();
                self.submit_at = time::now_ms();
                let server = self.ui.login.server.clone();
                let _ = self.tx.send(NetCmd::Login {
                    server,
                    name,
                    pass,
                    register: false,
                });
            }
        }

        let mut out: Vec<Intent> = Vec::new();
        self.shortcuts(ctx, &mut out);
        ui::root(ctx, &mut self.ui, &self.state, &mut out);
        for it in out {
            self.handle_intent(it, ctx);
        }
        if let Some(d) = self.repaint_after() {
            ctx.request_repaint_after(d);
        }
    }

    fn on_exit(&mut self, _gl: Option<&eframe::glow::Context>) {
        let _ = self.tx.send(NetCmd::Shutdown);
        if let Some(rt) = self.rt.take() {
            rt.shutdown_timeout(std::time::Duration::from_millis(200));
        }
    }
}
