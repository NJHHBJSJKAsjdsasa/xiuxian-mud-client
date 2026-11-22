//! 主界面：顶栏 + 书签式导航 + 场景/见闻 + 人物卡 + 底栏。
//!
//! 视觉目标是"翻开一本游记"，不是"打开一个后台"：
//! 中间是场景与见闻，右侧是人物牌位，左侧是书签。

use egui::{Align, Align2, Color32, Layout, Rect, RichText, Sense, Stroke, TextEdit, pos2, vec2};

use crate::net::ConnStatus;
use crate::store::AppState;
use crate::ui::{self, Intent, Page, UiState, scene, theme};
use crate::util::text::{percent, thousands};

pub fn show(ctx: &egui::Context, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    top_bar(ctx, st, app);
    bookmark_rail(ctx, st);
    character_panel(ctx, st, app, out);
    bottom_bar(ctx, st, app, out);

    egui::CentralPanel::default()
        .frame(
            egui::Frame::new()
                .fill(theme::BG)
                .inner_margin(egui::Margin::same(12)),
        )
        .show(ctx, |ui| match st.page {
            Page::Game => game_page(ui, st, app, out),
            Page::Character => character_page(ui, app),
            Page::Bag => bag_page(ui, app),
            Page::Skills => skills_page(ui, app),
            Page::Social => page_stub(ui, "江湖", "好友 · 师徒 · 道侣 · 仇敌 · 盟约 · 队伍", "M3"),
            Page::Market => page_stub(ui, "坊市", "挂单 · 我的 · 成交 · 曲线 · 拍卖 · 书信", "M3"),
            Page::Sect => sect_page(ui, app),
            Page::Map => map_page(ui, st, app, out),
            Page::Settings => settings_page(ui, st, app, out),
        });
}

// ───────────────────────── 顶栏 ─────────────────────────

