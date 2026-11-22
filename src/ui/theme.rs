//! 主题令牌：全项目唯一色源，别处不写字面量。

use egui::{Color32, FontFamily, FontId, Stroke, TextStyle};

pub const BG: Color32 = Color32::from_rgb(0x14, 0x16, 0x1A);
pub const PANEL: Color32 = Color32::from_rgb(0x1B, 0x1E, 0x24);
pub const PANEL_HI: Color32 = Color32::from_rgb(0x23, 0x28, 0x30);
pub const LINE: Color32 = Color32::from_rgb(0x2E, 0x34, 0x3D);
pub const TEXT: Color32 = Color32::from_rgb(0xE6, 0xE8, 0xEB);
pub const DIM: Color32 = Color32::from_rgb(0x9A, 0xA0, 0xA6);
pub const ACCENT: Color32 = Color32::from_rgb(0x45, 0xC8, 0xDC);
pub const GOOD: Color32 = Color32::from_rgb(0x4C, 0xAF, 0x50);
pub const BAD: Color32 = Color32::from_rgb(0xE5, 0x48, 0x4D);
pub const WARN: Color32 = Color32::from_rgb(0xC9, 0xA2, 0x27);
pub const GOLD: Color32 = Color32::from_rgb(0xD4, 0xA6, 0x4A);
pub const PVP: Color32 = Color32::from_rgb(0xB0, 0x8C, 0xE8);

/// 日志类型 → 颜色；未知类型按系统处理。
pub fn tone(k: &str) -> Color32 {
    match k {
        "sys" => DIM,
        "nar" => WARN,
        "combat" => TEXT,
        "good" => GOOD,
        "bad" => BAD,
        "lvl" => GOLD,
        "chat" => ACCENT,
        "pvp" => PVP,
        _ => TEXT,
    }
}

pub fn apply(ctx: &egui::Context) {
    // 固定深色：系统是浅色主题时也不跟着变。
    ctx.set_theme(egui::ThemePreference::Dark);

    let mut v = egui::Visuals::dark();
    v.panel_fill = BG;
    v.window_fill = PANEL;
    v.extreme_bg_color = Color32::from_rgb(0x10, 0x12, 0x16);
    v.faint_bg_color = PANEL_HI;
    v.override_text_color = Some(TEXT);
    v.selection.bg_fill = Color32::from_rgb(0x1E, 0x4A, 0x52);
    v.selection.stroke = Stroke::new(1.0_f32, ACCENT);
    v.widgets.noninteractive.bg_fill = PANEL;
    v.widgets.noninteractive.bg_stroke = Stroke::new(1.0_f32, LINE);
    v.widgets.inactive.bg_fill = PANEL_HI;
    v.widgets.inactive.bg_stroke = Stroke::new(1.0_f32, LINE);
    v.widgets.inactive.fg_stroke = Stroke::new(1.0_f32, TEXT);
    v.widgets.hovered.bg_fill = Color32::from_rgb(0x2B, 0x32, 0x3C);
    v.widgets.hovered.bg_stroke = Stroke::new(1.0_f32, ACCENT);
    v.widgets.active.bg_fill = Color32::from_rgb(0x1E, 0x4A, 0x52);
    v.widgets.active.bg_stroke = Stroke::new(1.0_f32, ACCENT);
    v.window_stroke = Stroke::new(1.0_f32, LINE);

    // 用 all_styles_mut 一次写入两套主题：
    // 只调 set_visuals 会被随后 set_style 的旧 style 覆盖（TextEdit 底仍是浅色）。
    ctx.all_styles_mut(|style| {
        style.visuals = v.clone();
        style.spacing.item_spacing = egui::vec2(8.0, 6.0);
        style.spacing.button_padding = egui::vec2(10.0, 4.0);
        style.text_styles.insert(
            TextStyle::Heading,
            FontId::new(20.0, FontFamily::Proportional),
        );
        style
            .text_styles
            .insert(TextStyle::Body, FontId::new(14.0, FontFamily::Proportional));
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
            FontId::new(13.5, FontFamily::Monospace),
        );
    });
}
