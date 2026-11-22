//! 终端配色与字形：全项目唯一色源。

use ratatui::style::Color;

pub const BG: Color = Color::Rgb(5, 7, 9);
pub const PANEL: Color = Color::Rgb(10, 13, 16);
pub const TEXT: Color = Color::Rgb(226, 226, 214);
pub const DIM: Color = Color::Rgb(120, 130, 138);
pub const FAINT: Color = Color::Rgb(70, 78, 86);
pub const ACCENT: Color = Color::Rgb(127, 209, 193);
pub const GOLD: Color = Color::Rgb(217, 179, 106);
pub const BAD: Color = Color::Rgb(196, 85, 61);
pub const GOOD: Color = Color::Rgb(111, 191, 115);
pub const WARN: Color = Color::Rgb(201, 162, 39);
pub const PVP: Color = Color::Rgb(169, 139, 200);

/// 日志类型 → 颜色。
pub fn tone(k: &str) -> Color {
    match k {
        "sys" => DIM,
        "nar" => Color::Rgb(201, 184, 150),
        "combat" => TEXT,
        "good" => GOOD,
        "bad" => BAD,
        "lvl" => GOLD,
        "chat" => ACCENT,
        "pvp" => PVP,
        _ => TEXT,
    }
}

/// 8 向箭头：服务端 y 向北为正，屏幕向上。
pub fn bearing_arrow(dx: i32, dy: i32) -> &'static str {
    if dx == 0 && dy == 0 {
        return "·";
    }
    const ARROWS: [&str; 8] = ["→", "↗", "↑", "↖", "←", "↙", "↓", "↘"];
    let ang = (dy as f32).atan2(dx as f32);
    let idx = (ang / (std::f32::consts::PI / 4.0)).round() as i32;
    ARROWS[idx.rem_euclid(8) as usize]
}

/// 进度条字符：`[####----]`。
pub fn bar(cur: i64, max: i64, cells: usize) -> String {
    let ratio = if max <= 0 {
        0.0
    } else {
        (cur as f32 / max as f32).clamp(0.0, 1.0)
    };
    let filled = (ratio * cells as f32).round() as usize;
    format!(
        "[{}{}]",
        "#".repeat(filled),
        "-".repeat(cells.saturating_sub(filled))
    )
}
