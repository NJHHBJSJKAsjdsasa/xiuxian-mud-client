//! 主界面：全屏场景 + 叠加信息。
//!
//! 布局范式：场景铺满整个窗口当底，UI 浮在上面——
//! 左上角是"现在该做什么"，底部是路标、旁白与行动键。
//! 不做三栏后台式布局：面板越少，越像游戏。

use egui::{Align, Align2, Color32, Layout, Rect, RichText, Sense, Stroke, UiBuilder, pos2, vec2};

use crate::net::ConnStatus;
use crate::store::AppState;
use crate::ui::{self, Intent, Page, UiState, scene, theme};
use crate::util::text::{percent, thousands};

pub fn show(ctx: &egui::Context, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    top_bar(ctx, st, app);
    egui::CentralPanel::default()
        .frame(
            egui::Frame::new()
                .fill(theme::BG)
                .inner_margin(egui::Margin::ZERO),
        )
        .show(ctx, |ui| match st.page {
            Page::Game => game_screen(ui, st, app, out),
            other => page_screen(ui, st, app, out, other),
        });
}

// ───────────────────────── 顶栏 ─────────────────────────

fn top_bar(ctx: &egui::Context, st: &mut UiState, app: &AppState) {
    egui::TopBottomPanel::top("top")
        .exact_height(48.0)
        .frame(
            egui::Frame::new()
                .fill(theme::PANEL)
                .inner_margin(egui::Margin::symmetric(16, 0)),
        )
        .show(ctx, |ui| {
            ui.horizontal_centered(|ui| {
                let (place, region) = match app.world.get() {
                    Some(m) => (m.name.clone(), m.region_name.clone()),
                    None => ("雾中".to_string(), String::new()),
                };
                ui.label(
                    RichText::new(place)
                        .font(theme::serif_id(18.0))
                        .color(theme::TEXT),
                );
                if !region.is_empty() {
                    ui.label(RichText::new(region).size(11.0).color(theme::FAINT));
                }

                ui.add_space(16.0);
                if let Some(m) = app.world.get() {
                    let season = season_name(m.season.as_deref().unwrap_or(""));
                    let weather = weather_name(m.weather.as_deref().unwrap_or(""));
                    let bits: Vec<&str> = [season, weather]
                        .into_iter()
                        .filter(|s| !s.is_empty())
                        .collect();
                    if !bits.is_empty() {
                        ui.label(RichText::new(bits.join(" · ")).size(12.0).color(theme::DIM));
                    }
                }
                let (color, text) = conn_badge(&app.conn);
                ui.label(RichText::new("●").size(10.0).color(color));
                ui.label(RichText::new(text).size(11.5).color(color));
                ui.label(
                    RichText::new(format!("在线 {}", app.presence))
                        .size(11.0)
                        .color(theme::FAINT),
                );

                ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                    // 古书签式的单字入口：字形比图标字体稳，也不吃跨平台差异。
                    for (glyph, page, hint) in [
                        ("设", Page::Settings, "设置 · F8"),
                        ("宗", Page::Sect, "宗门 · F6"),
                        ("交", Page::Market, "坊市 · F5"),
                        ("技", Page::Skills, "技能 · F3"),
                        ("囊", Page::Bag, "背包 · F2"),
                        ("人", Page::Character, "角色 · F1"),
                        ("江", Page::Social, "江湖 · F4"),
                        ("舆", Page::Map, "舆图 · F7"),
                    ] {
                        let active = st.page == page;
                        if ui
                            .add(
                                egui::Button::new(
                                    RichText::new(glyph)
                                        .font(theme::serif_id(15.0))
                                        .color(if active { theme::ACCENT } else { theme::DIM }),
                                )
                                .frame(false)
                                .min_size(vec2(26.0, 26.0)),
                            )
                            .on_hover_text(hint)
                            .clicked()
                        {
                            st.page = page;
                        }
                    }
                    ui.add_space(10.0);
                    ui.separator();
                    ui.add_space(6.0);
                    if let Some(p) = app.player.get() {
                        ui.label(
                            RichText::new(thousands(p.stones))
                                .size(13.0)
                                .color(theme::GOLD),
                        );
                        ui.label(RichText::new("灵石").size(10.0).color(theme::FAINT));
                        ui.add_space(8.0);
                        ui.label(RichText::new(&p.realm).size(11.5).color(theme::ACCENT));
                        ui.label(
                            RichText::new(&p.name)
                                .font(theme::serif_id(15.0))
                                .color(theme::TEXT),
                        );
                    }
                });
            });
        });
}

