//! 客户端状态：唯一可变源，由服务端帧驱动。
//! 这里不出现任何 `egui` 类型。

pub mod logbuf;
pub mod player;
pub mod world;

pub use logbuf::LogBuf;
pub use player::PlayerState;
pub use world::WorldState;

use crate::net::ConnStatus;
use crate::protocol::msg::{
    BattleView, ChatMsg, LogLine, MapView, PlayerView, ReadyView, VitalView,
};
use crate::util::time;

/// 聊天历史每频道上限（与服务端无关，纯展示缓存）。
const CHAT_KEEP: usize = 200;

pub struct AppState {
    pub conn: ConnStatus,
    pub player: PlayerState,
    pub world: WorldState,
    pub vitals: VitalView,
    pub presence: i64,
    pub logs: LogBuf,
    pub chats: Vec<ChatMsg>,
    pub battle: Option<BattleView>,
    /// `server_now - local_now`，毫秒。
    pub clock_offset: i64,
    pub server_season: String,
    pub server_daypart: String,
    pub has_char: bool,
}

impl Default for AppState {
    fn default() -> Self {
        Self {
            conn: ConnStatus::Idle,
            player: PlayerState::default(),
            world: WorldState::default(),
            vitals: VitalView::default(),
            presence: 0,
            logs: LogBuf::new(5000),
            chats: Vec::new(),
            battle: None,
            clock_offset: 0,
            server_season: String::new(),
            server_daypart: String::new(),
            has_char: false,
        }
    }
}

impl AppState {
    pub fn server_now(&self) -> i64 {
        time::now_ms() + self.clock_offset
    }

    pub fn apply_ready(&mut self, r: &ReadyView) {
        self.clock_offset = if r.server_now > 0 {
            r.server_now - time::now_ms()
        } else {
            0
        };
        self.presence = r.online_count();
        self.has_char = r.has_character();
        self.server_season = r.season.clone();
        self.server_daypart = r.daypart.clone();
        if let Some(p) = &r.player {
            self.apply_state(p);
        }
        for c in &r.chat_hist {
            self.push_chat(c.clone());
        }
    }

    /// `state` 是**全量快照**：整体替换，不做字段级合并。
    pub fn apply_state(&mut self, p: &PlayerView) {
        self.player.set(p.clone());
        self.vitals.hp = p.hp;
        self.vitals.mp = p.mp;
        self.vitals.exp = p.exp;
        self.vitals.exp_need = p.exp_need;
        self.vitals.stamina = p.stamina;
        self.vitals.cultivating = p.cultivating;
        self.vitals.buffs = p.buffs.clone();
    }

    pub fn apply_vital(&mut self, v: &VitalView) {
        self.vitals = v.clone();
        self.player.patch_vitals(v);
    }

    pub fn apply_map(&mut self, m: &MapView) {
        // `look` 的应答是单节点视图（exits 为空），不能拿它覆盖当前节点的出口，
        // 否则看一眼四周就把路全看没了。
        if let Some(old) = self.world.get()
            && m.exits.is_empty()
            && !old.loc.is_empty()
            && old.loc == m.loc
        {
            let mut merged = m.clone();
            merged.exits = old.exits.clone();
            self.world.set(merged);
            return;
        }
        self.world.set(m.clone());
    }

    pub fn push_logs(&mut self, lines: Vec<LogLine>) {
        for l in lines {
            self.logs.push(l);
        }
    }

    pub fn push_chat(&mut self, c: ChatMsg) {
        self.chats.push(c);
        let len = self.chats.len();
        if len > CHAT_KEEP {
            self.chats.drain(0..len - CHAT_KEEP);
        }
    }

    pub fn clear_session(&mut self) {
        let clock = self.clock_offset;
        let logs = std::mem::replace(&mut self.logs, LogBuf::new(5000));
        *self = AppState {
            clock_offset: clock,
            logs,
            ..AppState::default()
        };
    }
}
