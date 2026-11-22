//! 界面层：只读 `store`，只发 `Intent`。不碰 socket、不发动作帧。

pub mod fonts;
pub mod logview;
pub mod theme;
pub mod views;

use crate::store::AppState;
use crate::util::text::thousands;
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
            Page::Social => "社交",
            Page::Market => "交易",
            Page::Sect => "宗门",
            Page::Map => "地图",
            Page::Settings => "设置",
        }
    }

    pub fn all() -> [Page; 9] {
        [
            Page::Game,
            Page::Character,
            Page::Bag,
            Page::Skills,
            Page::Social,
            Page::Market,
            Page::Sect,
            Page::Map,
            Page::Settings,
        ]
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
    Quit,
    /// 客户端本地设置变更，不需要发网络。
    Local,
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
    pub selected_exit: usize,
    pub toasts: Vec<Toast>,
    pub log_stick: bool,
    pub show_timestamps: bool,
    pub ui_scale: f32,
    pub fatal: String,
    pub font_source: String,
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
            selected_exit: 0,
            toasts: Vec::new(),
            log_stick: true,
            show_timestamps: true,
            ui_scale: 1.0,
            fatal: String::new(),
            font_source: String::new(),
        }
    }
}

impl UiState {
    pub fn toast(&mut self, text: impl Into<String>, bad: bool) {
        let text = text.into();
        // 连续同类失败合并，避免刷屏。
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
            until: time::now_ms() + if bad { 0 } else { 3000 },
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
}

pub fn root(ctx: &egui::Context, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    st.prune_toasts();
    match st.view {
        View::Login => views::login::show(ctx, st, out),
        View::Create => views::create::show(ctx, st, out),
        View::Fatal => views::fatal::show(ctx, st, out),
        View::Main => views::game::show(ctx, st, app, out),
    }
    draw_toasts(ctx, st);
}

fn draw_toasts(ctx: &egui::Context, st: &mut UiState) {
    if st.toasts.is_empty() {
        return;
    }
    let mut dismiss: Option<usize> = None;
    egui::Area::new(egui::Id::new("toasts"))
        .anchor(egui::Align2::RIGHT_BOTTOM, egui::vec2(-16.0_f32, -60.0_f32))
        .order(egui::Order::Foreground)
        .show(ctx, |ui| {
            ui.set_max_width(420.0);
            for (i, t) in st.toasts.iter().enumerate() {
                let color = if t.bad { theme::BAD } else { theme::ACCENT };
                egui::Frame::new()
                    .fill(theme::PANEL)
                    .stroke(egui::Stroke::new(1.0_f32, color))
                    .inner_margin(egui::Margin::symmetric(10, 6))
                    .corner_radius(6)
                    .show(ui, |ui| {
                        ui.horizontal(|ui| {
                            ui.label(egui::RichText::new(&t.text).color(color));
                            if t.sticky && ui.small_button("×").clicked() {
                                dismiss = Some(i);
                            }
                        });
                    });
            }
        });
    if let Some(i) = dismiss {
        st.toasts.remove(i);
    }
}

/// 顶栏与状态栏共用的小工具：千分位数字 + 弱化单位。
pub fn num(ui: &mut egui::Ui, label: &str, value: i64, color: egui::Color32) {
    ui.label(egui::RichText::new(label).small().color(theme::DIM));
    ui.label(egui::RichText::new(thousands(value)).color(color));
}
