//! 字体：优先内置子集（`assets/fonts/`），否则回退系统字体。
//!
//! 中文必须显式装载：egui 默认字体不含 CJK，缺字会显示方块。

use std::sync::Arc;

/// 返回实际装载的字体来源描述，便于设置页显示。
pub fn install(ctx: &egui::Context) -> String {
    let mut fonts = egui::FontDefinitions::default();
    let mut loaded: Option<(String, Vec<u8>)> = None;

    // 1) 仓库内置子集（如果放了文件）
    for candidate in [
        "assets/fonts/NotoSansSC-Regular.subset.otf",
        "assets/fonts/NotoSansSC-Regular.otf",
    ] {
        if let Ok(bytes) = std::fs::read(candidate) {
            loaded = Some((format!("内置 {}", short(candidate)), bytes));
            break;
        }
    }

    // 2) 系统字体
    if loaded.is_none() {
        for candidate in system_candidates() {
            if let Ok(bytes) = std::fs::read(candidate) {
                loaded = Some((format!("系统 {}", short(candidate)), bytes));
                break;
            }
        }
    }

    let Some((desc, bytes)) = loaded else {
        tracing::warn!(
            "没有找到中文字体，中文将显示为方块；请把 NotoSansSC 子集放进 assets/fonts/"
        );
        return "未装载（缺字）".into();
    };

    fonts.font_data.insert(
        "cjk".to_owned(),
        Arc::new(egui::FontData::from_owned(bytes)),
    );
    for family in [egui::FontFamily::Proportional, egui::FontFamily::Monospace] {
        if let Some(list) = fonts.families.get_mut(&family) {
            // 放最前：中文优先用这个字体，拉丁与符号仍可由后面的默认字体补。
            list.insert(0, "cjk".to_owned());
        }
    }
    ctx.set_fonts(fonts);
    desc
}

fn short(path: &str) -> String {
    path.rsplit(['/', '\\']).next().unwrap_or(path).to_string()
}

fn system_candidates() -> Vec<&'static str> {
    let mut v = Vec::new();
    if cfg!(windows) {
        v.extend([
            r"C:\Windows\Fonts\msyh.ttc",
            r"C:\Windows\Fonts\msyhbd.ttc",
            r"C:\Windows\Fonts\simhei.ttf",
            r"C:\Windows\Fonts\simsun.ttc",
        ]);
    } else if cfg!(target_os = "macos") {
        v.extend([
            "/System/Library/Fonts/PingFang.ttc",
            "/System/Library/Fonts/STHeiti Light.ttc",
            "/Library/Fonts/Arial Unicode.ttf",
        ]);
    } else {
        v.extend([
            "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
            "/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
            "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
        ]);
    }
    v
}
