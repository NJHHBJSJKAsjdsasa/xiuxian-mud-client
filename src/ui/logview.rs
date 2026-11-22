//! 日志区：虚拟滚动 + 自动贴底 + 按类型上色。
//!
//! 只对可见行建控件：5000 行也不会卡。

use egui::{Align, Layout, RichText, ScrollArea, TextWrapMode};

use crate::store::AppState;
use crate::ui::{UiState, theme};
use crate::util::text::truncate_chars;
use crate::util::time;

const ROW_H: f32 = 20.0;

pub fn show(ui: &mut egui::Ui, st: &mut UiState, app: &AppState) {
    ui.horizontal(|ui| {
        ui.label(RichText::new("日志").small().color(theme::DIM));
        ui.label(
            RichText::new(format!("{} 行", app.logs.len()))
                .small()
                .color(theme::DIM),
        );
        if app.logs.dropped() > 0 {
            ui.label(
                RichText::new(format!("（已省略 {}）", app.logs.dropped()))
                    .small()
                    .color(theme::DIM),
            );
        }
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            if !st.log_stick && ui.small_button("↓ 回到底部").clicked() {
                st.log_stick = true;
            }
            if ui
                .small_button(if st.show_timestamps {
                    "隐藏时间"
                } else {
                    "显示时间"
                })
                .clicked()
            {
                st.show_timestamps = !st.show_timestamps;
            }
        });
    });
    ui.separator();

    let n = app.logs.len();
    if n == 0 {
        ui.add_space(24.0);
        ui.vertical_centered(|ui| {
            ui.label(RichText::new("还没有消息。左侧选一条路走走看。").color(theme::DIM));
        });
        return;
    }

    let out = ScrollArea::vertical()
        .id_salt("log_scroll")
        .auto_shrink([false, false])
        .stick_to_bottom(st.log_stick)
        .show_rows(ui, ROW_H, n, |ui, range| {
            ui.spacing_mut().item_spacing.y = 0.0;
            ui.style_mut().wrap_mode = Some(TextWrapMode::Truncate);
            for i in range {
                let Some(e) = app.logs.get(i) else { continue };
                ui.horizontal(|ui| {
                    if st.show_timestamps {
                        ui.label(
                            RichText::new(time::fmt_hms(e.ts))
                                .small()
                                .monospace()
                                .color(theme::DIM),
                        );
                    }
                    // `lvl` 加粗，其余常规。
                    let mut rt =
                        RichText::new(truncate_chars(&e.line.s, 300)).color(theme::tone(&e.line.k));
                    if e.line.k == "lvl" {
                        rt = rt.strong();
                    }
                    ui.add(egui::Label::new(rt).truncate())
                        .on_hover_text(&e.line.s);
                });
            }
        });

    // 用户在中栏里向上滚 → 暂停贴底，避免新行抢位置。
    let hovered = ui
        .ctx()
        .pointer_hover_pos()
        .is_some_and(|p| out.inner_rect.contains(p));
    if hovered {
        let dy = ui.ctx().input(|i| i.raw_scroll_delta.y);
        if dy > 0.5 {
            st.log_stick = false;
        }
        if ui.ctx().input(|i| i.key_pressed(egui::Key::End)) {
            st.log_stick = true;
        }
    }
}
