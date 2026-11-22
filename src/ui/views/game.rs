//! 主界面：顶栏 + 导航 + 主视图 + 状态栏 + 输入栏。

use egui::{Align, Layout, RichText, ScrollArea, TextEdit};

use crate::net::ConnStatus;
use crate::protocol::msg::PlayerView;
use crate::store::AppState;
use crate::ui::{self, Intent, Page, UiState, theme};
use crate::util::text::{percent, thousands};

pub fn show(ctx: &egui::Context, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    top_bar(ctx, st, app, out);
    nav(ctx, st);
    status_panel(ctx, st, app, out);
    bottom_bar(ctx, st, app, out);

    egui::CentralPanel::default().show(ctx, |ui| match st.page {
        Page::Game => game_page(ui, st, app, out),
        Page::Character => character_page(ui, app),
        Page::Bag => bag_page(ui, app),
        Page::Skills => skills_page(ui, app),
        Page::Social => page_stub(ui, "社交", "好友 / 师徒 / 队伍 / 屏蔽名单", "M3"),
        Page::Market => page_stub(ui, "交易", "挂单 / 我的 / 历史 / 曲线 / 拍卖 / 邮件", "M3"),
        Page::Sect => sect_page(ui, app),
        Page::Map => map_page(ui, app),
        Page::Settings => settings_page(ui, st, app, out),
    });
}

fn top_bar(ctx: &egui::Context, st: &mut UiState, app: &AppState, _out: &mut Vec<Intent>) {
    egui::TopBottomPanel::top("top")
        .exact_height(52.0)
        .show(ctx, |ui| {
            ui.horizontal_centered(|ui| {
                if let Some(p) = app.player.get() {
                    ui.label(RichText::new(&p.name).size(16.0).strong());
                    ui.label(RichText::new(&p.realm).color(theme::DIM));
                    ui.separator();
                    ui::num(ui, "灵石", p.stones, theme::GOLD);
                    ui.separator();
                    ui::num(ui, "功德", p.merit, theme::ACCENT);
                } else {
                    ui.label(RichText::new("（未载入角色）").color(theme::DIM));
                }
                ui.separator();

                if let Some(m) = app.world.get() {
                    let season = season_name(m.season.as_deref().unwrap_or(""));
                    let weather = weather_name(m.weather.as_deref().unwrap_or(""));
                    let shard = m.shard.map(|s| format!("{s} 线")).unwrap_or_default();
                    let mut bits: Vec<String> = Vec::new();
                    if !season.is_empty() {
                        bits.push(season.into());
                    }
                    if !weather.is_empty() {
                        bits.push(weather.into());
                    }
                    if !shard.is_empty() {
                        bits.push(shard);
                    }
                    bits.push(format!("在线 {}", app.presence));
                    ui.label(RichText::new(bits.join(" · ")).small().color(theme::DIM));
                }

                ui.separator();
                let (color, text) = conn_badge(&app.conn);
                ui.label(RichText::new("●").color(color).size(13.0));
                ui.label(RichText::new(text).color(color).small());
                ui.separator();
                let gear = ui
                    .add(egui::Button::new("设置").small())
                    .on_hover_text("界面与连接设置");
                if gear.clicked() {
                    st.page = Page::Settings;
                }
            });
        });
}

fn conn_badge(c: &ConnStatus) -> (egui::Color32, String) {
    match c {
        ConnStatus::Online => (theme::GOOD, "在线".into()),
        ConnStatus::Connecting => (theme::WARN, "连接中…".into()),
        ConnStatus::LoggingIn => (theme::WARN, "登录中…".into()),
        ConnStatus::Backoff { secs, attempt } => {
            (theme::WARN, format!("重连中 {secs}s（第 {attempt} 次）"))
        }
        ConnStatus::Idle => (theme::DIM, "未连接".into()),
        ConnStatus::Offline(msg) => (theme::BAD, msg.clone()),
        ConnStatus::Fatal(msg) => (theme::BAD, msg.clone()),
    }
}

