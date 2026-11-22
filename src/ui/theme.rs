//! 主题令牌：全项目唯一色源，别处不写字面量。
//!
//! 视觉基调：夜墨底、宣纸字、玉青点睛、朱砂示警。
//! 标题与场景名用衬线（楷/宋），正文用无衬线，保证小字号可读。

use egui::{Color32, FontFamily, FontId, Stroke, TextStyle};

pub const BG: Color32 = Color32::from_rgb(0x0E, 0x11, 0x14);
pub const PANEL: Color32 = Color32::from_rgb(0x15, 0x1A, 0x1F);
pub const PANEL_HI: Color32 = Color32::from_rgb(0x1D, 0x24, 0x2B);
pub const CARD: Color32 = Color32::from_rgb(0x12, 0x17, 0x1B);
pub const LINE: Color32 = Color32::from_rgb(0x2A, 0x33, 0x3C);
pub const TEXT: Color32 = Color32::from_rgb(0xE6, 0xE2, 0xD8);
pub const DIM: Color32 = Color32::from_rgb(0x8B, 0x94, 0x9C);
pub const FAINT: Color32 = Color32::from_rgb(0x5A, 0x63, 0x6B);
pub const ACCENT: Color32 = Color32::from_rgb(0x7F, 0xD1, 0xC1);
pub const GOOD: Color32 = Color32::from_rgb(0x6F, 0xBF, 0x73);
pub const BAD: Color32 = Color32::from_rgb(0xC4, 0x55, 0x3D);
pub const WARN: Color32 = Color32::from_rgb(0xC9, 0xA2, 0x27);
pub const GOLD: Color32 = Color32::from_rgb(0xD9, 0xB3, 0x6A);
pub const PVP: Color32 = Color32::from_rgb(0xA9, 0x8B, 0xC8);

/// 衬线家族名，由 `ui::fonts` 注册。
pub const SERIF: &str = "serif";

/// 皮肤：水墨（默认）或终端（等宽字符 + 直角 + 无渐变）。
static TERMINAL: std::sync::atomic::AtomicBool = std::sync::atomic::AtomicBool::new(false);

pub fn set_terminal(on: bool) {
    TERMINAL.store(on, std::sync::atomic::Ordering::Relaxed);
}

pub fn terminal() -> bool {
    TERMINAL.load(std::sync::atomic::Ordering::Relaxed)
}

fn body_family() -> FontFamily {
    if terminal() {
        FontFamily::Monospace
    } else {
        FontFamily::Proportional
    }
}

/// 标题：终端皮肤下也用等宽，保持"同一个字符网格"。
pub fn title_family() -> FontFamily {
    if terminal() {
        FontFamily::Monospace
    } else {
        serif()
    }
}

pub fn title_id(size: f32) -> FontId {
    FontId::new(size, title_family())
}

pub fn serif() -> FontFamily {
    FontFamily::Name(SERIF.into())
}

pub fn serif_id(size: f32) -> FontId {
    if terminal() {
        FontId::new(size, FontFamily::Monospace)
    } else {
        FontId::new(size, serif())
    }
}

/// 日志类型 → 颜色；未知类型按系统处理。
pub fn tone(k: &str) -> Color32 {
    match k {
        "sys" => DIM,
        "nar" => Color32::from_rgb(0xC9, 0xB8, 0x96),
        "combat" => TEXT,
        "good" => GOOD,
        "bad" => BAD,
        "lvl" => GOLD,
        "chat" => ACCENT,
        "pvp" => PVP,
        _ => TEXT,
    }
}

pub fn mix(a: Color32, b: Color32, t: f32) -> Color32 {
    let t = t.clamp(0.0, 1.0);
    let f = |x: u8, y: u8| (x as f32 + (y as f32 - x as f32) * t).round() as u8;
    Color32::from_rgba_unmultiplied(
        f(a.r(), b.r()),
        f(a.g(), b.g()),
        f(a.b(), b.b()),
        f(a.a(), b.a()),
    )
}

pub fn apply(ctx: &egui::Context) {
    // 固定深色：系统是浅色主题时也不跟着变。
    ctx.set_theme(egui::ThemePreference::Dark);

    let mut v = egui::Visuals::dark();
    v.panel_fill = BG;
    v.window_fill = PANEL;
    v.extreme_bg_color = Color32::from_rgb(0x0B, 0x0E, 0x11);
    v.faint_bg_color = PANEL_HI;
    v.override_text_color = Some(TEXT);
    v.selection.bg_fill = Color32::from_rgb(0x1C, 0x3D, 0x3A);
    v.selection.stroke = Stroke::new(1.0_f32, ACCENT);
    v.widgets.noninteractive.bg_fill = PANEL;
    v.widgets.noninteractive.bg_stroke = Stroke::new(1.0_f32, LINE);
    v.widgets.inactive.bg_fill = PANEL_HI;
    v.widgets.inactive.bg_stroke = Stroke::new(1.0_f32, LINE);
    v.widgets.inactive.fg_stroke = Stroke::new(1.0_f32, TEXT);
    v.widgets.hovered.bg_fill = Color32::from_rgb(0x24, 0x2E, 0x35);
    v.widgets.hovered.bg_stroke = Stroke::new(1.0_f32, ACCENT);
    v.widgets.active.bg_fill = Color32::from_rgb(0x1C, 0x3D, 0x3A);
    v.widgets.active.bg_stroke = Stroke::new(1.0_f32, ACCENT);
    v.window_stroke = Stroke::new(1.0_f32, LINE);
    // 控件圆角小一些，接近木牌/签条，而不是网页卡片。
    let radius = if terminal() { 0 } else { 3 };
    v.widgets.noninteractive.corner_radius = egui::CornerRadius::same(radius);
    v.widgets.inactive.corner_radius = egui::CornerRadius::same(radius);
    v.widgets.hovered.corner_radius = egui::CornerRadius::same(radius);
    v.widgets.active.corner_radius = egui::CornerRadius::same(radius);
    if terminal() {
        v.panel_fill = Color32::from_rgb(0x05, 0x07, 0x09);
        v.extreme_bg_color = Color32::from_rgb(0x03, 0x04, 0x06);
        v.window_fill = Color32::from_rgb(0x05, 0x07, 0x09);
    }

    // 用 all_styles_mut 一次写入两套主题：
    // 只调 set_visuals 会被随后 set_style 的旧 style 覆盖（TextEdit 底仍是浅色）。
    ctx.all_styles_mut(|style| {
        style.visuals = v.clone();
        style.spacing.item_spacing = egui::vec2(8.0, 6.0);
        style.spacing.button_padding = egui::vec2(10.0, 4.0);
        style
            .text_styles
            .insert(TextStyle::Heading, FontId::new(22.0, serif()));
        style
            .text_styles
            .insert(TextStyle::Body, FontId::new(14.5, body_family()));
        style.text_styles.insert(
            TextStyle::Button,
            FontId::new(14.0, FontFamily::Proportional),
        );
        style.text_styles.insert(
            TextStyle::Small,
            FontId::new(12.0, FontFamily::Proportional),
        );
        style.text_styles.insert(
            TextStyle::Monospace,
            FontId::new(13.0, FontFamily::Monospace),
        );
    });
}