fn top_bar(ctx: &egui::Context, st: &mut UiState, app: &AppState) {
    egui::TopBottomPanel::top("top")
        .exact_height(46.0)
        .frame(
            egui::Frame::new()
                .fill(theme::PANEL)
                .inner_margin(egui::Margin::symmetric(14, 0)),
        )
        .show(ctx, |ui| {
            ui.horizontal_centered(|ui| {
                if let Some(p) = app.player.get() {
                    ui.label(
                        RichText::new(&p.name)
                            .font(theme::serif_id(17.0))
                            .color(theme::TEXT),
                    );
                    ui.add_space(2.0);
                    ui.label(RichText::new(&p.realm).size(12.0).color(theme::ACCENT));
                    ui.add_space(14.0);
                    ui.label(RichText::new("灵石").size(11.0).color(theme::FAINT));
                    ui.label(
                        RichText::new(thousands(p.stones))
                            .size(14.0)
                            .color(theme::GOLD),
                    );
                    ui.add_space(10.0);
                    ui.label(RichText::new("功德").size(11.0).color(theme::FAINT));
                    ui.label(
                        RichText::new(thousands(p.merit))
                            .size(14.0)
                            .color(theme::ACCENT),
                    );
                } else {
                    ui.label(RichText::new("尚未载入角色").color(theme::DIM));
                }

                ui.add_space(18.0);
                if let Some(m) = app.world.get() {
                    let season = season_name(m.season.as_deref().unwrap_or(""));
                    let weather = weather_name(m.weather.as_deref().unwrap_or(""));
                    let shard = m.shard.map(|s| format!("{s} 线")).unwrap_or_default();
                    let bits: Vec<&str> = [season, weather]
                        .into_iter()
                        .filter(|s| !s.is_empty())
                        .collect();
                    let mut text = bits.join(" · ");
                    if !shard.is_empty() {
                        if !text.is_empty() {
                            text.push_str(" · ");
                        }
                        text.push_str(&shard);
                    }
                    if !text.is_empty() {
                        ui.label(RichText::new(text).size(12.0).color(theme::DIM));
                    }
                }
                ui.label(
                    RichText::new(format!("在线 {}", app.presence))
                        .size(12.0)
                        .color(theme::FAINT),
                );

                ui.add_space(12.0);
                let (color, text) = conn_badge(&app.conn);
                ui.label(RichText::new("●").size(11.0).color(color));
                ui.label(RichText::new(text).size(12.0).color(color));

                ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                    if ui
                        .add(egui::Button::new(RichText::new("设置").size(12.0)).frame(false))
                        .clicked()
                    {
                        st.page = Page::Settings;
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
        ConnStatus::Backoff { secs, attempt } => {
            (theme::WARN, format!("重连 {secs}s（第 {attempt} 次）"))
        }
        ConnStatus::Idle => (theme::FAINT, "未连接".into()),
        ConnStatus::Offline(msg) => (theme::BAD, msg.clone()),
        ConnStatus::Fatal(msg) => (theme::BAD, msg.clone()),
    }
}

// ───────────────────────── 书签导航 ─────────────────────────

fn bookmark_rail(ctx: &egui::Context, st: &mut UiState) {
    egui::SidePanel::left("rail")
        .exact_width(88.0)
        .resizable(false)
        .frame(
            egui::Frame::new()
                .fill(theme::PANEL)
                .inner_margin(egui::Margin::symmetric(0, 10)),
        )
        .show(ctx, |ui| {
            for page in Page::all() {
                let active = st.page == page;
                let (rect, resp) =
                    ui.allocate_exact_size(vec2(ui.available_width(), 48.0), Sense::click());
                let p = ui.painter();
                if active {
                    p.rect_filled(rect, 0, theme::PANEL_HI);
                    p.rect_filled(
                        Rect::from_min_size(rect.left_top(), vec2(3.0, rect.height())),
                        0,
                        theme::ACCENT,
                    );
                } else if resp.hovered() {
                    p.rect_filled(rect, 0, theme::mix(theme::PANEL, theme::PANEL_HI, 0.6));
                }
                p.text(
                    rect.center(),
                    Align2::CENTER_CENTER,
                    page.label(),
                    theme::serif_id(if active { 17.0 } else { 16.0 }),
                    if active { theme::ACCENT } else { theme::DIM },
                );
                if resp.clicked() {
                    st.page = page;
                }
                resp.on_hover_text(page_hint(page));
            }
        });
}

fn page_hint(p: Page) -> &'static str {
    match p {
        Page::Game => "当前场景 · Esc",
        Page::Character => "角色 · F1",
        Page::Bag => "背包 · F2",
        Page::Skills => "技能 · F3",
        Page::Social => "江湖 · F4",
        Page::Market => "坊市 · F5",
        Page::Sect => "宗门 · F6",
        Page::Map => "舆图 · F7",
        Page::Settings => "设置 · F8",
    }
}

// ───────────────────────── 人物卡 ─────────────────────────

fn character_panel(ctx: &egui::Context, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    egui::SidePanel::right("character")
        // 固定宽度：窗口初始化时若被压过一次，egui 会记住 0 宽，之后一直不显示。
        .exact_width(300.0)
        .resizable(false)
        .show(ctx, |ui| {
            let p = app.player.get();
            let v = &app.vitals;

            ui.horizontal(|ui| {
                let (rect, _) = ui.allocate_exact_size(vec2(52.0, 52.0), Sense::hover());
                let pnt = ui.painter();
                pnt.circle_filled(rect.center(), 25.0, theme::CARD);
                pnt.circle_stroke(rect.center(), 25.0, Stroke::new(1.0_f32, theme::LINE));
                pnt.circle_stroke(rect.center(), 21.0, Stroke::new(1.0_f32, theme::FAINT));
                let initial = p
                    .and_then(|p| p.name.chars().next())
                    .map(|c| c.to_string())
                    .unwrap_or_else(|| "道".into());
                pnt.text(
                    rect.center(),
                    Align2::CENTER_CENTER,
                    initial,
                    theme::serif_id(26.0),
                    theme::GOLD,
                );
                ui.add_space(10.0);
                ui.vertical(|ui| {
                    ui.label(
                        RichText::new(p.map(|p| p.name.as_str()).unwrap_or("—"))
                            .font(theme::serif_id(20.0))
                            .color(theme::TEXT),
                    );
                    let realm = p.map(|p| p.realm.as_str()).unwrap_or("");
                    let root = p
                        .and_then(|p| p.root.as_ref())
                        .map(|r| r.name.as_str())
                        .unwrap_or("");
                    let mut sub = realm.to_string();
                    if !root.is_empty() {
                        sub.push_str(" · ");
                        sub.push_str(root);
                    }
                    ui.label(RichText::new(sub).size(12.0).color(theme::DIM));
                });
            });

            ui.add_space(12.0);
            ui.separator();
            ui.add_space(8.0);

            let max_hp = p.map(|p| p.max_hp).unwrap_or(1).max(1);
            let max_mp = p.map(|p| p.max_mp).unwrap_or(1).max(1);
            stat_bar(ui, "气血", v.hp, max_hp, theme::BAD);
            stat_bar(ui, "真气", v.mp, max_mp, theme::ACCENT);
            stat_bar(ui, "修为", v.exp, v.exp_need.max(1), theme::GOLD);

            ui.add_space(4.0);
            let stamina_max = p.map(|p| p.stamina_max).unwrap_or(200).max(1);
            ui.horizontal(|ui| {
                ui.label(RichText::new("体力").size(11.0).color(theme::FAINT));
                let color = if percent(v.stamina, stamina_max) < 0.2 {
                    theme::BAD
                } else {
                    theme::TEXT
                };
                ui.label(
                    RichText::new(format!("{} / {}", v.stamina, stamina_max))
                        .size(12.0)
                        .color(color),
                );
                if v.cultivating {
                    ui.label(RichText::new("· 打坐中").size(11.0).color(theme::ACCENT));
                }
            });

            ui.add_space(10.0);
            ui.separator();
            ui.add_space(8.0);

            let busy = !matches!(app.conn, ConnStatus::Online);
            ui.horizontal(|ui| {
                if action_button(ui, "历练", "Space", busy, true, 0).clicked() {
                    out.push(Intent::Explore);
                }
                let cult = v.cultivating;
                if action_button(ui, if cult { "收功" } else { "打坐" }, "C", busy, false, 1)
                    .clicked()
                {
                    out.push(Intent::Cultivate(!cult));
                }
            });
            ui.horizontal(|ui| {
                if action_button(ui, "突破", "", busy, false, 0).clicked() {
                    out.push(Intent::Breakthrough);
                }
                if action_button(ui, "四处看看", "", busy, false, 1).clicked() {
                    out.push(Intent::Look);
                }
            });

            // 把"今日"以下推到贴底，避免人物卡中间空一大块。
            let reserve = 108.0;
            let gap = (ui.available_height() - reserve).max(8.0);
            ui.add_space(gap);
            ui.separator();
            ui.add_space(6.0);
            ui.label(RichText::new("今日").size(11.0).color(theme::FAINT));
            if let Some(p) = p {
                ui.horizontal(|ui| {
                    ui.label(
                        RichText::new(format!("历练 {}", p.stats.explores))
                            .size(12.0)
                            .color(theme::DIM),
                    );
                    ui.label(
                        RichText::new(format!("击杀 {}", p.stats.kills))
                            .size(12.0)
                            .color(theme::DIM),
                    );
                    ui.label(
                        RichText::new(format!("突破 {}", p.stats.breaks))
                            .size(12.0)
                            .color(theme::DIM),
                    );
                });
            }

            ui.label(
                RichText::new("F1–F8 切页 · Space 历练 · C 打坐 · Enter 说话")
                    .size(10.5)
                    .color(theme::FAINT),
            );
            let _ = st;
        });
}

fn action_button(
    ui: &mut egui::Ui,
    label: &str,
    hint: &str,
    disabled: bool,
    primary: bool,
    col: usize,
) -> egui::Response {
    // 两列等宽：由调用方传列号，宽度按半行算。
    let w = if col == 0 {
        (ui.available_width() - 8.0) / 2.0
    } else {
        ui.available_width()
    };
    let text = if hint.is_empty() {
        RichText::new(label).size(14.0)
    } else {
        RichText::new(format!("{label}  {hint}")).size(13.0)
    };
    let mut btn = egui::Button::new(text).min_size(vec2(w.max(64.0), 32.0));
    if primary {
        btn = btn
            .fill(Color32::from_rgb(0x1C, 0x3D, 0x3A))
            .stroke(Stroke::new(1.0_f32, theme::ACCENT));
    }
    ui.add_enabled(!disabled, btn)
}

/// 自绘状态条：底槽 + 填充 + 左右文字，比默认 ProgressBar 更像游戏血条。
fn stat_bar(ui: &mut egui::Ui, label: &str, cur: i64, max: i64, color: Color32) {
    let h = 17.0;
    let (rect, _) = ui.allocate_exact_size(vec2(ui.available_width(), h), Sense::hover());
    let p = ui.painter();
    p.rect_filled(rect, 2, Color32::from_rgb(0x0A, 0x0D, 0x10));
    p.rect_stroke(
        rect,
        2,
        Stroke::new(1.0_f32, theme::LINE),
        egui::StrokeKind::Inside,
    );
    let ratio = percent(cur, max);
    if ratio > 0.0 {
        let fill = Rect::from_min_size(
            rect.min + vec2(1.0, 1.0),
            vec2((rect.width() - 2.0) * ratio, h - 2.0),
        );
        p.rect_filled(fill, 1, color.linear_multiply(0.72));
        p.rect_filled(
            Rect::from_min_size(fill.min, vec2(fill.width(), 1.0)),
            0,
            color.linear_multiply(1.25),
        );
    }
    p.text(
        rect.left_center() + vec2(7.0, 0.0),
        Align2::LEFT_CENTER,
        label,
        egui::FontId::proportional(11.0),
        theme::mix(theme::TEXT, theme::BG, 0.15),
    );
    p.text(
        rect.right_center() + vec2(-7.0, 0.0),
        Align2::RIGHT_CENTER,
        format!("{} / {}", cur.max(0), max),
        egui::FontId::proportional(11.0),
        theme::mix(theme::TEXT, theme::BG, 0.05),
    );
    ui.add_space(3.0);
}

// ───────────────────────── 游戏页 ─────────────────────────

fn game_page(ui: &mut egui::Ui, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
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

    let scene_h = (ui.available_height() * 0.47).clamp(200.0, 320.0);
    scene::show(
        ui,
        scene::Scene {
            name: &m.name,
            region: &m.region_name,
            terrain: &m.terrain,
            weather: m.weather.as_deref().unwrap_or(""),
            daypart: m.daypart.as_deref().unwrap_or(""),
            grade: m.grade.as_deref(),
            desc: &m.desc,
            explored: m.explored,
        },
        scene_h,
    );

    ui.add_space(6.0);
    travel_bar(ui, st, app);
    ui.add_space(6.0);
    let traveling = app
        .player
        .get()
        .map(|p| p.travel.is_some())
        .unwrap_or(false);
    exits_row(ui, st, m, out, traveling);

    ui.add_space(6.0);
    ui.horizontal(|ui| {
        ui.label(
            RichText::new("见闻")
                .font(theme::serif_id(15.0))
                .color(theme::GOLD),
        );
        ui.label(
            RichText::new(format!("{} 条", app.logs.len()))
                .size(11.0)
                .color(theme::FAINT),
        );
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            if !st.log_stick && ui.small_button("↓ 到底").clicked() {
                st.log_stick = true;
            }
            if ui
                .add(egui::Button::new(RichText::new("时刻").size(11.0)).frame(false))
                .on_hover_text(if st.show_timestamps {
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
    let w = ui.available_width();
    let (rect, _) = ui.allocate_exact_size(vec2(w, 1.0), Sense::hover());
    ui.painter()
        .rect_filled(rect, 0, theme::LINE.linear_multiply(0.8));
    ui.add_space(2.0);
    // 见闻区做成纸页：淡底 + 细边，避免大片纯黑。
    egui::Frame::new()
        .fill(theme::CARD)
        .stroke(Stroke::new(1.0_f32, theme::LINE.linear_multiply(0.7)))
        .inner_margin(egui::Margin::symmetric(10, 8))
        .show(ui, |ui| {
            ui.set_min_height(ui.available_height());
            ui::logview::show(ui, st, app);
        });
}

fn exits_row(
    ui: &mut egui::Ui,
    st: &mut UiState,
    m: &crate::protocol::msg::MapView,
    out: &mut Vec<Intent>,
    traveling: bool,
) {
    ui.horizontal(|ui| {
        ui.label(
            RichText::new("去处")
                .font(theme::serif_id(15.0))
                .color(theme::GOLD),
        );
        ui.label(
            RichText::new(format!("{} 处", m.exits.len()))
                .size(11.0)
                .color(theme::FAINT),
        );
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            ui.label(
                RichText::new("点一张就走 · 也可按 ↑↓ 选、Enter 确认")
                    .size(10.5)
                    .color(theme::FAINT),
            );
        });
    });
    ui.add_space(2.0);
    if m.exits.is_empty() {
        ui.label(
            RichText::new("四面是崖，无路可走。")
                .size(13.0)
                .color(theme::DIM),
        );
        return;
    }

    let total_w = ui.available_width();
    let per_row = ((total_w + 8.0) / 188.0).floor().max(1.0) as usize;
    let card_w = (total_w - 8.0 * (per_row as f32 - 1.0)) / per_row as f32;
    egui::Grid::new("exits_grid")
        .num_columns(per_row)
        .spacing(vec2(8.0, 8.0))
        .show(ui, |ui| {
            for (i, ex) in m.exits.iter().enumerate() {
                let name = ex
                    .name
                    .clone()
                    .filter(|s| !s.is_empty())
                    .unwrap_or_else(|| "雾中".into());
                let blocked = ex.blocked == Some(true);
                // 赶路中所有出口都不可点：服务端也会拒，这里先拦一道。
                let ok = ex.ok && !blocked && !traveling;
                let mut sub = format!("{} · {}s", terrain_name(&ex.terrain), ex.cost_sec);
                if let Some(fee) = ex.fee
                    && fee > 0
                {
                    sub.push_str(&format!(" · {fee} 石"));
                }
                if blocked {
                    sub = "断路".into();
                } else if let Some(cond) = &ex.cond
                    && !cond.is_empty()
                {
                    sub.push('·');
                    sub.push_str(&cond_text(cond));
                }
                // 有坐标就按真实方位给箭头，一眼知道往哪走。
                let arrow = match (m.x, m.y, ex.x, ex.y) {
                    (Some(cx), Some(cy), Some(tx), Some(ty)) => {
                        format!("{} ", bearing_arrow(tx - cx, ty - cy))
                    }
                    _ => String::new(),
                };
                let color = if ok { theme::TEXT } else { theme::FAINT };
                let text = RichText::new(format!("{arrow}{name}\n{sub}"))
                    .size(13.0)
                    .color(color);
                let mut btn = egui::Button::new(text)
                    .min_size(vec2(card_w, 46.0))
                    .fill(theme::CARD)
                    .stroke(Stroke::new(
                        1.0_f32,
                        if ok {
                            theme::LINE
                        } else {
                            theme::LINE.linear_multiply(0.6)
                        },
                    ));
                if ex.temp == Some(true) {
                    btn = btn.stroke(Stroke::new(1.0_f32, theme::WARN.linear_multiply(0.8)));
                }
                let resp = ui.add_enabled(ok, btn).on_hover_text(if ok {
                    format!("前往 {} · 约 {} 秒", name, ex.cost_sec)
                } else if traveling {
                    "正在赶路，到了才能再走".to_string()
                } else if blocked {
                    "这条路断了".to_string()
                } else {
                    format!(
                        "走不了：{}",
                        ex.cond.as_deref().map(cond_text).unwrap_or_default()
                    )
                });
                if resp.clicked() {
                    st.selected_exit = i;
                    out.push(Intent::Move(ex.to.clone()));
                }
                if i + 1 == m.exits.len() || (i + 1) % per_row == 0 {
                    ui.end_row();
                }
            }
        });
}

/// 8 向箭头。服务端 y 向北为正，屏幕向上，所以直接用 dy 求角。
fn bearing_arrow(dx: i32, dy: i32) -> &'static str {
    if dx == 0 && dy == 0 {
        return "·";
    }
    const ARROWS: [&str; 8] = ["→", "↗", "↑", "↖", "←", "↙", "↓", "↘"];
    let ang = (dy as f32).atan2(dx as f32);
    let idx = (ang / (std::f32::consts::PI / 4.0)).round() as i32;
    ARROWS[idx.rem_euclid(8) as usize]
}

/// 赶路状态：放在场景正下方，比塞在右栏一行小字显眼。
fn travel_bar(ui: &mut egui::Ui, st: &mut UiState, app: &AppState) {
    let Some(t) = app.player.get().and_then(|p| p.travel.as_ref()) else {
        return;
    };
    let left = t.arrive_at - app.server_now();
    let _ = st;
    egui::Frame::new()
        .fill(Color32::from_rgb(0x18, 0x2A, 0x2A))
        .stroke(Stroke::new(1.0_f32, theme::ACCENT.linear_multiply(0.7)))
        .inner_margin(egui::Margin::symmetric(10, 6))
        .show(ui, |ui| {
            ui.set_width(ui.available_width());
            ui.horizontal(|ui| {
                ui.label(RichText::new("→").size(15.0).color(theme::ACCENT));
                ui.label(
                    RichText::new(format!("正在赶往 {}", t.name))
                        .size(13.0)
                        .color(theme::TEXT),
                );
                ui.label(
                    RichText::new("路上不能历练、交易")
                        .size(10.5)
                        .color(theme::FAINT),
                );
                ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                    ui.label(
                        RichText::new(crate::util::time::fmt_countdown(left))
                            .size(14.0)
                            .monospace()
                            .color(theme::ACCENT),
                    );
                    ui.label(RichText::new("剩余").size(10.5).color(theme::FAINT));
                });
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

fn bottom_bar(ctx: &egui::Context, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    egui::TopBottomPanel::bottom("bottom_bar")
        .exact_height(62.0)
        .show(ctx, |ui| {
            egui::Frame::new()
                .fill(theme::PANEL)
                .inner_margin(egui::Margin::symmetric(14, 6))
                .show(ui, |ui| {
                    ui.horizontal(|ui| {
                        let ch_label = match st.chat_ch.as_str() {
                            "region" => "区域",
                            "party" => "队伍",
                            "dm" => "私语",
                            _ => "世界",
                        };
                        egui::ComboBox::from_id_salt("chat_ch")
                            .selected_text(RichText::new(ch_label).size(12.0))
                            .width(66.0)
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
                                    .desired_width(84.0),
                            );
                        }
                        let online = matches!(app.conn, ConnStatus::Online);
                        let w = ui.available_width();
                        let resp = ui.add(
                            TextEdit::singleline(&mut st.chat_text)
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
                                pos2(r.left(), r.bottom() + 2.0),
                                pos2(r.right(), r.bottom() + 2.0),
                            ],
                            Stroke::new(
                                1.0_f32,
                                if resp.has_focus() {
                                    theme::ACCENT
                                } else {
                                    theme::LINE
                                },
                            ),
                        );
                        let enter =
                            resp.lost_focus() && ui.input(|i| i.key_pressed(egui::Key::Enter));
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
                            }
                        }
                    });
                    ui.add_space(3.0);
                    ui.horizontal(|ui| {
                        ui.label(
                            RichText::new(match &app.conn {
                                ConnStatus::Online => "已连上服务端",
                                ConnStatus::Connecting | ConnStatus::LoggingIn => {
                                    "连接中，动作暂不可用"
                                }
                                ConnStatus::Backoff { .. } => "正在重连",
                                ConnStatus::Offline(m) => m.as_str(),
                                ConnStatus::Fatal(m) => m.as_str(),
                                ConnStatus::Idle => "未连接",
                            })
                            .size(10.5)
                            .color(theme::FAINT),
                        );
                    });
                });
        });
}

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