fn conn_badge(c: &ConnStatus) -> (Color32, String) {
    match c {
        ConnStatus::Online => (theme::GOOD, "在线".into()),
        ConnStatus::Connecting => (theme::WARN, "连接中".into()),
        ConnStatus::LoggingIn => (theme::WARN, "登录中".into()),
        ConnStatus::Backoff { secs, .. } => (theme::WARN, format!("重连 {secs}s")),
        ConnStatus::Idle => (theme::FAINT, "未连接".into()),
        ConnStatus::Offline(msg) => (theme::BAD, msg.clone()),
        ConnStatus::Fatal(msg) => (theme::BAD, msg.clone()),
    }
}

// ───────────────────────── 主画面 ─────────────────────────

fn game_screen(ui: &mut egui::Ui, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    let rect = ui.max_rect();
    let Some(m) = app.world.get() else {
        ui.centered_and_justified(|ui| {
            ui.label(
                RichText::new("雾还没散，看不清脚下。")
                    .font(theme::serif_id(18.0))
                    .color(theme::DIM),
            );
        });
        return;
    };

    // 场景占据上部，底部留给路标/旁白/行动。
    let bottom_h = 236.0;
    let scene_rect = Rect::from_min_max(
        rect.min,
        pos2(
            rect.right(),
            (rect.bottom() - bottom_h).max(rect.top() + 220.0),
        ),
    );
    let pose = if app.vitals.cultivating {
        scene::Pose::Sitting
    } else if app.player.get().and_then(|p| p.travel.as_ref()).is_some() {
        scene::Pose::Walking
    } else {
        scene::Pose::Standing
    };
    scene::paint(
        ui,
        scene_rect,
        &scene::Scene {
            name: &m.name,
            region: &m.region_name,
            terrain: &m.terrain,
            weather: m.weather.as_deref().unwrap_or(""),
            daypart: m.daypart.as_deref().unwrap_or(""),
            grade: m.grade.as_deref(),
            desc: &m.desc,
            explored: m.explored,
        },
        pose,
    );

    // 场景到面板的过渡：一条由透明到面板色的渐变，避免硬接缝。
    let seam = Rect::from_min_max(
        pos2(scene_rect.left(), scene_rect.bottom() - 46.0),
        pos2(scene_rect.right(), scene_rect.bottom()),
    );
    for i in 0..10 {
        let seg = Rect::from_min_max(
            pos2(seam.left(), seam.top() + seam.height() * (i as f32 / 10.0)),
            pos2(
                seam.right(),
                seam.top() + seam.height() * ((i + 1) as f32 / 10.0) + 1.0,
            ),
        );
        let t = (i as f32 + 1.0) / 10.0;
        ui.painter().rect_filled(
            seg,
            0,
            Color32::from_rgba_unmultiplied(0x0A, 0x0C, 0x0E, (t * 230.0) as u8),
        );
    }

    // 左上：当前该做什么
    objective_card(ui, app);

    // 底部三段：路标 → 旁白 → 行动
    let exits_rect = Rect::from_min_max(
        pos2(rect.left() + 16.0, rect.bottom() - 230.0),
        pos2(rect.right() - 16.0, rect.bottom() - 196.0),
    );
    ui.scope_builder(UiBuilder::new().max_rect(exits_rect), |ui| {
        exits_bar(ui, st, app, m, out);
    });

    let log_rect = Rect::from_min_max(
        pos2(rect.left() + 16.0, rect.bottom() - 190.0),
        pos2(rect.right() - 16.0, rect.bottom() - 96.0),
    );
    ui.scope_builder(UiBuilder::new().max_rect(log_rect), |ui| {
        dialogue_box(ui, st, app, out);
    });

    let act_rect = Rect::from_min_max(
        pos2(rect.left() + 16.0, rect.bottom() - 92.0),
        pos2(rect.right() - 16.0, rect.bottom() - 12.0),
    );
    ui.scope_builder(UiBuilder::new().max_rect(act_rect), |ui| {
        action_row(ui, app, out);
    });
}

/// 目标卡：玩家每次看屏幕都该知道"下一步干什么"。
fn objective_card(ui: &mut egui::Ui, app: &AppState) {
    let (title, line) = objective(app);
    let rect = Rect::from_min_size(
        pos2(ui.max_rect().left() + 20.0, ui.max_rect().top() + 122.0),
        vec2(248.0, 68.0),
    );
    let p = ui.painter();
    p.rect_filled(
        rect,
        5,
        Color32::from_rgba_unmultiplied(0x06, 0x09, 0x0B, 190),
    );
    p.rect_stroke(
        rect,
        5,
        Stroke::new(1.0_f32, theme::GOLD.linear_multiply(0.5)),
        egui::StrokeKind::Inside,
    );
    p.text(
        rect.left_top() + vec2(12.0, 10.0),
        Align2::LEFT_TOP,
        title,
        egui::FontId::proportional(10.5),
        theme::GOLD,
    );
    p.text(
        rect.left_top() + vec2(12.0, 26.0),
        Align2::LEFT_TOP,
        crate::util::text::truncate_chars(&line, 22),
        theme::serif_id(15.0),
        theme::TEXT,
    );
}