fn nav(ctx: &egui::Context, st: &mut UiState) {
    egui::SidePanel::left("nav")
        .exact_width(68.0)
        .resizable(false)
        .show(ctx, |ui| {
            ui.add_space(8.0);
            for (i, page) in Page::all().iter().enumerate() {
                let active = st.page == *page;
                let label = RichText::new(page.label()).size(13.0).color(if active {
                    theme::ACCENT
                } else {
                    theme::TEXT
                });
                let resp = ui.add_sized(
                    [ui.available_width(), 40.0],
                    egui::Button::selectable(active, label),
                );
                if resp.clicked() {
                    st.page = *page;
                }
                if i < 8 {
                    let hint = match page {
                        Page::Game => "游戏",
                        Page::Character => "角色 · F1",
                        Page::Bag => "背包 · F2",
                        Page::Skills => "技能 · F3",
                        Page::Social => "社交 · F4",
                        Page::Market => "交易 · F5",
                        Page::Sect => "宗门 · F6",
                        Page::Map => "地图 · F7",
                        Page::Settings => "设置 · F8",
                    };
                    resp.on_hover_text(hint);
                }
                ui.add_space(2.0);
            }
        });
}

fn status_panel(ctx: &egui::Context, _st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    egui::SidePanel::right("status")
        .default_width(288.0)
        .width_range(240.0..=360.0)
        .show(ctx, |ui| {
            ui.add_space(6.0);
            let v = &app.vitals;
            let p = app.player.get();
            let max_hp = p.map(|p| p.max_hp).unwrap_or(0).max(1);
            let max_mp = p.map(|p| p.max_mp).unwrap_or(0).max(1);

            bar(ui, "气血", v.hp, max_hp, theme::GOOD);
            bar(ui, "真气", v.mp, max_mp, theme::ACCENT);
            bar(ui, "修为", v.exp, v.exp_need.max(1), theme::GOLD);
            ui.add_space(6.0);

            ui.horizontal(|ui| {
                ui.label(RichText::new("体力").small().color(theme::DIM));
                let stamina_max = p.map(|p| p.stamina_max).unwrap_or(200).max(1);
                let color = if percent(v.stamina, stamina_max) < 0.2 {
                    theme::BAD
                } else {
                    theme::TEXT
                };
                ui.label(RichText::new(format!("{} / {}", v.stamina, stamina_max)).color(color));
                if v.cultivating {
                    ui.label(RichText::new("· 打坐中").small().color(theme::ACCENT));
                }
            });

            if let Some(t) = p.and_then(|p| p.travel.as_ref()) {
                let left = t.arrive_at - app.server_now();
                ui.horizontal(|ui| {
                    ui.label(RichText::new("赶路").small().color(theme::DIM));
                    ui.label(
                        RichText::new(format!(
                            "{} · {}",
                            t.name,
                            crate::util::time::fmt_countdown(left)
                        ))
                        .color(theme::ACCENT),
                    );
                });
            }

            if let Some(p) = p {
                ui.add_space(6.0);
                ui.separator();
                ui.label(RichText::new("今日").small().color(theme::DIM));
                ui.horizontal_wrapped(|ui| {
                    ui.label(
                        RichText::new(format!("历练 {}", p.stats.explores))
                            .small()
                            .color(theme::TEXT),
                    );
                    ui.label(
                        RichText::new(format!("击杀 {}", p.stats.kills))
                            .small()
                            .color(theme::TEXT),
                    );
                    ui.label(
                        RichText::new(format!("突破 {}", p.stats.breaks))
                            .small()
                            .color(theme::TEXT),
                    );
                });
                if p.merit != 0 || p.lives != 0 {
                    ui.horizontal(|ui| {
                        ui.label(
                            RichText::new(format!("功德 {} · 轮回 {}", p.merit, p.lives))
                                .small()
                                .color(theme::DIM),
                        );
                    });
                }
            }

            ui.add_space(8.0);
            ui.separator();
            ui.add_space(4.0);

            let busy = matches!(app.conn, ConnStatus::Online).eq(&false);
            ui.horizontal(|ui| {
                let cult = v.cultivating;
                let b = ui.add_enabled(
                    !busy,
                    egui::Button::new(if cult { "停止打坐" } else { "打坐" }),
                );
                if b.clicked() {
                    out.push(Intent::Cultivate(!cult));
                }
                let b = ui.add_enabled(!busy, egui::Button::new("历练"));
                if b.clicked() {
                    out.push(Intent::Explore);
                }
                let b = ui.add_enabled(!busy, egui::Button::new("突破"));
                if b.clicked() {
                    out.push(Intent::Breakthrough);
                }
            });
            ui.horizontal(|ui| {
                let b = ui.add_enabled(!busy, egui::Button::new("看看四周"));
                if b.clicked() {
                    out.push(Intent::Look);
                }
                if ui.button("退出登录").clicked() {
                    out.push(Intent::Logout);
                }
            });

            ui.add_space(6.0);
            ui.separator();
            ui.label(RichText::new("快捷键").small().color(theme::DIM));
            ui.label(
                RichText::new("F1–F8 切页 · Space 历练 · C 打坐 · Enter 聊天")
                    .small()
                    .color(theme::DIM),
            );
        });
}

