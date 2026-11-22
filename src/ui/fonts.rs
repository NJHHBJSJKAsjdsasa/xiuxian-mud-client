//! 字体：正文用无衬线，标题/场景名用衬线（楷或宋）。
//!
//! 中文必须显式装载：egui 默认字体不含 CJK，缺字会显示方块。
//! 优先仓库内置子集（`assets/fonts/`），没有就读系统字体。

use std::sync::Arc;

use crate::ui::theme;

/// 返回装载结果描述，设置页会显示。
pub fn install(ctx: &egui::Context) -> String {
    let mut fonts = egui::FontDefinitions::default();

    let body = pick(&[
        "assets/fonts/NotoSansSC-Regular.subset.otf",
        "assets/fonts/NotoSansSC-Regular.otf",
        r"C:\Windows\Fonts\msyh.ttc",
        r"C:\Windows\Fonts\msyhbd.ttc",
        r"C:\Windows\Fonts\simhei.ttf",
        r"C:\Windows\Fonts\simsun.ttc",
        "/System/Library/Fonts/PingFang.ttc",
        "/System/Library/Fonts/STHeiti Light.ttc",
        "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
        "/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
    ]);

    let serif = pick(&[
        "assets/fonts/NotoSerifSC-Regular.subset.otf",
        r"C:\Windows\Fonts\simkai.ttf",
        r"C:\Windows\Fonts\simsun.ttc",
        r"C:\Windows\Fonts\msyh.ttc",
        "/System/Library/Fonts/Supplemental/Songti.ttc",
        "/System/Library/Fonts/PingFang.ttc",
        "/usr/share/fonts/opentype/noto/NotoSerifCJK-Regular.ttc",
        "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
    ]);

    let Some(body) = body else {
        tracing::warn!("没有找到中文字体，中文将显示为方块；请把字体子集放进 assets/fonts/");
        return "未装载（缺字）".into();
    };
    fonts.font_data.insert(
        "cjk".to_owned(),
        Arc::new(egui::FontData::from_owned(body.1)),
    );
    for family in [egui::FontFamily::Proportional, egui::FontFamily::Monospace] {
        if let Some(list) = fonts.families.get_mut(&family) {
            // 放最前：中文优先用这个字体，拉丁与符号仍可由后面的默认字体补。
            list.insert(0, "cjk".to_owned());
        }
    }

    let serif_desc = if let Some((path, bytes)) = serif {
        fonts.font_data.insert(
            "serif_cjk".to_owned(),
            Arc::new(egui::FontData::from_owned(bytes)),
        );
        fonts.families.insert(
            theme::serif(),
            vec!["serif_cjk".to_owned(), "cjk".to_owned()],
        );
        short(path)
    } else {
        // 没有衬线就退回正文，至少不缺字。
        fonts
            .families
            .insert(theme::serif(), vec!["cjk".to_owned()]);
        "同正文".to_owned()
    };

    ctx.set_fonts(fonts);
    format!("正文 {} · 标题 {}", short(body.0), serif_desc)
}

fn pick(candidates: &[&'static str]) -> Option<(&'static str, Vec<u8>)> {
    for path in candidates {
        if let Ok(bytes) = std::fs::read(path) {
            return Some((path, bytes));
        }
    }
    None
}

fn short(path: &str) -> String {
    path.rsplit(['/', '\\']).next().unwrap_or(path).to_string()
}
