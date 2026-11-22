//! 建号页：`ready.hasChar == false` 时进入。

use egui::{Align, Layout, RichText};

use crate::ui::{Intent, UiState, theme};

pub fn show(ctx: &egui::Context, st: &mut UiState, out: &mut Vec<Intent>) {
    egui::CentralPanel::default().show(ctx, |ui| {
        ui.add_space(80.0);
        ui.vertical_centered(|ui| {
            ui.label(RichText::new("踏入凡界").size(26.0).color(theme::ACCENT));
            ui.add_space(6.0);
            ui.label(
                RichText::new("先取一个道号。往后同门、仇敌与史书都用它称呼你。").color(theme::DIM),
            );
            ui.add_space(24.0);
        });

        ui.allocate_ui_with_layout(
            egui::vec2(360.0, 220.0),
            Layout::top_down(Align::Min),
            |ui| {
                ui.label(RichText::new("道号").small().color(theme::DIM));
                let name = ui.add(
                    egui::TextEdit::singleline(&mut st.create_name)
                        .hint_text("2–8 个字符，中文或字母")
                        .desired_width(f32::INFINITY),
                );
                ui.add_space(6.0);
                ui.label(
                    RichText::new("建号后灵根随机，凡界从青云山脚开始。")
                        .small()
                        .color(theme::DIM),
                );
                ui.add_space(14.0);

                let len = st.create_name.trim().chars().count();
                let ok = (2..=8).contains(&len);
                let btn = ui.add_enabled(
                    ok,
                    egui::Button::new(RichText::new("创建角色").size(15.0))
                        .min_size(egui::vec2(ui.available_width(), 34.0)),
                );
                let enter = name.lost_focus() && ui.input(|i| i.key_pressed(egui::Key::Enter));
                if (btn.clicked() || enter) && ok {
                    out.push(Intent::Create(st.create_name.trim().to_string()));
                }
                if !st.create_name.is_empty() && !ok {
                    ui.add_space(6.0);
                    ui.label(RichText::new("道号长度需在 2–8 个字符之间").color(theme::WARN));
                }
            },
        );
    });
}
