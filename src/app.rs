//! 应用状态机：键盘 → Intent → 网络命令；网络事件 → 状态 → 重绘。
//!
//! 这一层不碰终端，也不碰协议细节：方便单测与替换渲染层。

use crossbeam_channel::Receiver;
use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};
use tokio::sync::mpsc::UnboundedSender;

use crate::config::Config;
use crate::net::{self, ConnStatus, NetCmd, NetEvent};
use crate::protocol::action;
use crate::protocol::err;
use crate::protocol::msg::LogLine;
use crate::store::AppState;
use crate::util::text::truncate_chars;
use crate::util::time;
use crate::view::{Intent, Page, UiState, View};

pub struct App {
    pub state: AppState,
    pub ui: UiState,
    pub cfg: Config,
    pub quit: bool,
    /// 日志向上翻了几行（0 = 贴底）。
    pub log_back: usize,
    tx: UnboundedSender<NetCmd>,
    rx: Receiver<NetEvent>,
    rt: Option<tokio::runtime::Runtime>,
    submit_at: i64,
    pending_login: Option<(String, String)>,
    pending_create: Option<String>,
    preferred_page: Option<Page>,
    /// `--move n`：进主界面后自动走第 n 个出口。
    pending_move: Option<usize>,
    /// 下一次允许补拉舆图的时刻（毫秒）。赶路中服务端会拒 `worldMap`（E-BUSY），
    /// 拒了就等一会儿再要，否则地名会一直停在出发地。
    map_retry_at: i64,
}

impl App {
    pub fn new(cfg: Config) -> Self {
        let rt = tokio::runtime::Builder::new_multi_thread()
            .worker_threads(2)
            .enable_all()
            .thread_name("xiuxian-net")
            .build()
            .expect("tokio runtime 初始化失败");
        let (cmd_tx, cmd_rx) = tokio::sync::mpsc::unbounded_channel();
        let (ev_tx, ev_rx) = crossbeam_channel::bounded(4096);
        rt.spawn(async move {
            net::ws::run(cmd_rx, ev_tx).await;
        });

        let mut ui = UiState::default();
        ui.login.server = cfg.server.clone();
        ui.login.name = cfg.last_name.clone();

        Self {
            state: AppState::default(),
            ui,
            cfg,
            quit: false,
            log_back: 0,
            tx: cmd_tx,
            rx: ev_rx,
            rt: Some(rt),
            submit_at: 0,
            pending_login: None,
            pending_create: None,
            preferred_page: None,
            pending_move: None,
            map_retry_at: 0,
        }
    }

    pub fn auto_login(&mut self, name: String, pass: String) {
        self.ui.login.name = name.clone();
        self.ui.login.pass = pass.clone();
        self.pending_login = Some((name, pass));
    }

    pub fn auto_create(&mut self, name: String) {
        self.pending_create = Some(name);
    }

    pub fn auto_move(&mut self, index: usize) {
        self.pending_move = Some(index);
    }

    pub fn start_page(&mut self, name: &str) {
        self.ui.page = match name {
            "character" => Page::Character,
            "bag" => Page::Bag,
            "skills" => Page::Skills,
            "social" => Page::Social,
            "market" => Page::Market,
            "sect" => Page::Sect,
            "map" => Page::Map,
            "settings" => Page::Settings,
            _ => Page::Game,
        };
        self.preferred_page = Some(self.ui.page);
    }

    /// 发一个动作：统一在这里记日志，排查"到底发了什么"时只看这一处。
    fn act(&mut self, a: &str, d: serde_json::Value) {
        tracing::debug!("→ {a} {d}");
        let _ = self.tx.send(NetCmd::Act { a: a.into(), d });
    }

