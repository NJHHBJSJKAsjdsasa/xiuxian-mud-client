//! 致命错误页：版本不兼容、渲染后端失败等，只留退出。

use egui::{Align, Layout, RichText};

use crate::ui::{Intent, UiState, theme};

pub fn show(ctx: &egui::Context, st: &mut UiState, out: &mut Vec<Intent>) {
    egui::CentralPanel::default().show(ctx, |ui| {
        ui.add_space(100.0);
        ui.vertical_centered(|ui| {
            ui.label(RichText::new("无法进入游戏").size(26.0).color(theme::BAD));
            ui.add_space(10.0);
            ui.label(RichText::new(&st.fatal).color(theme::TEXT));
            ui.add_space(6.0);
            ui.label(
                RichText::new(format!("本机协议版本 v{}", crate::protocol::PROTO))
                    .small()
                    .color(theme::DIM),
            );
            ui.add_space(24.0);
        });

        ui.allocate_ui_with_layout(
            egui::vec2(360.0, 120.0),
            Layout::top_down(Align::Min),
            |ui| {
                if ui
                    .add(
                        egui::Button::new("复制诊断信息")
                            .min_size(egui::vec2(ui.available_width(), 30.0)),
                    )
                    .clicked()
                {
                    let text = format!(
                        "客户端协议 v{}\n错误：{}\n字体：{}\n平台：{} {}",
                        crate::protocol::PROTO,
                        st.fatal,
                        st.font_source,
                        std::env::consts::OS,
                        std::env::consts::ARCH
                    );
                    ui.ctx().copy_text(text);
                    st.toast("诊断信息已复制", false);
                }
                ui.add_space(8.0);
                if ui
                    .add(egui::Button::new("退出").min_size(egui::vec2(ui.available_width(), 30.0)))
                    .clicked()
                {
                    out.push(Intent::Quit);
                }
            },
        );
    });
}