fn objective(app: &AppState) -> (String, String) {
    if let Some(t) = app.player.get().and_then(|p| p.travel.as_ref()) {
        return (
            "赶路中".into(),
            format!(
                "前往 {} · 剩 {}",
                t.name,
                crate::util::time::fmt_countdown(t.arrive_at - app.server_now())
            ),
        );
    }
    if app.vitals.cultivating {
        return ("打坐中".into(), "真气在慢慢回，按 C 起身".into());
    }
    if app.vitals.stamina < 30 {
        return ("体力不济".into(), "坐下调息，或回城歇脚".into());
    }
    let explored = app.world.get().map(|m| m.explored).unwrap_or(0);
    if explored < 3 {
        return ("摸清这条路".into(), "点下面的路标四处走走".into());
    }
    ("寻个去处".into(), "去没走过的地方看看".into())
}

/// 底部路标：方向 + 地名 + 耗时，点一下就走。
fn exits_bar(
    ui: &mut egui::Ui,
    st: &mut UiState,
    app: &AppState,
    m: &crate::protocol::msg::MapView,
    out: &mut Vec<Intent>,
) {
    let traveling = app
        .player
        .get()
        .map(|p| p.travel.is_some())
        .unwrap_or(false);

    egui::Frame::new()
        .fill(Color32::from_rgba_unmultiplied(0x0B, 0x0E, 0x11, 205))
        .stroke(Stroke::new(1.0_f32, theme::LINE.linear_multiply(0.8)))
        .inner_margin(egui::Margin::symmetric(8, 3))
        .corner_radius(6)
        .show(ui, |ui| {
            ui.set_width(ui.available_width());
            egui::ScrollArea::horizontal()
                .id_salt("exits_bar")
                .scroll_bar_visibility(egui::scroll_area::ScrollBarVisibility::AlwaysHidden)
                .show(ui, |ui| {
                    ui.horizontal(|ui| {
                        ui.label(RichText::new("路").size(11.0).color(theme::GOLD));
                        for (i, ex) in m.exits.iter().enumerate() {
                            let name = ex
                                .name
                                .clone()
                                .filter(|s| !s.is_empty())
                                .unwrap_or_else(|| "雾中".into());
                            let blocked = ex.blocked == Some(true);
                            let ok = ex.ok && !blocked && !traveling;
                            let arrow = match (m.x, m.y, ex.x, ex.y) {
                                (Some(cx), Some(cy), Some(tx), Some(ty)) => {
                                    bearing_arrow(tx - cx, ty - cy)
                                }
                                _ => "·",
                            };
                            let label = format!(
                                "{} {}  {}s",
                                arrow,
                                crate::util::text::truncate_chars(&name, 5),
                                ex.cost_sec
                            );
                            let text = RichText::new(label).size(12.5).color(if ok {
                                theme::TEXT
                            } else {
                                theme::FAINT
                            });
                            let btn = egui::Button::new(text)
                                .fill(theme::CARD)
                                .stroke(Stroke::new(
                                    1.0_f32,
                                    if ok {
                                        theme::LINE
                                    } else {
                                        theme::LINE.linear_multiply(0.5)
                                    },
                                ))
                                .min_size(vec2(0.0, 26.0));
                            let resp = ui.add_enabled(ok, btn);
                            if resp.clicked() {
                                st.selected_exit = i;
                                out.push(Intent::Move(ex.to.clone()));
                            }
                            resp.on_hover_text(if ok {
                                format!(
                                    "前往 {}（{}）· 约 {} 秒",
                                    name,
                                    terrain_name(&ex.terrain),
                                    ex.cost_sec
                                )
                            } else if traveling {
                                "正在赶路".to_string()
                            } else if blocked {
                                "断路".to_string()
                            } else {
                                format!(
                                    "走不了：{}",
                                    ex.cond.as_deref().map(cond_text).unwrap_or_default()
                                )
                            });
                        }
                    });
                });
        });
}

