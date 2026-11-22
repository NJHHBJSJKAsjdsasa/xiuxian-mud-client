//! 见闻区：虚拟滚动 + 自动贴底 + 按类型上色。
//!
//! 只对可见行建控件：5000 行也不会卡。旁白用衬线，读起来像书页。

use egui::{Align2, RichText, ScrollArea, Sense, TextWrapMode, vec2};

use crate::store::AppState;
use crate::ui::{UiState, theme};
use crate::util::text::truncate_chars;
use crate::util::time;

const ROW_H: f32 = 22.0;

pub fn show(ui: &mut egui::Ui, st: &mut UiState, app: &AppState) {
    let n = app.logs.len();
    if n == 0 {
        ui.add_space(20.0);
        ui.vertical_centered(|ui| {
            ui.label(
                RichText::new("还没有发生什么。往任意一条路走走看。")
                    .size(13.0)
                    .color(theme::FAINT),
            );
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
                let tone = theme::tone(&e.line.k);
                ui.horizontal(|ui| {
                    // 行首色点：比整行上色更克制，也更容易一眼分类型。
                    let (dot, _) = ui.allocate_exact_size(vec2(10.0, ROW_H), Sense::hover());
                    ui.painter().circle_filled(dot.center(), 2.2, tone);

                    if st.show_timestamps {
                        ui.allocate_ui_with_layout(
                            vec2(52.0, ROW_H),
                            egui::Layout::left_to_right(egui::Align::Center),
                            |ui| {
                                ui.label(
                                    RichText::new(time::fmt_hms(e.ts))
                                        .size(10.5)
                                        .monospace()
                                        .color(theme::FAINT),
                                );
                            },
                        );
                    }

                    let text = truncate_chars(&e.line.s, 300);
                    let mut rt = RichText::new(text).size(14.0).color(tone);
                    if e.line.k == "nar" {
                        rt = RichText::new(truncate_chars(&e.line.s, 300))
                            .font(theme::serif_id(15.0))
                            .color(tone);
                    } else if e.line.k == "lvl" {
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
    let _ = Align2::LEFT_TOP;
}
