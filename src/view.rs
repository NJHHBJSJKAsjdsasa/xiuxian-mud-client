//! 视图状态与用户意图：纯数据，不依赖任何 UI 框架。

use crate::util::time;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum View {
    Login,
    Create,
    Main,
    Fatal,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Page {
    Game,
    Character,
    Bag,
    Skills,
    Social,
    Market,
    Sect,
    Map,
    Settings,
}

impl Page {
    pub fn label(self) -> &'static str {
        match self {
            Page::Game => "游戏",
            Page::Character => "角色",
            Page::Bag => "背包",
            Page::Skills => "技能",
            Page::Social => "江湖",
            Page::Market => "坊市",
            Page::Sect => "宗门",
            Page::Map => "舆图",
            Page::Settings => "设置",
        }
    }

    /// F1–F8 对应表（游戏页用 Esc 回）。
    pub fn from_fn_key(n: u8) -> Option<Page> {
        Some(match n {
            1 => Page::Character,
            2 => Page::Bag,
            3 => Page::Skills,
            4 => Page::Social,
            5 => Page::Market,
            6 => Page::Sect,
            7 => Page::Map,
            8 => Page::Settings,
            _ => return None,
        })
    }
}

#[derive(Debug, Clone)]
pub enum Intent {
    Login {
        register: bool,
    },
    Create(String),
    Move(String),
    Explore,
    Cultivate(bool),
    Breakthrough,
    Chat {
        ch: String,
        to: Option<String>,
        text: String,
    },
    Look,
    Logout,
}

pub struct Toast {
    pub text: String,
    pub bad: bool,
    pub until: i64,
    pub sticky: bool,
}

pub struct LoginForm {
    pub server: String,
    pub name: String,
    pub pass: String,
    pub register: bool,
    pub loading: bool,
    pub error: String,
    /// 当前在编辑第几个输入框（0 服务器 / 1 道号 / 2 密码）。
    pub focus: usize,
}

impl Default for LoginForm {
    fn default() -> Self {
        Self {
            server: "ws://127.0.0.1:8080".into(),
            name: String::new(),
            pass: String::new(),
            register: false,
            loading: false,
            error: String::new(),
            focus: 0,
        }
    }
}

pub struct UiState {
    pub view: View,
    pub page: Page,
    pub login: LoginForm,
    pub create_name: String,
    pub chat_ch: String,
    pub chat_to: String,
    pub chat_text: String,
    /// 是否处于聊天输入模式（Enter 提交、Esc 取消）。
    pub typing: bool,
    pub chat_stick: bool,
    pub selected_exit: usize,
    pub toasts: Vec<Toast>,
    pub fatal: String,
}

impl Default for UiState {
    fn default() -> Self {
        Self {
            view: View::Login,
            page: Page::Game,
            login: LoginForm::default(),
            create_name: String::new(),
            chat_ch: "world".into(),
            chat_to: String::new(),
            chat_text: String::new(),
            typing: false,
            chat_stick: true,
            selected_exit: 0,
            toasts: Vec::new(),
            fatal: String::new(),
        }
    }
}

impl UiState {
    pub fn toast(&mut self, text: impl Into<String>, bad: bool) {
        let text = text.into();
        if let Some(last) = self.toasts.last_mut()
            && last.bad == bad
            && last.text == text
        {
            last.until = time::now_ms() + 3000;
            return;
        }
        self.toasts.push(Toast {
            text,
            bad,
            until: time::now_ms() + if bad { 8000 } else { 3000 },
            sticky: bad,
        });
        if self.toasts.len() > 3 {
            self.toasts.remove(0);
        }
    }

    pub fn prune_toasts(&mut self) {
        let now = time::now_ms();
        self.toasts.retain(|t| t.sticky || t.until > now);
    }

    pub fn active_toast(&self) -> Option<&Toast> {
        self.toasts.last()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fn_keys_map_to_pages() {
        assert_eq!(Page::from_fn_key(1), Some(Page::Character));
        assert_eq!(Page::from_fn_key(8), Some(Page::Settings));
        assert_eq!(Page::from_fn_key(9), None);
    }

    #[test]
    fn toast_dedups_consecutive_same_text() {
        let mut st = UiState::default();
        st.toast("连接断开", true);
        st.toast("连接断开", true);
        assert_eq!(st.toasts.len(), 1);
        st.toast("别的事", false);
        assert_eq!(st.toasts.len(), 2);
    }
}