/// 旁白框：最近几条消息 + 一句话输入，像 AVG 的文本框。
fn dialogue_box(ui: &mut egui::Ui, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    egui::Frame::new()
        .fill(Color32::from_rgba_unmultiplied(0x07, 0x09, 0x0B, 214))
        .stroke(Stroke::new(1.0_f32, theme::LINE.linear_multiply(0.9)))
        .inner_margin(egui::Margin::symmetric(12, 6))
        .corner_radius(6)
        .show(ui, |ui| {
            ui.set_width(ui.available_width());
            let n = app.logs.len();
            if n == 0 {
                ui.label(
                    RichText::new("四下无声。往任一条路走走看。")
                        .font(theme::serif_id(15.0))
                        .color(theme::DIM),
                );
            } else {
                egui::ScrollArea::vertical()
                    .id_salt("dialogue_scroll")
                    .max_height(62.0)
                    .auto_shrink([false, false])
                    .stick_to_bottom(st.chat_stick)
                    .show(ui, |ui| {
                        let start = n.saturating_sub(40);
                        for i in start..n {
                            let Some(e) = app.logs.get(i) else { continue };
                            ui.horizontal(|ui| {
                                let (dot, _) =
                                    ui.allocate_exact_size(vec2(8.0, 15.0), Sense::hover());
                                ui.painter().circle_filled(
                                    dot.center(),
                                    2.0,
                                    theme::tone(&e.line.k),
                                );
                                let mut rt = RichText::new(crate::util::text::truncate_chars(
                                    &e.line.s, 150,
                                ))
                                .size(14.0)
                                .color(theme::tone(&e.line.k));
                                if e.line.k == "nar" {
                                    rt = rt.font(theme::serif_id(15.0));
                                }
                                ui.label(rt);
                            });
                        }
                    });
            }

            ui.add_space(1.0);
            ui.horizontal(|ui| {
                let ch = match st.chat_ch.as_str() {
                    "region" => "区域",
                    "party" => "队伍",
                    "dm" => "私语",
                    _ => "世界",
                };
                egui::ComboBox::from_id_salt("chat_ch")
                    .selected_text(RichText::new(ch).size(11.0).color(theme::FAINT))
                    .width(54.0)
                    .show_ui(ui, |ui| {
                        ui.selectable_value(&mut st.chat_ch, "world".into(), "世界");
                        ui.selectable_value(&mut st.chat_ch, "region".into(), "区域");
                        ui.selectable_value(&mut st.chat_ch, "party".into(), "队伍");
                        ui.selectable_value(&mut st.chat_ch, "dm".into(), "私语");
                    });
                if st.chat_ch == "dm" {
                    ui.add(
                        egui::TextEdit::singleline(&mut st.chat_to)
                            .hint_text("道号")
                            .desired_width(70.0),
                    );
                }
                let online = matches!(app.conn, ConnStatus::Online);
                let w = ui.available_width();
                let resp = ui.add(
                    egui::TextEdit::singleline(&mut st.chat_text)
                        .desired_width(w)
                        .hint_text(if online {
                            "说点什么…（Enter 送出）"
                        } else {
                            "未连接"
                        })
                        .char_limit(80)
                        .frame(false),
                );
                let r = resp.rect;
                ui.painter().line_segment(
                    [
                        pos2(r.left(), r.bottom() + 1.0),
                        pos2(r.right(), r.bottom() + 1.0),
                    ],
                    Stroke::new(
                        1.0_f32,
                        if resp.has_focus() {
                            theme::ACCENT
                        } else {
                            theme::LINE.linear_multiply(0.8)
                        },
                    ),
                );
                let enter = resp.lost_focus() && ui.input(|i| i.key_pressed(egui::Key::Enter));
                if enter && online {
                    let text = st.chat_text.trim().to_string();
                    if !text.is_empty() {
                        out.push(Intent::Chat {
                            ch: st.chat_ch.clone(),
                            to: if st.chat_ch == "dm" {
                                Some(st.chat_to.trim().to_string())
                            } else {
                                None
                            },
                            text,
                        });
                        st.chat_text.clear();
                        st.chat_stick = true;
                    }
                }
            });
        });
}

/// 行动行：三个主键 + 属性条。
fn action_row(ui: &mut egui::Ui, app: &AppState, out: &mut Vec<Intent>) {
    let busy = !matches!(app.conn, ConnStatus::Online);
    let cult = app.vitals.cultivating;
    ui.horizontal_centered(|ui| {
        if big_button(ui, "历练", "Space", busy, true).clicked() {
            out.push(Intent::Explore);
        }
        if big_button(ui, if cult { "收功" } else { "打坐" }, "C", busy, false).clicked() {
            out.push(Intent::Cultivate(!cult));
        }
        if big_button(ui, "突破", "", busy, false).clicked() {
            out.push(Intent::Breakthrough);
        }
        if ui
            .add(egui::Button::new(RichText::new("看看四周").size(12.5)).min_size(vec2(84.0, 56.0)))
            .on_hover_text("重新打量脚下（look）")
            .clicked()
        {
            out.push(Intent::Look);
        }

        ui.add_space(10.0);
        ui.vertical(|ui| {
            let p = app.player.get();
            let max_hp = p.map(|p| p.max_hp).unwrap_or(1).max(1);
            let max_mp = p.map(|p| p.max_mp).unwrap_or(1).max(1);
            let stamina_max = p.map(|p| p.stamina_max).unwrap_or(200).max(1);
            mini_bar(ui, "气血", app.vitals.hp, max_hp, theme::BAD);
            mini_bar(ui, "真气", app.vitals.mp, max_mp, theme::ACCENT);
            mini_bar(
                ui,
                "修为",
                app.vitals.exp,
                app.vitals.exp_need.max(1),
                theme::GOLD,
            );
            ui.horizontal(|ui| {
                ui.label(RichText::new("体力").size(10.0).color(theme::FAINT));
                ui.label(
                    RichText::new(format!("{}/{}", app.vitals.stamina, stamina_max))
                        .size(11.0)
                        .color(if percent(app.vitals.stamina, stamina_max) < 0.2 {
                            theme::BAD
                        } else {
                            theme::TEXT
                        }),
                );
            });
        });
    });
}