fn bar(ui: &mut egui::Ui, label: &str, cur: i64, max: i64, color: egui::Color32) {
    ui.horizontal(|ui| {
        ui.label(RichText::new(label).small().color(theme::DIM));
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            ui.label(
                RichText::new(format!("{}/{}", cur.max(0), max))
                    .small()
                    .color(theme::TEXT),
            );
        });
    });
    ui.add(
        egui::ProgressBar::new(percent(cur, max))
            .fill(color)
            .desired_height(8.0),
    );
    ui.add_space(2.0);
}

fn bottom_bar(ctx: &egui::Context, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    // 聊天与外发状态合并成一条底栏：两个 bottom panel 叠在一起时，
    // 上面那条的 available_width 会算成无穷，输入框会被布局跳过。
    egui::TopBottomPanel::bottom("bottom_bar")
        .exact_height(62.0)
        .show(ctx, |ui| {
            ui.add_space(2.0);
            ui.horizontal(|ui| {
                let ch_label = match st.chat_ch.as_str() {
                    "region" => "区域",
                    "party" => "队伍",
                    "dm" => "私聊",
                    _ => "世界",
                };
                egui::ComboBox::from_id_salt("chat_ch")
                    .selected_text(ch_label)
                    .width(66.0)
                    .show_ui(ui, |ui| {
                        ui.selectable_value(&mut st.chat_ch, "world".into(), "世界");
                        ui.selectable_value(&mut st.chat_ch, "region".into(), "区域");
                        ui.selectable_value(&mut st.chat_ch, "party".into(), "队伍");
                        ui.selectable_value(&mut st.chat_ch, "dm".into(), "私聊");
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
                            "说点什么…（Enter 发送）"
                        } else {
                            "未连接"
                        })
                        .char_limit(80),
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
                    }
                }
            });
            ui.add_space(2.0);
            hint_bar(ui, app);
        });
}

fn hint_bar(ui: &mut egui::Ui, app: &AppState) {
    ui.horizontal_centered(|ui| {
        ui.label(
            RichText::new(match &app.conn {
                ConnStatus::Online => "已连接 · 服务端权威",
                ConnStatus::Connecting | ConnStatus::LoggingIn => "连接中，动作已禁用",
                ConnStatus::Backoff { .. } => "重连中，动作已禁用",
                ConnStatus::Offline(m) => m.as_str(),
                ConnStatus::Fatal(m) => m.as_str(),
                ConnStatus::Idle => "未连接",
            })
            .small()
            .color(theme::DIM),
        );
    });
}