    pub fn drain_net(&mut self) {
        let mut n = 0;
        while let Ok(ev) = self.rx.try_recv() {
            self.handle_event(ev);
            n += 1;
            if n > 512 {
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
                        if self.ui.view == View::Login && self.state.has_char {
                            self.ui.view = View::Main;
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
                self.ui.login.loading = false;
                self.ui.view = if r.has_character() {
                    View::Main
                } else {
                    View::Create
                };
                if r.has_character() {
                    self.ui.page = self.preferred_page.take().unwrap_or(Page::Game);
                    self.ui.selected_exit = 0;
                    self.act(action::WORLD_MAP, serde_json::json!({}));
                }
                if let (false, Some(name)) = (r.has_character(), self.pending_create.take()) {
                    self.act(action::CREATE, serde_json::json!({ "name": name }));
                }
            }
            NetEvent::State(p) => {
                let moved = self
                    .state
                    .player
                    .get()
                    .map(|old| old.loc != p.loc)
                    .unwrap_or(false);
                self.state.apply_state(&p);
                if moved {
                    self.act(action::WORLD_MAP, serde_json::json!({}));
                }
            }
            NetEvent::Vital(v) => self.state.apply_vital(&v),
            NetEvent::Log(lines) => {
                self.state.push_logs(lines);
                self.log_back = 0;
            }
            NetEvent::Map(m) => {
                self.state.apply_map(&m);
                self.ui.selected_exit = 0;
                if let Some(i) = self.pending_move.take()
                    && let Some(ex) = m.exits.get(i)
                    && ex.ok
                {
                    let to = ex.to.clone();
                    self.act(action::MOVE, serde_json::json!({ "to": to }));
                }
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
                self.log_back = 0;
            }
            NetEvent::Presence(count) => self.state.presence = count,
            NetEvent::Battle(b) => self.state.battle = Some(*b),
            NetEvent::Ack { .. } => {}
            NetEvent::Err { code, msg, .. } => {
                self.ui.login.loading = false;
                // 建号竞态：create 刚落库时若先到的 worldMap 被服务端判成"还没角色"，
                // 这不是用户的错，紧跟其后的 ready/state 会把画面补齐，所以不弹红条。
                if code == err::E_NOCHAR {
                    tracing::debug!("建号尚未就绪，忽略 E-NOCHAR");
                    return;
                }
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

    pub fn intent(&mut self, it: Intent) {
        match it {
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
                self.act(action::CREATE, serde_json::json!({ "name": name }));
            }
            Intent::Move(to) => {
                if self.online() {
                    self.act(action::MOVE, serde_json::json!({ "to": to }));
                }
            }
            Intent::Explore => {
                if self.online() {
                    let region = self
                        .state
                        .world
                        .get()
                        .map(|m| m.region.clone())
                        .unwrap_or_default();
                    self.act(action::EXPLORE, serde_json::json!({ "region": region }));
                }
            }
            Intent::Cultivate(on) => {
                if self.online() {
                    self.act(action::CULTIVATE, serde_json::json!({ "on": on }));
                }
            }
            Intent::Breakthrough => {
                if self.online() {
                    self.act(action::BREAKTHROUGH, serde_json::json!({}));
                }
            }
            Intent::Look => {
                if self.online() {
                    self.act(action::LOOK, serde_json::json!({}));
                }
            }
            Intent::Chat { ch, to, text } => {
                if self.online() {
                    let _ = self.tx.send(NetCmd::Chat { ch, to, text });
                }
            }
            Intent::Logout => {
                let _ = self.tx.send(NetCmd::Logout);
                self.state.clear_session();
                self.ui.view = View::Login;
                self.ui.login.loading = false;
                self.ui.page = Page::Game;
            }
        }
    }

    fn online(&self) -> bool {
        matches!(self.state.conn, ConnStatus::Online)
    }

    pub fn on_key(&mut self, k: KeyEvent) {
        if k.modifiers.contains(KeyModifiers::CONTROL) && k.code == KeyCode::Char('c') {
            self.quit = true;
            return;
        }
        match self.ui.view {
            View::Login => self.key_login(k),
            View::Create => self.key_create(k),
            View::Fatal => {
                if matches!(k.code, KeyCode::Char('q') | KeyCode::Esc | KeyCode::Enter) {
                    self.quit = true;
                }
            }
            View::Main => self.key_main(k),
        }
    }

    fn key_login(&mut self, k: KeyEvent) {
        let focus = self.ui.login.focus;
        match k.code {
            KeyCode::Tab | KeyCode::Down => self.ui.login.focus = (focus + 1) % 3,
            KeyCode::Up => self.ui.login.focus = (focus + 2) % 3,
            KeyCode::Enter => {
                let register = self.ui.login.register;
                self.intent(Intent::Login { register });
            }
            KeyCode::Char('r') if k.modifiers.contains(KeyModifiers::CONTROL) => {
                self.ui.login.register = !self.ui.login.register;
                self.ui.login.error.clear();
            }
            KeyCode::Char(c) => {
                let target = match focus {
                    0 => &mut self.ui.login.server,
                    1 => &mut self.ui.login.name,
                    _ => &mut self.ui.login.pass,
                };
                target.push(c);
            }
            KeyCode::Backspace => {
                let target = match focus {
                    0 => &mut self.ui.login.server,
                    1 => &mut self.ui.login.name,
                    _ => &mut self.ui.login.pass,
                };
                target.pop();
            }
            _ => {}
        }
    }

    fn key_create(&mut self, k: KeyEvent) {
        match k.code {
            KeyCode::Enter => {
                let name = self.ui.create_name.trim().to_string();
                if (2..=8).contains(&name.chars().count()) {
                    self.intent(Intent::Create(name));
                } else {
                    self.ui.toast("道号长度需在 2–8 个字符之间", true);
                }
            }
            KeyCode::Char(c) => self.ui.create_name.push(c),
            KeyCode::Backspace => {
                self.ui.create_name.pop();
            }
            _ => {}
        }
    }

    fn key_main(&mut self, k: KeyEvent) {
        if self.ui.typing {
            match k.code {
                KeyCode::Enter => {
                    let text = self.ui.chat_text.trim().to_string();
                    if !text.is_empty() {
                        let it = Intent::Chat {
                            ch: self.ui.chat_ch.clone(),
                            to: if self.ui.chat_ch == "dm" {
                                Some(self.ui.chat_to.trim().to_string())
                            } else {
                                None
                            },
                            text,
                        };
                        self.intent(it);
                    }
                    self.ui.chat_text.clear();
                    self.ui.typing = false;
                    self.ui.chat_stick = true;
                    self.log_back = 0;
                }
                KeyCode::Esc => {
                    self.ui.chat_text.clear();
                    self.ui.typing = false;
                }
                KeyCode::Char(c) => self.ui.chat_text.push(c),
                KeyCode::Backspace => {
                    self.ui.chat_text.pop();
                }
                KeyCode::Tab => {
                    self.ui.chat_ch = match self.ui.chat_ch.as_str() {
                        "world" => "region".into(),
                        "region" => "party".into(),
                        "party" => "dm".into(),
                        _ => "world".into(),
                    };
                }
                _ => {}
            }
            return;
        }

        if let KeyCode::F(n) = k.code
            && let Some(p) = Page::from_fn_key(n)
        {
            self.ui.page = p;
            return;
        }

        match k.code {
            KeyCode::Esc => {
                if self.ui.page != Page::Game {
                    self.ui.page = Page::Game;
                }
            }
            KeyCode::Char('q') => self.quit = true,
            KeyCode::Char('/') => {
                self.ui.typing = true;
                self.ui.chat_text.clear();
            }
            KeyCode::Char(' ') => self.intent(Intent::Explore),
            KeyCode::Char('c') => {
                let on = !self.state.vitals.cultivating;
                self.intent(Intent::Cultivate(on));
            }
            KeyCode::Char('b') => self.intent(Intent::Breakthrough),
            KeyCode::Char('l') => self.intent(Intent::Look),
            KeyCode::Char('o') => self.intent(Intent::Logout),
            KeyCode::Up | KeyCode::Char('k') => {
                let n = self.state.world.get().map(|m| m.exits.len()).unwrap_or(0);
                if n > 0 {
                    self.ui.selected_exit = (self.ui.selected_exit + n - 1) % n;
                }
            }
            KeyCode::Down | KeyCode::Char('j') => {
                let n = self.state.world.get().map(|m| m.exits.len()).unwrap_or(0);
                if n > 0 {
                    self.ui.selected_exit = (self.ui.selected_exit + 1) % n;
                }
            }
            KeyCode::Enter => {
                if let Some(m) = self.state.world.get()
                    && let Some(ex) = m.exits.get(self.ui.selected_exit)
                    && ex.ok
                {
                    let to = ex.to.clone();
                    self.intent(Intent::Move(to));
                }
            }
            KeyCode::PageUp => {
                let max = self.state.logs.len().saturating_sub(1);
                self.log_back = (self.log_back + 5).min(max);
            }
            KeyCode::PageDown => {
                self.log_back = self.log_back.saturating_sub(5);
            }
            KeyCode::Home => {
                let max = self.state.logs.len().saturating_sub(1);
                self.log_back = max;
            }
            KeyCode::End => self.log_back = 0,
            _ => {}
        }
    }

    pub fn tick(&mut self) {
        self.refresh_stale_map();
        if self.ui.login.loading && self.submit_at > 0 && time::now_ms() - self.submit_at > 12_000 {
            self.ui.login.loading = false;
            self.ui.login.error = "服务端没有响应，请检查地址与网络".into();
        }
        self.ui.prune_toasts();
        if let Some((name, pass)) = self.pending_login.take()
            && self.ui.view == View::Login
            && !self.ui.login.loading
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

    /// 舆图与角色位置不一致时补拉一次 `worldMap`。
    ///
    /// `worldMap` 在赶路途中会被服务端以 E-BUSY 拒掉（服务端口径：此刻不可为），
    /// 所以不能只在"位置变了"那一刻要一次；到点后按 600ms 节流重试，
    /// 直到 `map` 帧把 `world.loc` 对齐到 `player.loc`。
    fn refresh_stale_map(&mut self) {
        if !self.online() || !self.state.has_char {
            return;
        }
        let (Some(map), Some(player)) = (self.state.world.get(), self.state.player.get()) else {
            return;
        };
        if map.loc == player.loc {
            return;
        }
        let now = time::now_ms();
        if now < self.map_retry_at {
            return;
        }
        self.map_retry_at = now + 600;
        self.act(action::WORLD_MAP, serde_json::json!({}));
    }

    pub fn shutdown(&mut self) {
        let _ = self.tx.send(NetCmd::Shutdown);
        if let Some(rt) = self.rt.take() {
            rt.shutdown_timeout(std::time::Duration::from_millis(200));
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::view::Page;

    fn key(c: KeyCode) -> KeyEvent {
        KeyEvent::new(c, KeyModifiers::NONE)
    }

    #[test]
    fn slash_starts_typing_and_esc_cancels() {
        let mut app = App::new(Config::default());
        app.ui.view = View::Main;
        app.on_key(key(KeyCode::Char('/')));
        assert!(app.ui.typing);
        app.on_key(key(KeyCode::Char('你')));
        app.on_key(key(KeyCode::Char('好')));
        assert_eq!(app.ui.chat_text, "你好");
        app.on_key(key(KeyCode::Esc));
        assert!(!app.ui.typing);
        assert!(app.ui.chat_text.is_empty());
        app.shutdown();
    }

    #[test]
    fn f_keys_switch_pages_and_esc_returns() {
        let mut app = App::new(Config::default());
        app.ui.view = View::Main;
        app.on_key(key(KeyCode::F(2)));
        assert_eq!(app.ui.page, Page::Bag);
        app.on_key(key(KeyCode::Esc));
        assert_eq!(app.ui.page, Page::Game);
        app.shutdown();
    }
}