fn big_button(
    ui: &mut egui::Ui,
    label: &str,
    hint: &str,
    disabled: bool,
    primary: bool,
) -> egui::Response {
    let text = if hint.is_empty() {
        RichText::new(label).size(15.0)
    } else {
        RichText::new(format!("{label}\n{hint}")).size(14.0)
    };
    let mut btn = egui::Button::new(text).min_size(vec2(88.0, 56.0));
    if primary {
        btn = btn
            .fill(Color32::from_rgb(0x1C, 0x3D, 0x3A))
            .stroke(Stroke::new(1.0_f32, theme::ACCENT));
    }
    ui.add_enabled(!disabled, btn)
}

fn mini_bar(ui: &mut egui::Ui, label: &str, cur: i64, max: i64, color: Color32) {
    let (rect, _) = ui.allocate_exact_size(vec2(238.0, 13.0), Sense::hover());
    let p = ui.painter();
    p.rect_filled(rect, 2, Color32::from_rgb(0x0A, 0x0D, 0x10));
    let ratio = percent(cur, max);
    if ratio > 0.0 {
        p.rect_filled(
            Rect::from_min_size(rect.min, vec2(rect.width() * ratio, rect.height())),
            2,
            color.linear_multiply(0.72),
        );
    }
    p.text(
        rect.left_center() + vec2(6.0, 0.0),
        Align2::LEFT_CENTER,
        label,
        egui::FontId::proportional(9.5),
        theme::mix(theme::TEXT, theme::BG, 0.1),
    );
    p.text(
        rect.right_center() + vec2(-6.0, 0.0),
        Align2::RIGHT_CENTER,
        format!("{}/{}", cur.max(0), max),
        egui::FontId::proportional(9.5),
        theme::mix(theme::TEXT, theme::BG, 0.05),
    );
    ui.add_space(0.0);
}

fn bearing_arrow(dx: i32, dy: i32) -> &'static str {
    if dx == 0 && dy == 0 {
        return "·";
    }
    const ARROWS: [&str; 8] = ["→", "↗", "↑", "↖", "←", "↙", "↓", "↘"];
    let ang = (dy as f32).atan2(dx as f32);
    let idx = (ang / (std::f32::consts::PI / 4.0)).round() as i32;
    ARROWS[idx.rem_euclid(8) as usize]
}

// ───────────────────────── 数据页（整页覆盖） ─────────────────────────

fn page_screen(
    ui: &mut egui::Ui,
    st: &mut UiState,
    app: &AppState,
    out: &mut Vec<Intent>,
    page: Page,
) {
    egui::TopBottomPanel::top("page_head")
        .exact_height(44.0)
        .frame(
            egui::Frame::new()
                .fill(theme::PANEL)
                .inner_margin(egui::Margin::symmetric(16, 0)),
        )
        .show_inside(ui, |ui| {
            ui.horizontal_centered(|ui| {
                if ui
                    .add(egui::Button::new(RichText::new("← 返回").size(12.0)).frame(false))
                    .clicked()
                {
                    st.page = Page::Game;
                }
                ui.add_space(6.0);
                ui.label(
                    RichText::new(page.label())
                        .font(theme::serif_id(17.0))
                        .color(theme::TEXT),
                );
                ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                    ui.label(RichText::new("Esc 返回").size(10.5).color(theme::FAINT));
                });
            });
        });
    egui::CentralPanel::default()
        .frame(
            egui::Frame::new()
                .fill(theme::BG)
                .inner_margin(egui::Margin::symmetric(20, 14)),
        )
        .show_inside(ui, |ui| {
            egui::ScrollArea::vertical().show(ui, |ui| match page {
                Page::Character => character_page(ui, app),
                Page::Bag => bag_page(ui, app),
                Page::Skills => skills_page(ui, app),
                Page::Social => {
                    page_stub(ui, "江湖", "好友 · 师徒 · 道侣 · 仇敌 · 盟约 · 队伍", "M3")
                }
                Page::Market => {
                    page_stub(ui, "坊市", "挂单 · 我的 · 成交 · 曲线 · 拍卖 · 书信", "M3")
                }
                Page::Sect => sect_page(ui, app),
                Page::Map => map_page(ui, st, app, out),
                Page::Settings => settings_page(ui, st, app, out),
                Page::Game => {}
            });
        });
}