fn game_page(ui: &mut egui::Ui, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    let Some(m) = app.world.get() else {
        ui.add_space(24.0);
        ui.vertical_centered(|ui| {
            ui.label(RichText::new("正在读取地貌…").color(theme::DIM));
        });
        return;
    };

    egui::Frame::group(ui.style()).show(ui, |ui| {
        ui.set_width(ui.available_width());
        ui.horizontal(|ui| {
            ui.label(RichText::new(&m.name).size(18.0).color(theme::ACCENT));
            if !m.region_name.is_empty() {
                ui.label(RichText::new(&m.region_name).small().color(theme::DIM));
            }
            if let Some(g) = &m.grade {
                ui.label(RichText::new(g).small().color(theme::GOLD));
            }
            ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                ui.label(
                    RichText::new(format!(
                        "{} · 探索 {}",
                        terrain_name(&m.terrain),
                        m.explored
                    ))
                    .small()
                    .color(theme::DIM),
                );
            });
        });
        if !m.desc.is_empty() {
            ui.label(RichText::new(&m.desc).color(theme::TEXT));
        }
        if let Some(w) = &m.world {
            ui.label(
                RichText::new(format!("天象：{}", w.name))
                    .small()
                    .color(theme::WARN),
            );
        }
        if !m.npcs.is_empty() {
            ui.horizontal_wrapped(|ui| {
                ui.label(RichText::new("此地有人：").small().color(theme::DIM));
                for n in &m.npcs {
                    ui.label(
                        RichText::new(format!("{}（{}）", n.name, n.title))
                            .small()
                            .color(theme::TEXT),
                    );
                }
            });
        }
    });

    ui.add_space(6.0);
    ui.horizontal(|ui| {
        ui.label(RichText::new("出口").small().color(theme::DIM));
        ui.label(
            RichText::new(format!("{} 条", m.exits.len()))
                .small()
                .color(theme::DIM),
        );
    });

    ScrollArea::vertical()
        .id_salt("exits")
        .max_height(210.0)
        .auto_shrink([false, true])
        .show(ui, |ui| {
            if m.exits.is_empty() {
                ui.label(RichText::new("四面是崖，无路可走。").color(theme::DIM));
            }
            for (idx, ex) in m.exits.iter().enumerate() {
                let name = ex
                    .name
                    .clone()
                    .filter(|s| !s.is_empty())
                    .unwrap_or_else(|| "（雾中）".into());
                let blocked = ex.blocked == Some(true);
                let ok = ex.ok && !blocked;
                let selected = idx == st.selected_exit;
                let mut line = format!(
                    "{:>2}. {}    {} · {}s",
                    idx + 1,
                    name,
                    terrain_name(&ex.terrain),
                    ex.cost_sec
                );
                if let Some(fee) = ex.fee
                    && fee > 0
                {
                    line.push_str(&format!("   {fee} 灵石"));
                }
                if blocked {
                    let at = ex.until.map(crate::util::time::fmt_hms).unwrap_or_default();
                    line.push_str(&format!("   断路 · 约 {at} 通"));
                } else if let Some(cond) = &ex.cond
                    && !cond.is_empty()
                {
                    line.push_str(&format!("   {}", cond_text(cond)));
                }
                if ex.temp == Some(true) {
                    line.push_str("   临时");
                }
                let text = RichText::new(line).color(if ok { theme::TEXT } else { theme::DIM });
                let resp = ui.add_enabled(ok, egui::Button::selectable(selected, text));
                if resp.clicked() {
                    st.selected_exit = idx;
                    out.push(Intent::Move(ex.to.clone()));
                }
            }
        });

    ui.add_space(6.0);
    ui.separator();
    ui::logview::show(ui, st, app);
}

