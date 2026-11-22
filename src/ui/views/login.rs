//! 登录页：服务器地址、道号、密码、登录/注册。

use egui::{Align, Layout, RichText};

use crate::ui::{Intent, UiState, View, theme};

pub fn show(ctx: &egui::Context, st: &mut UiState, out: &mut Vec<Intent>) {
    egui::CentralPanel::default().show(ctx, |ui| {
        ui.add_space(56.0);
        ui.vertical_centered(|ui| {
            ui.label(
                RichText::new("九州问道")
                    .size(34.0)
                    .color(theme::ACCENT)
                    .strong(),
            );
            ui.add_space(4.0);
            ui.label(RichText::new("云端存档 · 服务端权威").color(theme::DIM));
            ui.add_space(28.0);
        });

        ui.horizontal(|ui| {
            let form_w = 380.0;
            ui.add_space(((ui.available_width() - form_w) / 2.0).max(0.0));
            ui.allocate_ui_with_layout(
                egui::vec2(form_w, 400.0),
                Layout::top_down(Align::Min),
                |ui| {
                    ui.horizontal(|ui| {
                        if ui.selectable_label(!st.login.register, "登录").clicked() {
                            st.login.register = false;
                            st.login.error.clear();
                        }
                        if ui.selectable_label(st.login.register, "注册").clicked() {
                            st.login.register = true;
                            st.login.error.clear();
                        }
                        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                            ui.label(RichText::new("协议 v1").small().color(theme::DIM));
                        });
                    });
                    ui.add_space(8.0);

                    ui.label(RichText::new("服务器").small().color(theme::DIM));
                    ui.add(
                        egui::TextEdit::singleline(&mut st.login.server)
                            .hint_text("ws://127.0.0.1:8080")
                            .desired_width(f32::INFINITY),
                    );
                    ui.add_space(6.0);

                    ui.label(RichText::new("道号").small().color(theme::DIM));
                    ui.add(
                        egui::TextEdit::singleline(&mut st.login.name)
                            .hint_text("2–8 个字符")
                            .desired_width(f32::INFINITY),
                    );
                    ui.add_space(6.0);

                    ui.label(RichText::new("密码").small().color(theme::DIM));
                    let pass = ui.add(
                        egui::TextEdit::singleline(&mut st.login.pass)
                            .password(true)
                            .hint_text("6–32 个字符")
                            .desired_width(f32::INFINITY),
                    );
                    ui.add_space(14.0);

                    let can_submit = !st.login.loading
                        && !st.login.name.trim().is_empty()
                        && st.login.pass.len() >= 6;
                    let label = if st.login.loading {
                        "连接中…"
                    } else if st.login.register {
                        "注册并进入"
                    } else {
                        "登录"
                    };
                    let btn = ui.add_enabled(
                        can_submit,
                        egui::Button::new(RichText::new(label).size(15.0))
                            .min_size(egui::vec2(ui.available_width(), 34.0)),
                    );
                    let enter = pass.lost_focus() && ui.input(|i| i.key_pressed(egui::Key::Enter));
                    if (btn.clicked() || enter) && can_submit {
                        st.login.loading = true;
                        st.login.error.clear();
                        out.push(Intent::Login {
                            register: st.login.register,
                        });
                    }

                    if !st.login.error.is_empty() {
                        ui.add_space(10.0);
                        ui.label(RichText::new(&st.login.error).color(theme::BAD));
                    }

                    ui.add_space(18.0);
                    ui.separator();
                    ui.add_space(6.0);
                    ui.label(
                        RichText::new("服务端地址也可以写在 config.toml，命令行 --server 优先。")
                            .small()
                            .color(theme::DIM),
                    );
                    if !st.font_source.is_empty() {
                        ui.label(
                            RichText::new(format!("字体：{}", st.font_source))
                                .small()
                                .color(theme::DIM),
                        );
                    }
                },
            );
        });
    });

    // 致命错误后返回登录页时，清掉 loading。
    if st.view == View::Login && st.login.loading {
        // 由 app 在收到 ready / 错误时清；此处不做超时兜底（app 有 8 秒超时事件）。
    }
}