// ───────────────────────── 数据页 ─────────────────────────

fn page_header(ui: &mut egui::Ui, title: &str, sub: &str) {
    ui.label(
        RichText::new(title)
            .font(theme::serif_id(24.0))
            .color(theme::TEXT),
    );
    if !sub.is_empty() {
        ui.label(RichText::new(sub).size(12.0).color(theme::DIM));
    }
    ui.add_space(4.0);
    let w = ui.available_width();
    let (rect, _) = ui.allocate_exact_size(vec2(w, 1.0), Sense::hover());
    ui.painter().rect_filled(rect, 0, theme::LINE);
    ui.add_space(8.0);
}

fn character_page(ui: &mut egui::Ui, app: &AppState) {
    let Some(p) = app.player.get() else {
        page_stub(ui, "角色", "尚未载入角色", "M1");
        return;
    };
    page_header(ui, "角色", &format!("{} · {}", p.name, p.realm));
    ui.horizontal_wrapped(|ui| {
        stat(ui, "攻击", &p.atk.to_string());
        stat(ui, "防御", &p.def.to_string());
        stat(ui, "身法", &p.spd.to_string());
        stat(ui, "会心", &format!("{:.0}%", p.crit * 100.0));
        stat(ui, "战力", &p.power.to_string());
        stat(
            ui,
            "灵根",
            &p.root
                .as_ref()
                .map(|r| r.name.clone())
                .unwrap_or("—".into()),
        );
    });
    ui.add_space(12.0);
    section(ui, "根骨");
    ui.horizontal_wrapped(|ui| {
        stat(ui, "体魄", &p.attrs.ti.to_string());
        stat(ui, "灵性", &p.attrs.ling.to_string());
        stat(ui, "身法", &p.attrs.feng.to_string());
        stat(ui, "护体", &p.attrs.hu.to_string());
        stat(ui, "神识", &p.attrs.shen.to_string());
        stat(ui, "未分配", &p.free.to_string());
    });
    ui.add_space(12.0);
    section(ui, "名分");
    ui.horizontal_wrapped(|ui| {
        stat(
            ui,
            "势力",
            if p.faction_name.is_empty() {
                "—"
            } else {
                &p.faction_name
            },
        );
        stat(
            ui,
            "宗门",
            if p.guild_name.is_empty() {
                "—"
            } else {
                &p.guild_name
            },
        );
        stat(
            ui,
            "职位",
            if p.guild_rank.is_empty() {
                "—"
            } else {
                &p.guild_rank
            },
        );
        stat(ui, "功德", &p.merit.to_string());
        stat(ui, "轮回", &p.lives.to_string());
        stat(ui, "丹毒", &format!("{}（{}）", p.toxin, p.toxin_note));
    });
    if !p.quests.is_empty() {
        ui.add_space(12.0);
        section(ui, "手中事");
        for q in &p.quests {
            ui.horizontal(|ui| {
                ui.label(RichText::new(&q.desc).size(13.0).color(theme::TEXT));
                ui.label(
                    RichText::new(format!("{}/{}", q.prog, q.target))
                        .size(11.0)
                        .color(theme::DIM),
                );
                if q.claimed {
                    ui.label(RichText::new("已领").size(11.0).color(theme::FAINT));
                }
            });
        }
    }
}

fn bag_page(ui: &mut egui::Ui, app: &AppState) {
    let Some(p) = app.player.get() else {
        page_stub(ui, "背包", "尚未载入角色", "M1");
        return;
    };
    page_header(
        ui,
        "背包",
        &format!("{} 件 · 灵石 {}", p.bag.len(), thousands(p.stones)),
    );
    egui::Grid::new("bag_grid")
        .num_columns(4)
        .striped(true)
        .min_col_width(120.0)
        .show(ui, |ui| {
            head(ui, "名称");
            head(ui, "数量");
            head(ui, "类别");
            head(ui, "售价");
            ui.end_row();
            for it in &p.bag {
                ui.label(RichText::new(&it.name).size(13.0).color(theme::TEXT));
                ui.label(
                    RichText::new(it.n.to_string())
                        .size(13.0)
                        .color(theme::TEXT),
                );
                ui.label(
                    RichText::new(kind_name(&it.kind))
                        .size(12.0)
                        .color(theme::DIM),
                );
                ui.label(
                    RichText::new(thousands(it.sell))
                        .size(12.0)
                        .color(theme::GOLD),
                );
                ui.end_row();
            }
        });
    if p.bag.is_empty() {
        ui.add_space(16.0);
        ui.vertical_centered(|ui| {
            ui.label(RichText::new("行囊空空。").size(14.0).color(theme::DIM));
        });
    }
}