fn character_page(ui: &mut egui::Ui, app: &AppState) {
    let Some(p) = app.player.get() else {
        page_stub(ui, "角色", "尚未载入角色", "M1");
        return;
    };
    ui.label(RichText::new("角色").size(20.0));
    ui.add_space(6.0);
    ui.horizontal_wrapped(|ui| {
        stat(ui, "境界", &p.realm);
        stat(
            ui,
            "灵根",
            &p.root
                .as_ref()
                .map(|r| r.name.clone())
                .unwrap_or("—".into()),
        );
        stat(ui, "攻击", &p.atk.to_string());
        stat(ui, "防御", &p.def.to_string());
        stat(ui, "身法", &p.spd.to_string());
        stat(ui, "会心", &format!("{:.0}%", p.crit * 100.0));
        stat(ui, "战力", &p.power.to_string());
    });
    ui.add_space(10.0);
    ui.separator();
    ui.label(RichText::new("属性").color(theme::DIM).small());
    ui.horizontal_wrapped(|ui| {
        stat(ui, "体魄", &p.attrs.ti.to_string());
        stat(ui, "灵性", &p.attrs.ling.to_string());
        stat(ui, "身法", &p.attrs.feng.to_string());
        stat(ui, "护体", &p.attrs.hu.to_string());
        stat(ui, "神识", &p.attrs.shen.to_string());
        stat(ui, "未分配", &p.free.to_string());
    });
    ui.add_space(10.0);
    ui.separator();
    ui.label(RichText::new("名分").color(theme::DIM).small());
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
        ui.add_space(10.0);
        ui.separator();
        ui.label(RichText::new("任务").color(theme::DIM).small());
        for q in &p.quests {
            ui.horizontal(|ui| {
                ui.label(RichText::new(&q.desc).color(theme::TEXT));
                ui.label(
                    RichText::new(format!("{}/{}", q.prog, q.target))
                        .small()
                        .color(theme::DIM),
                );
                if q.claimed {
                    ui.label(RichText::new("已领").small().color(theme::DIM));
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
    ui.horizontal(|ui| {
        ui.label(RichText::new("背包").size(20.0));
        ui.label(
            RichText::new(format!("{} 件", p.bag.len()))
                .small()
                .color(theme::DIM),
        );
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            ui.label(RichText::new(format!("灵石 {}", thousands(p.stones))).color(theme::GOLD));
        });
    });
    ui.add_space(6.0);
    egui::Grid::new("bag_grid")
        .num_columns(4)
        .striped(true)
        .min_col_width(120.0)
        .show(ui, |ui| {
            ui.label(RichText::new("名称").small().color(theme::DIM));
            ui.label(RichText::new("数量").small().color(theme::DIM));
            ui.label(RichText::new("类型").small().color(theme::DIM));
            ui.label(RichText::new("售价").small().color(theme::DIM));
            ui.end_row();
            for it in &p.bag {
                ui.label(RichText::new(&it.name).color(theme::TEXT));
                ui.label(RichText::new(it.n.to_string()).color(theme::TEXT));
                ui.label(RichText::new(kind_name(&it.kind)).small().color(theme::DIM));
                ui.label(RichText::new(thousands(it.sell)).small().color(theme::GOLD));
                ui.end_row();
            }
        });
    if p.bag.is_empty() {
        ui.add_space(20.0);
        ui.vertical_centered(|ui| {
            ui.label(RichText::new("背包是空的。").color(theme::DIM));
        });
    }
}

fn skills_page(ui: &mut egui::Ui, app: &AppState) {
    let Some(p) = app.player.get() else {
        page_stub(ui, "技能", "尚未载入角色", "M1");
        return;
    };
    ui.label(RichText::new("技能").size(20.0));
    ui.add_space(4.0);
    ui.label(
        RichText::new(format!(
            "主修：{} · 功法 {} 门 · 技能槽 {}",
            if p.main_name.is_empty() {
                "—"
            } else {
                &p.main_name
            },
            p.manuals.len(),
            p.slots.len()
        ))
        .color(theme::DIM),
    );
    ui.add_space(6.0);
    egui::Grid::new("skill_grid")
        .num_columns(4)
        .striped(true)
        .min_col_width(120.0)
        .show(ui, |ui| {
            ui.label(RichText::new("技能").small().color(theme::DIM));
            ui.label(RichText::new("真气").small().color(theme::DIM));
            ui.label(RichText::new("威力").small().color(theme::DIM));
            ui.label(RichText::new("状态").small().color(theme::DIM));
            ui.end_row();
            for s in &p.skill_book {
                ui.label(RichText::new(&s.name).color(theme::TEXT));
                ui.label(RichText::new(s.mp.to_string()).color(theme::ACCENT));
                ui.label(RichText::new(format!("{:.1}", s.power)).color(theme::TEXT));
                let state = if s.equipped { "已装配" } else { "未装配" };
                ui.label(RichText::new(state).small().color(theme::DIM));
                ui.end_row();
            }
        });
}

fn sect_page(ui: &mut egui::Ui, app: &AppState) {
    let Some(p) = app.player.get() else {
        page_stub(ui, "宗门", "尚未载入角色", "M1");
        return;
    };
    ui.label(RichText::new("宗门").size(20.0));
    ui.add_space(6.0);
    if p.guild_name.is_empty() {
        ui.label(RichText::new("尚未加入玩家宗门。").color(theme::DIM));
        ui.add_space(4.0);
        ui.label(
            RichText::new("建宗、捐献、宗门战在 M4 落地。")
                .small()
                .color(theme::DIM),
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
    ui.add_space(8.0);
    ui.separator();
    ui.horizontal_wrapped(|ui| {
        stat(
            ui,
            "NPC 势力",
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

fn map_page(ui: &mut egui::Ui, app: &AppState) {
    let Some(m) = app.world.get() else {
        page_stub(ui, "地图", "尚未载入地貌", "M1");
        return;
    };
    ui.label(RichText::new("地图").size(20.0));
    ui.add_space(4.0);
    ui.label(
        RichText::new(format!("{} · {}", m.region_name, m.path.join(" / ")))
            .small()
            .color(theme::DIM),
    );
    ui.add_space(8.0);
    ui.label(RichText::new("相邻节点").small().color(theme::DIM));
    for ex in &m.exits {
        ui.horizontal(|ui| {
            ui.label(
                RichText::new(ex.name.clone().unwrap_or_else(|| "（雾中）".into()))
                    .color(if ex.ok { theme::TEXT } else { theme::DIM }),
            );
            ui.label(
                RichText::new(format!("{} · {}s", terrain_name(&ex.terrain), ex.cost_sec))
                    .small()
                    .color(theme::DIM),
            );
        });
    }
    ui.add_space(10.0);
    ui.label(
        RichText::new("节点拓扑图在 M5 用自绘画布实现；当前只列相邻关系。")
            .small()
            .color(theme::DIM),
    );
}

fn settings_page(ui: &mut egui::Ui, st: &mut UiState, app: &AppState, out: &mut Vec<Intent>) {
    ui.label(RichText::new("设置").size(20.0));
    ui.add_space(8.0);
    ui.horizontal(|ui| {
        ui.label(RichText::new("界面缩放").color(theme::TEXT));
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
    ui.checkbox(&mut st.show_timestamps, "日志显示时间");
    ui.add_space(8.0);
    ui.separator();
    ui.horizontal(|ui| {
        ui.label(RichText::new("服务器").small().color(theme::DIM));
        ui.label(RichText::new(&st.login.server).color(theme::TEXT));
    });
    ui.horizontal(|ui| {
        ui.label(RichText::new("字体").small().color(theme::DIM));
        ui.label(RichText::new(&st.font_source).color(theme::TEXT));
    });
    ui.horizontal(|ui| {
        ui.label(RichText::new("协议").small().color(theme::DIM));
        ui.label(
            RichText::new(format!(
                "v{} · 角色 {} · 日志 {} 行",
                crate::protocol::PROTO,
                if app.has_char { "已载入" } else { "无" },
                app.logs.len()
            ))
            .color(theme::TEXT),
        );
    });
    ui.add_space(12.0);
    if ui.button("退出登录").clicked() {
        out.push(Intent::Logout);
    }
}

fn page_stub(ui: &mut egui::Ui, title: &str, desc: &str, milestone: &str) {
    ui.label(RichText::new(title).size(20.0));
    ui.add_space(6.0);
    ui.label(RichText::new(desc).color(theme::DIM));
    ui.add_space(10.0);
    ui.label(
        RichText::new(format!(
            "该页在 {milestone} 落地；当前版本先保证主循环可玩。"
        ))
        .small()
        .color(theme::WARN),
    );
}

fn stat(ui: &mut egui::Ui, k: &str, v: &str) {
    egui::Frame::new()
        .fill(theme::PANEL)
        .inner_margin(egui::Margin::symmetric(8, 4))
        .corner_radius(4u8)
        .show(ui, |ui| {
            ui.vertical(|ui| {
                ui.label(RichText::new(k).small().color(theme::DIM));
                ui.label(RichText::new(v).color(theme::TEXT));
            });
        });
}

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
        "forest" => "密林",
        "bamboo" => "竹林",
        "river" => "河流",
        "lake" => "湖",
        "spring" => "泉",
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

fn _unused_player(_p: &PlayerView) {}