fn skills_page(ui: &mut egui::Ui, app: &AppState) {
    let Some(p) = app.player.get() else {
        page_stub(ui, "技能", "尚未载入角色", "M1");
        return;
    };
    page_header(
        ui,
        "技能",
        &format!(
            "主修 {} · 功法 {} 门 · 技能槽 {}",
            if p.main_name.is_empty() {
                "—"
            } else {
                &p.main_name
            },
            p.manuals.len(),
            p.slots.len()
        ),
    );
    egui::Grid::new("skill_grid")
        .num_columns(4)
        .striped(true)
        .min_col_width(120.0)
        .show(ui, |ui| {
            head(ui, "技能");
            head(ui, "真气");
            head(ui, "威力");
            head(ui, "状态");
            ui.end_row();
            for s in &p.skill_book {
                ui.label(RichText::new(&s.name).size(13.0).color(theme::TEXT));
                ui.label(
                    RichText::new(s.mp.to_string())
                        .size(12.0)
                        .color(theme::ACCENT),
                );
                ui.label(
                    RichText::new(format!("{:.1}", s.power))
                        .size(12.0)
                        .color(theme::TEXT),
                );
                ui.label(
                    RichText::new(if s.equipped { "已装配" } else { "未装配" })
                        .size(11.0)
                        .color(theme::DIM),
                );
                ui.end_row();
            }
        });
}

fn sect_page(ui: &mut egui::Ui, app: &AppState) {
    let Some(p) = app.player.get() else {
        page_stub(ui, "宗门", "尚未载入角色", "M1");
        return;
    };
    page_header(
        ui,
        "宗门",
        if p.guild_name.is_empty() {
            "散修"
        } else {
            &p.guild_name
        },
    );
    if p.guild_name.is_empty() {
        ui.label(
            RichText::new("尚未加入玩家宗门。")
                .size(13.0)
                .color(theme::DIM),
        );
        ui.add_space(6.0);
        ui.label(
            RichText::new("建宗、捐献、宗门战在 M4 落地。")
                .size(12.0)
                .color(theme::FAINT),
        );
    } else {
        ui.horizontal_wrapped(|ui| {
            stat(ui, "宗门", &p.guild_name);
            stat(
                ui,
                "职位",
                if p.guild_rank.is_empty() {
                    "—"
                } else {
                    &p.guild_rank
                },
            );
            stat(ui, "贡献", &p.guild_contrib.to_string());
        });
    }
    ui.add_space(12.0);
    section(ui, "NPC 势力");
    ui.horizontal_wrapped(|ui| {
        stat(
            ui,
            "势力",
            if p.faction_name.is_empty() {
                "—"
            } else {
                &p.faction_name
            },
        );
        stat(ui, "声望", &format!("{}（{}）", p.fame, p.fame_tier));
        stat(ui, "势力贡献", &p.contrib.to_string());
    });
}

fn map_page(ui: &mut egui::Ui, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    let Some(m) = app.world.get() else {
        page_stub(ui, "舆图", "尚未载入地貌", "M1");
        return;
    };
    page_header(
        ui,
        "舆图",
        &format!("{} · {}", m.region_name, m.path.join(" / ")),
    );
    if let Some((to, walkable)) = ui::mapview::show(ui, &app.map_graph, &m.loc, &mut st.map_view)
        && walkable
    {
        out.push(Intent::Move(to));
    }
}

fn settings_page(ui: &mut egui::Ui, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    page_header(ui, "设置", "");
    ui.horizontal(|ui| {
        ui.label(RichText::new("界面缩放").size(13.0).color(theme::TEXT));
        for s in [1.0f32, 1.25, 1.5, 2.0] {
            if ui
                .selectable_label((st.ui_scale - s).abs() < 0.01, format!("{:.0}%", s * 100.0))
                .clicked()
            {
                st.ui_scale = s;
                ui.ctx().set_zoom_factor(s);
                out.push(Intent::Local);
            }
        }
    });
    ui.checkbox(&mut st.show_timestamps, "见闻显示时刻");
    ui.add_space(10.0);
    section(ui, "连接");
    kv(ui, "服务器", &st.login.server);
    kv(ui, "字体", &st.font_source);
    kv(
        ui,
        "协议",
        &format!(
            "v{} · 角色 {} · 日志 {} 行",
            crate::protocol::PROTO,
            if app.has_char { "已载入" } else { "无" },
            app.logs.len()
        ),
    );
    ui.add_space(12.0);
    if ui.button("退出登录").clicked() {
        out.push(Intent::Logout);
    }
}

fn page_stub(ui: &mut egui::Ui, title: &str, desc: &str, milestone: &str) {
    page_header(ui, title, desc);
    ui.add_space(24.0);
    ui.vertical_centered(|ui| {
        ui.label(
            RichText::new(format!("此处在 {milestone} 开张"))
                .font(theme::serif_id(18.0))
                .color(theme::DIM),
        );
        ui.add_space(6.0);
        ui.label(
            RichText::new("当前版本先保证主循环顺手。")
                .size(12.0)
                .color(theme::FAINT),
        );
    });
}

fn section(ui: &mut egui::Ui, title: &str) {
    ui.label(
        RichText::new(title)
            .font(theme::serif_id(14.0))
            .color(theme::GOLD),
    );
    ui.add_space(3.0);
}

fn head(ui: &mut egui::Ui, text: &str) {
    ui.label(RichText::new(text).size(11.0).color(theme::FAINT));
}

fn kv(ui: &mut egui::Ui, k: &str, v: &str) {
    ui.horizontal(|ui| {
        ui.label(RichText::new(k).size(11.0).color(theme::FAINT));
        ui.label(RichText::new(v).size(12.0).color(theme::TEXT));
    });
}

fn stat(ui: &mut egui::Ui, k: &str, v: &str) {
    egui::Frame::new()
        .fill(theme::CARD)
        .stroke(Stroke::new(1.0_f32, theme::LINE))
        .inner_margin(egui::Margin::symmetric(10, 6))
        .corner_radius(3u8)
        .show(ui, |ui| {
            ui.vertical(|ui| {
                ui.label(RichText::new(k).size(10.5).color(theme::FAINT));
                ui.label(RichText::new(v).size(14.0).color(theme::TEXT));
            });
        });
}

// ───────────────────────── 底栏 ─────────────────────────

// ───────────────────────── 名称映射 ─────────────────────────

fn season_name(s: &str) -> &str {
    match s {
        "spring" => "春",
        "summer" => "夏",
        "autumn" => "秋",
        "winter" => "冬",
        other => other,
    }
}

fn weather_name(w: &str) -> &str {
    match w {
        "clear" => "晴",
        "rain" => "雨",
        "fog" => "雾",
        "wind" => "风",
        "snow" => "雪",
        "frost" => "霜",
        "hail" => "冰雹",
        "sandstorm" => "沙尘",
        "storm" => "雷暴",
        other => other,
    }
}

fn terrain_name(t: &str) -> &str {
    match t {
        "plain" => "平地",
        "road" => "官道",
        "trail" => "山径",
        "woods" | "forest" => "密林",
        "bamboo" => "竹林",
        "river" => "河流",
        "lake" => "湖",
        "spring" => "灵泉",
        "cliff" => "悬崖",
        "peak" => "峰顶",
        "slope" => "坡",
        "valley" => "谷",
        "cave" => "洞",
        "town" => "城镇",
        "gate" => "关隘",
        "marsh" => "沼泽",
        "scree" => "石林",
        "plank" => "栈道",
        "lava" => "熔岩",
        "frost" => "霜原",
        "desert" => "荒漠",
        "snow" => "雪原",
        "ruin" => "废墟",
        "shrine" => "祠",
        other => other,
    }
}

fn cond_text(cond: &str) -> String {
    if let Some(rest) = cond.strip_prefix("realm:") {
        return format!("需境界 {rest}");
    }
    if let Some(rest) = cond.strip_prefix("item:") {
        return format!("需物品 {rest}");
    }
    if let Some(rest) = cond.strip_prefix("manual:") {
        return format!("需功法 {rest}");
    }
    if let Some(rest) = cond.strip_prefix("token:") {
        return format!("需信物 {rest}");
    }
    if let Some(rest) = cond.strip_prefix("time:") {
        return format!("限时 {rest}");
    }
    if let Some(rest) = cond.strip_prefix("blocked:") {
        return format!("断路 {rest}");
    }
    match cond {
        "fly" => "需御风".into(),
        "water" => "需避水".into(),
        "owner" => "主人专用".into(),
        other => other.to_string(),
    }
}

fn kind_name(k: &str) -> &str {
    match k {
        "pill" | "dan" => "丹药",
        "weapon" => "法器",
        "armor" => "护甲",
        "boots" => "鞋履",
        "accessory" => "饰物",
        "herb" | "material" => "材料",
        "manual" => "功法",
        "token" => "信物",
        "food" => "食物",
        other => other,
    }
}
