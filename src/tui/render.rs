//! 界面绘制：一屏一屏画，全量重绘（终端里无所谓）。
//!
//! 布局：场景在上，路标/对白/行动在下；数据页整屏覆盖。

use ratatui::Frame;
use ratatui::layout::{Constraint, Layout, Rect};
use ratatui::style::{Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Clear, Paragraph};

use crate::app::App;
use crate::protocol::msg::LogLine;
use crate::store::AppState;
use crate::tui::scene::{self, Cell, Pose};
use crate::tui::theme;
use crate::util::text::{self, percent, truncate_chars};
use crate::view::{Page, View};

pub fn draw(f: &mut Frame, app: &App) {
    match app.ui.view {
        View::Login => draw_login(f, app),
        View::Create => draw_create(f, app),
        View::Fatal => draw_fatal(f, app),
        View::Main => {
            if app.ui.page == Page::Game {
                draw_game(f, app);
            } else {
                draw_page(f, app);
            }
        }
    }
    draw_toast(f, app);
}

// ───────────────────────── 登录 / 建号 / 致命 ─────────────────────────

fn draw_login(f: &mut Frame, app: &App) {
    let area = centered(f.area(), 62, 16);
    let l = &app.ui.login;
    let mut lines = vec![
        Line::from(""),
        Line::from(Span::styled(
            "九 州 问 道",
            Style::default()
                .fg(theme::ACCENT)
                .add_modifier(Modifier::BOLD),
        )),
        Line::from(Span::styled(
            "云端存档 · 服务端权威",
            Style::default().fg(theme::DIM),
        )),
        Line::from(""),
        field("服务器", &l.server, l.focus == 0, "ws://127.0.0.1:8080"),
        field("道号", &l.name, l.focus == 1, "2–8 个字符"),
        field(
            "密码",
            &"*".repeat(l.pass.chars().count()),
            l.focus == 2,
            "6–32 个字符",
        ),
        Line::from(""),
    ];
    let mode = if l.register { "注册" } else { "登录" };
    let busy = if l.loading { "连接中…" } else { "" };
    lines.push(Line::from(vec![
        Span::styled(
            format!("  [ {mode} ]  "),
            Style::default().fg(theme::BG).bg(theme::ACCENT),
        ),
        Span::styled("  Ctrl+R 切登录/注册", Style::default().fg(theme::FAINT)),
        Span::styled(format!("   {busy}"), Style::default().fg(theme::WARN)),
    ]));
    lines.push(Line::from(""));
    if !l.error.is_empty() {
        lines.push(Line::from(Span::styled(
            format!("  ! {}", l.error),
            Style::default().fg(theme::BAD),
        )));
    } else {
        lines.push(Line::from(Span::styled(
            "  Tab 切换  Enter 提交  Ctrl+C 退出",
            Style::default().fg(theme::FAINT),
        )));
    }
    f.render_widget(Paragraph::new(lines), area);
}

fn field(label: &str, value: &str, focused: bool, hint: &str) -> Line<'static> {
    let cursor = if focused { "▌" } else { " " };
    let text = if value.is_empty() {
        Span::styled(hint.to_string(), Style::default().fg(theme::FAINT))
    } else {
        Span::styled(value.to_string(), Style::default().fg(theme::TEXT))
    };
    Line::from(vec![
        Span::styled(
            format!("  {label:<6}"),
            Style::default().fg(if focused { theme::ACCENT } else { theme::DIM }),
        ),
        Span::styled(format!("[{cursor}"), Style::default().fg(theme::FAINT)),
        text,
        Span::styled("]", Style::default().fg(theme::FAINT)),
    ])
}

fn draw_create(f: &mut Frame, app: &App) {
    let area = centered(f.area(), 56, 10);
    let name = &app.ui.create_name;
    let lines = vec![
        Line::from(""),
        Line::from(Span::styled(
            "踏 入 凡 界",
            Style::default()
                .fg(theme::ACCENT)
                .add_modifier(Modifier::BOLD),
        )),
        Line::from(Span::styled(
            "测灵石摆在案上，还空着。",
            Style::default().fg(theme::DIM),
        )),
        Line::from(""),
        field("道号", name, true, "2–8 个字符"),
        Line::from(""),
        Line::from(Span::styled(
            "  Enter 落笔  Ctrl+C 退出",
            Style::default().fg(theme::FAINT),
        )),
    ];
    f.render_widget(Paragraph::new(lines), area);
}

fn draw_fatal(f: &mut Frame, app: &App) {
    let area = centered(f.area(), 64, 8);
    let lines = vec![
        Line::from(""),
        Line::from(Span::styled(
            "无法进入游戏",
            Style::default().fg(theme::BAD).add_modifier(Modifier::BOLD),
        )),
        Line::from(""),
        Line::from(Span::styled(
            format!("  {}", app.ui.fatal),
            Style::default().fg(theme::TEXT),
        )),
        Line::from(Span::styled(
            format!("  本机协议 v{}", crate::protocol::PROTO),
            Style::default().fg(theme::FAINT),
        )),
        Line::from(""),
        Line::from(Span::styled("  q 退出", Style::default().fg(theme::FAINT))),
    ];
    f.render_widget(Paragraph::new(lines), area);
}

// ───────────────────────── 主界面 ─────────────────────────

fn draw_game(f: &mut Frame, app: &App) {
    let area = f.area();
    let rows = Layout::vertical([
        Constraint::Length(1), // 顶栏
        Constraint::Min(5),    // 场景
        Constraint::Length(2), // 目标
        Constraint::Length(1), // 路标
        Constraint::Length(7), // 见闻
        Constraint::Length(2), // 行动
        Constraint::Length(1), // 提示
    ])
    .split(area);

    draw_top(f, rows[0], app);

    // 场景
    let m = app.state.world.get();
    let (terrain, weather, daypart) = match m {
        Some(m) => (
            m.terrain.as_str(),
            m.weather.as_deref().unwrap_or(""),
            m.daypart.as_deref().unwrap_or(""),
        ),
        None => ("plain", "", ""),
    };
    let pose = if app.state.vitals.cultivating {
        Pose::Sitting
    } else if app
        .state
        .player
        .get()
        .and_then(|p| p.travel.as_ref())
        .is_some()
    {
        Pose::Walking
    } else {
        Pose::Standing
    };
    let s = scene::Scene {
        terrain,
        weather,
        daypart,
        pose,
    };
    let grid = scene::build(rows[1].width, rows[1].height, &s);
    f.render_widget(Paragraph::new(grid_lines(&grid)), rows[1]);

    draw_objective(f, rows[2], app);
    draw_exits(f, rows[3], app);
    draw_logs(f, rows[4], app);
    draw_actions(f, rows[5], app);
    draw_hint(f, rows[6], app);
}

fn draw_top(f: &mut Frame, area: Rect, app: &App) {
    let (place, region) = match app.state.world.get() {
        Some(m) => (m.name.clone(), m.region_name.clone()),
        None => ("雾中".into(), String::new()),
    };
    let mut spans = vec![
        Span::styled(
            format!(" {place}"),
            Style::default()
                .fg(theme::TEXT)
                .add_modifier(Modifier::BOLD),
        ),
        Span::styled(format!(" {region}"), Style::default().fg(theme::FAINT)),
    ];
    if let Some(m) = app.state.world.get() {
        let season = season_name(m.season.as_deref().unwrap_or(""));
        let weather = weather_name(m.weather.as_deref().unwrap_or(""));
        let bits: Vec<&str> = [season, weather]
            .into_iter()
            .filter(|x| !x.is_empty())
            .collect();
        if !bits.is_empty() {
            spans.push(Span::styled(
                format!("   {}", bits.join("·")),
                Style::default().fg(theme::DIM),
            ));
        }
    }
    let (cc, ct) = conn_badge(app);
    spans.push(Span::styled(format!("   ● {ct}"), Style::default().fg(cc)));
    spans.push(Span::styled(
        format!("  在线 {}", app.state.presence),
        Style::default().fg(theme::FAINT),
    ));

    let right = match app.state.player.get() {
        Some(p) => format!(
            "{} · {}   灵石 {}   功德 {} ",
            p.name,
            p.realm,
            crate::util::text::thousands(p.stones),
            p.merit
        ),
        None => String::new(),
    };
    f.render_widget(Paragraph::new(padded_row(spans, &right, area.width)), area);
}

fn draw_objective(f: &mut Frame, area: Rect, app: &App) {
    let (title, line) = objective(app);
    let lines = vec![
        Line::from(Span::styled(
            format!(" > {title}"),
            Style::default().fg(theme::GOLD),
        )),
        Line::from(Span::styled(
            format!(
                "   {}",
                truncate_chars(&line, (area.width as usize).saturating_sub(4))
            ),
            Style::default().fg(theme::TEXT),
        )),
    ];
    f.render_widget(Paragraph::new(lines), area);
}

fn draw_exits(f: &mut Frame, area: Rect, app: &App) {
    let Some(m) = app.state.world.get() else {
        return;
    };
    let traveling = app
        .state
        .player
        .get()
        .map(|p| p.travel.is_some())
        .unwrap_or(false);
    let mut spans: Vec<Span> = vec![Span::styled(" 路 ", Style::default().fg(theme::GOLD))];
    let mut used = 3usize;
    let budget = area.width as usize;
    for (i, ex) in m.exits.iter().enumerate() {
        // 有坐标就画方位箭头；没有就只写耗时，不留一个看不出意思的"·"。
        let arrow = match (m.x, m.y, ex.x, ex.y) {
            (Some(cx), Some(cy), Some(tx), Some(ty)) => theme::bearing_arrow(tx - cx, ty - cy),
            _ => "",
        };
        // 雾里望不见地名时用目标地形顶替（原/坡/林/水/镇），只给一个字，
        // 不至于一排看不出差别的耗时。
        let label = match ex.name.as_deref().filter(|s| !s.is_empty()) {
            Some(name) => format!("{arrow}{} {}s", truncate_chars(name, 5), ex.cost_sec),
            None => format!("{arrow}{} {}s", terrain_word(&ex.terrain), ex.cost_sec),
        };
        let chip = format!("[{label}]");
        let w = text::width(&chip) + 1;
        if used + w > budget.saturating_sub(2) {
            spans.push(Span::styled("…", Style::default().fg(theme::FAINT)));
            break;
        }
        used += w;
        let ok = ex.ok && !traveling;
        let style = if i == app.ui.selected_exit && ok {
            Style::default().fg(theme::BG).bg(theme::ACCENT)
        } else if ok {
            Style::default().fg(theme::TEXT)
        } else {
            Style::default().fg(theme::FAINT)
        };
        spans.push(Span::styled(chip, style));
        spans.push(Span::raw(" "));
    }
    f.render_widget(Paragraph::new(Line::from(spans)), area);
}

fn draw_logs(f: &mut Frame, area: Rect, app: &App) {
    let n = app.state.logs.len();
    let rows = area.height as usize;
    let end = n.saturating_sub(app.log_back);
    let start = end.saturating_sub(rows);
    let mut lines: Vec<Line> = Vec::with_capacity(rows);
    for i in start..end {
        if let Some(e) = app.state.logs.get(i) {
            lines.push(log_line(e, area.width));
        }
    }
    while lines.len() < rows {
        lines.push(Line::from(""));
    }
    f.render_widget(Paragraph::new(lines), area);
}

fn log_line(e: &crate::store::logbuf::Entry, width: u16) -> Line<'static> {
    let color = theme::tone(&e.line.k);
    let text = truncate_chars(&e.line.s, (width as usize).saturating_sub(4));
    Line::from(vec![
        Span::styled(" • ", Style::default().fg(color)),
        Span::styled(text, Style::default().fg(color)),
    ])
}

fn draw_actions(f: &mut Frame, area: Rect, app: &App) {
    let cult = app.state.vitals.cultivating;
    let busy = !matches!(app.state.conn, crate::net::ConnStatus::Online);
    let dim = Style::default().fg(if busy { theme::FAINT } else { theme::TEXT });
    let hot = Style::default()
        .fg(theme::ACCENT)
        .add_modifier(Modifier::BOLD);
    let buttons = Line::from(vec![
        Span::styled("  [ 历练 Space ]", if busy { dim } else { hot }),
        Span::styled(
            format!(" [ {} C ]", if cult { "收功" } else { "打坐" }),
            dim,
        ),
        Span::styled(" [ 突破 B ]", dim),
        Span::styled(" [ 看看 L ]", dim),
    ]);

    let p = app.state.player.get();
    let max_hp = p.map(|p| p.max_hp).unwrap_or(1).max(1);
    let max_mp = p.map(|p| p.max_mp).unwrap_or(1).max(1);
    let stamina_max = p.map(|p| p.stamina_max).unwrap_or(200).max(1);
    let v = &app.state.vitals;
    let stats = Line::from(vec![
        Span::styled(
            format!("  气血{} {}/{}", theme::bar(v.hp, max_hp, 10), v.hp, max_hp),
            Style::default().fg(theme::BAD),
        ),
        Span::styled(
            format!("  真气{} {}/{}", theme::bar(v.mp, max_mp, 10), v.mp, max_mp),
            Style::default().fg(theme::ACCENT),
        ),
        Span::styled(
            format!(
                "  修为{} {}/{}",
                theme::bar(v.exp, v.exp_need.max(1), 10),
                v.exp,
                v.exp_need
            ),
            Style::default().fg(theme::GOLD),
        ),
        Span::styled(
            format!("  体力 {}/{}", v.stamina, stamina_max),
            Style::default().fg(if percent(v.stamina, stamina_max) < 0.2 {
                theme::BAD
            } else {
                theme::DIM
            }),
        ),
    ]);

    f.render_widget(Paragraph::new(vec![buttons, stats]), area);
}

fn draw_hint(f: &mut Frame, area: Rect, app: &App) {
    let (text, color) = if app.ui.typing {
        let ch = match app.ui.chat_ch.as_str() {
            "region" => "区域",
            "party" => "队伍",
            "dm" => "私语",
            _ => "世界",
        };
        (
            format!(
                " {ch}> {}▌   Enter 送出  Tab 换频道  Esc 取消",
                app.ui.chat_text
            ),
            theme::ACCENT,
        )
    } else {
        (
            " / 说话   ↑↓ 选路  Enter 前往   Space 历练   c 打坐   b 突破   PgUp 见闻   F1–F8 页面   q 退出"
                .to_string(),
            theme::FAINT,
        )
    };
    f.render_widget(
        Paragraph::new(Line::from(Span::styled(text, Style::default().fg(color)))),
        area,
    );
}

// ───────────────────────── 数据页 ─────────────────────────

fn draw_page(f: &mut Frame, app: &App) {
    let area = f.area();
    let rows = Layout::vertical([
        Constraint::Length(1),
        Constraint::Min(3),
        Constraint::Length(1),
    ])
    .split(area);
    f.render_widget(
        Paragraph::new(Line::from(vec![
            Span::styled(
                format!(" {}", app.ui.page.label()),
                Style::default()
                    .fg(theme::TEXT)
                    .add_modifier(Modifier::BOLD),
            ),
            Span::styled("   Esc 返回", Style::default().fg(theme::FAINT)),
        ])),
        rows[0],
    );
    match app.ui.page {
        Page::Map => draw_map(f, rows[1], app),
        Page::Character => draw_character(f, rows[1], app),
        Page::Bag => draw_bag(f, rows[1], app),
        Page::Skills => draw_skills(f, rows[1], app),
        Page::Settings => draw_settings(f, rows[1], app),
        Page::Sect => draw_sect(f, rows[1], app),
        Page::Social | Page::Market => {
            let (name, desc, m) = if app.ui.page == Page::Social {
                ("江湖", "好友 · 师徒 · 道侣 · 仇敌 · 盟约 · 队伍", "M3")
            } else {
                ("坊市", "挂单 · 我的 · 成交 · 曲线 · 拍卖 · 书信", "M3")
            };
            f.render_widget(
                Paragraph::new(vec![
                    Line::from(""),
                    Line::from(Span::styled(
                        format!("  {name}"),
                        Style::default().fg(theme::GOLD),
                    )),
                    Line::from(Span::styled(
                        format!("  {desc}"),
                        Style::default().fg(theme::DIM),
                    )),
                    Line::from(""),
                    Line::from(Span::styled(
                        format!("  此处在 {m} 开张"),
                        Style::default().fg(theme::FAINT),
                    )),
                ]),
                rows[1],
            );
        }
        Page::Game => {}
    }
    f.render_widget(
        Paragraph::new(Line::from(Span::styled(
            " Esc 返回    PgUp/PgDn 翻页",
            Style::default().fg(theme::FAINT),
        ))),
        rows[2],
    );
}

fn draw_character(f: &mut Frame, area: Rect, app: &App) {
    let Some(p) = app.state.player.get() else {
        return;
    };
    let lines = vec![
        Line::from(""),
        kv("境界", &p.realm),
        kv(
            "灵根",
            &p.root
                .as_ref()
                .map(|r| r.name.clone())
                .unwrap_or("—".into()),
        ),
        kv("攻击 / 防御", &format!("{} / {}", p.atk, p.def)),
        kv(
            "身法 / 会心",
            &format!("{} / {:.0}%", p.spd, p.crit * 100.0),
        ),
        kv("战力", &p.power.to_string()),
        Line::from(""),
        kv(
            "根骨",
            &format!(
                "体魄 {}  灵性 {}  身法 {}  护体 {}  神识 {}  未分配 {}",
                p.attrs.ti, p.attrs.ling, p.attrs.feng, p.attrs.hu, p.attrs.shen, p.free
            ),
        ),
        Line::from(""),
        kv(
            "名分",
            &format!(
                "势力 {}  宗门 {}  职位 {}",
                if p.faction_name.is_empty() {
                    "—"
                } else {
                    &p.faction_name
                },
                if p.guild_name.is_empty() {
                    "—"
                } else {
                    &p.guild_name
                },
                if p.guild_rank.is_empty() {
                    "—"
                } else {
                    &p.guild_rank
                },
            ),
        ),
        kv("功德 / 轮回", &format!("{} / {}", p.merit, p.lives)),
        kv("丹毒", &format!("{}（{}）", p.toxin, p.toxin_note)),
    ];
    f.render_widget(Paragraph::new(lines), area);
}

fn draw_bag(f: &mut Frame, area: Rect, app: &App) {
    let Some(p) = app.state.player.get() else {
        return;
    };
    let mut lines = vec![
        Line::from(Span::styled(
            format!("  {} 件   灵石 {}", p.bag.len(), p.stones),
            Style::default().fg(theme::DIM),
        )),
        Line::from(Span::styled(
            format!("  {:<16}{:<6}{:<8}{}", "名称", "数量", "类别", "售价"),
            Style::default().fg(theme::FAINT),
        )),
    ];
    for it in &p.bag {
        lines.push(Line::from(Span::styled(
            format!(
                "  {:<16}{:<6}{:<8}{}",
                pad_to(&it.name, 16),
                it.n,
                pad_to(kind_name(&it.kind), 8),
                it.sell
            ),
            Style::default().fg(theme::TEXT),
        )));
    }
    if p.bag.is_empty() {
        lines.push(Line::from(Span::styled(
            "  行囊空空。",
            Style::default().fg(theme::DIM),
        )));
    }
    f.render_widget(Paragraph::new(lines), area);
}

fn draw_skills(f: &mut Frame, area: Rect, app: &App) {
    let Some(p) = app.state.player.get() else {
        return;
    };
    let mut lines = vec![
        Line::from(Span::styled(
            format!(
                "  主修 {}   功法 {} 门   技能槽 {}",
                if p.main_name.is_empty() {
                    "—"
                } else {
                    &p.main_name
                },
                p.manuals.len(),
                p.slots.len()
            ),
            Style::default().fg(theme::DIM),
        )),
        Line::from(Span::styled(
            format!("  {:<14}{:<6}{:<8}{}", "技能", "真气", "威力", "状态"),
            Style::default().fg(theme::FAINT),
        )),
    ];
    for s in &p.skill_book {
        lines.push(Line::from(Span::styled(
            format!(
                "  {:<14}{:<6}{:<8}{}",
                pad_to(&s.name, 14),
                s.mp,
                format!("{:.1}", s.power),
                if s.equipped { "已装配" } else { "未装配" }
            ),
            Style::default().fg(if s.equipped {
                theme::ACCENT
            } else {
                theme::TEXT
            }),
        )));
    }
    f.render_widget(Paragraph::new(lines), area);
}

fn draw_sect(f: &mut Frame, area: Rect, app: &App) {
    let Some(p) = app.state.player.get() else {
        return;
    };
    let lines = vec![
        Line::from(""),
        kv(
            "玩家宗门",
            if p.guild_name.is_empty() {
                "散修"
            } else {
                &p.guild_name
            },
        ),
        kv(
            "职位 / 贡献",
            &format!(
                "{} / {}",
                if p.guild_rank.is_empty() {
                    "—"
                } else {
                    &p.guild_rank
                },
                p.guild_contrib
            ),
        ),
        Line::from(""),
        kv(
            "NPC 势力",
            if p.faction_name.is_empty() {
                "—"
            } else {
                &p.faction_name
            },
        ),
        kv("声望", &format!("{}（{}）", p.fame, p.fame_tier)),
        kv("势力贡献", &p.contrib.to_string()),
        Line::from(""),
        Line::from(Span::styled(
            "  建宗、捐献、宗门战在 M4 落地。",
            Style::default().fg(theme::FAINT),
        )),
    ];
    f.render_widget(Paragraph::new(lines), area);
}

fn draw_settings(f: &mut Frame, area: Rect, app: &App) {
    let lines = vec![
        Line::from(""),
        kv("服务器", &app.cfg.server),
        kv(
            "协议",
            &format!(
                "v{}   角色 {}   日志 {} 行",
                crate::protocol::PROTO,
                if app.state.has_char {
                    "已载入"
                } else {
                    "无"
                },
                app.state.logs.len()
            ),
        ),
        kv("配置", &crate::config::Config::path().display().to_string()),
        Line::from(""),
        Line::from(Span::styled(
            "  退出登录：Esc 回主界面后按 q（仅退出客户端）",
            Style::default().fg(theme::FAINT),
        )),
    ];
    f.render_widget(Paragraph::new(lines), area);
}

/// 舆图：把世界坐标映射到字符网格，节点用 ◎ / ● / ○，连线用 ·。
fn draw_map(f: &mut Frame, area: Rect, app: &App) {
    let Some(m) = app.state.world.get() else {
        return;
    };
    let graph = &app.state.map_graph;
    if graph.is_empty() {
        f.render_widget(
            Paragraph::new(Span::styled(
                "  还没有走过路，舆图上一片空白。",
                Style::default().fg(theme::DIM),
            )),
            area,
        );
        return;
    }
    let w = area.width as i32;
    let h = area.height as i32;
    if w < 10 || h < 5 {
        return;
    }
    // 字符宽高比约 1:2，纵向乘 2 才不会压扁。
    let layout = graph.layout(&m.loc);
    let maxr = layout
        .values()
        .map(|(x, y)| (x * x + y * y).sqrt())
        .fold(1.0f32, f32::max);
    let scale = ((w.min(h * 2) as f32) * 0.42 / maxr).max(0.02);
    let cx = w / 2;
    let cy = h / 2;

    let mut grid = vec![vec![Cell::default(); w as usize]; h as usize];
    let pos = |x: f32, y: f32| -> (i32, i32) {
        (
            cx + (x * scale).round() as i32,
            cy + (y * scale * 0.5).round() as i32,
        )
    };

    // 边
    for ((a, b), e) in &graph.edges {
        let (Some(pa), Some(pb)) = (layout.get(a), layout.get(b)) else {
            continue;
        };
        let (x0, y0) = pos(pa.0, pa.1);
        let (x1, y1) = pos(pb.0, pb.1);
        let steps = ((x1 - x0).abs() + (y1 - y0).abs()).max(1);
        for i in 0..=steps {
            let t = i as f32 / steps as f32;
            let x = x0 + ((x1 - x0) as f32 * t).round() as i32;
            let y = y0 + ((y1 - y0) as f32 * t).round() as i32;
            if x > 0 && x < w - 1 && y > 0 && y < h - 1 && grid[y as usize][x as usize].ch == ' ' {
                grid[y as usize][x as usize] = Cell {
                    ch: if e.ok { '·' } else { 'x' },
                    fg: if e.ok { theme::FAINT } else { theme::BAD },
                };
            }
        }
    }

    // 节点
    for (id, (x, y)) in &layout {
        let (gx, gy) = pos(*x, *y);
        if gx <= 0 || gx >= w - 1 || gy <= 0 || gy >= h - 1 {
            continue;
        }
        let node = graph.nodes.get(id);
        let visited = node.map(|n| n.visited).unwrap_or(false);
        let is_cur = id == &m.loc;
        let (ch, fg) = if is_cur {
            ('◎', theme::ACCENT)
        } else if visited {
            ('●', theme::DIM)
        } else {
            ('○', theme::FAINT)
        };
        grid[gy as usize][gx as usize] = Cell { ch, fg };

        // 名字写在节点下方。没名字的地方（雾里望见的）不画字：
        // 一圈"未名"比留白更难看，符号本身已经说明那里有路。
        let Some(name) = node.map(|n| n.name.clone()).filter(|s| !s.is_empty()) else {
            continue;
        };
        let label = truncate_chars(&name, 6);
        let x0 = gx - (text::width(&label) as i32) / 2;
        let y = gy + 1;
        let mut x = x0;
        for c in label.chars() {
            let cw = text::char_width(c) as i32;
            if y < h && x >= 0 && x + cw <= w && label_cell_free(&grid, y, x) {
                grid[y as usize][x as usize] = Cell {
                    ch: c,
                    fg: if is_cur { theme::ACCENT } else { theme::DIM },
                };
                // 中文占两列：后一格标记为"续格"，渲染时跳过，否则会被当成空格。
                if cw == 2 && x + 1 < w {
                    grid[y as usize][(x + 1) as usize] = Cell {
                        ch: '\0',
                        fg: if is_cur { theme::ACCENT } else { theme::DIM },
                    };
                }
            }
            x += cw;
        }
    }

    f.render_widget(Paragraph::new(grid_lines(&grid)), area);
    // 图例
    let legend = format!(
        " ◎ 当前  ● 到过  ○ 只是望见   已记 {} 处",
        graph.nodes.len()
    );
    let y = area.bottom().saturating_sub(1);
    f.render_widget(
        Paragraph::new(Line::from(Span::styled(
            legend,
            Style::default().fg(theme::FAINT),
        ))),
        Rect {
            x: area.x,
            y,
            width: area.width,
            height: 1,
        },
    );
}

// ───────────────────────── 辅助 ─────────────────────────

fn draw_toast(f: &mut Frame, app: &App) {
    let Some(t) = app.ui.active_toast() else {
        return;
    };
    let area = f.area();
    // 宽度按显示列算：中文占两列，按字符数算会把文字挤出屏幕右缘。
    let w = (text::width(&t.text) as u16 + 6).min(area.width);
    let rect = Rect {
        x: area.right().saturating_sub(w),
        y: area.y + 1,
        width: w,
        height: 1,
    };
    f.render_widget(Clear, rect);
    f.render_widget(
        Paragraph::new(Line::from(Span::styled(
            format!(" ! {}", text::truncate_width(&t.text, w as usize - 4)),
            Style::default()
                .fg(if t.bad { theme::BAD } else { theme::ACCENT })
                .bg(theme::PANEL),
        ))),
        rect,
    );
}

fn objective(app: &App) -> (String, String) {
    if let Some(t) = app.state.player.get().and_then(|p| p.travel.as_ref()) {
        return (
            "赶路中".into(),
            format!(
                "前往 {} · 剩 {}",
                t.name,
                crate::util::time::fmt_countdown(t.arrive_at - app.state.server_now())
            ),
        );
    }
    if app.state.vitals.cultivating {
        return ("打坐中".into(), "真气在慢慢回，按 c 起身".into());
    }
    if app.state.vitals.stamina < 30 {
        return ("体力不济".into(), "坐下调息，或回城歇脚".into());
    }
    let explored = app.state.world.get().map(|m| m.explored).unwrap_or(0);
    if explored < 3 {
        return ("摸清这条路".into(), "按 ↑↓ 选路标，Enter 就走".into());
    }
    ("寻个去处".into(), "去没走过的地方看看".into())
}

fn conn_badge(app: &App) -> (ratatui::style::Color, String) {
    use crate::net::ConnStatus;
    match &app.state.conn {
        ConnStatus::Online => (theme::GOOD, "在线".into()),
        ConnStatus::Connecting => (theme::WARN, "连接中".into()),
        ConnStatus::LoggingIn => (theme::WARN, "登录中".into()),
        ConnStatus::Backoff { secs, .. } => (theme::WARN, format!("重连 {secs}s")),
        ConnStatus::Idle => (theme::FAINT, "未连接".into()),
        ConnStatus::Offline(m) => (theme::BAD, m.clone()),
        ConnStatus::Fatal(m) => (theme::BAD, m.clone()),
    }
}

/// 地形一字简称：路标看不到地名时用它顶替。
fn terrain_word(t: &str) -> &'static str {
    match t {
        "water" => "水",
        "woods" | "forest" => "林",
        "slope" | "cliff" => "坡",
        "town" | "city" => "镇",
        "plain" => "原",
        _ => "路",
    }
}

/// 舆图上的字能不能落在这一格：空白与连线可以让给名字，节点符号与别的名字不行。
fn label_cell_free(grid: &[Vec<Cell>], y: i32, x: i32) -> bool {
    match grid.get(y as usize).and_then(|row| row.get(x as usize)) {
        Some(c) => matches!(c.ch, ' ' | '·' | 'x'),
        None => false,
    }
}

fn grid_lines(grid: &[Vec<Cell>]) -> Vec<Line<'static>> {
    let mut out = Vec::with_capacity(grid.len());
    for row in grid {
        let mut spans: Vec<Span> = Vec::new();
        let mut cur: Option<(String, ratatui::style::Color)> = None;
        for c in row {
            if c.ch == '\0' {
                continue; // 宽字符的续格：字已经由前一格占了两列。
            }
            match &mut cur {
                Some((s, col)) if *col == c.fg => s.push(c.ch),
                Some((s, col)) => {
                    spans.push(Span::styled(std::mem::take(s), Style::default().fg(*col)));
                    cur = Some((c.ch.to_string(), c.fg));
                }
                None => cur = Some((c.ch.to_string(), c.fg)),
            }
        }
        if let Some((s, col)) = cur {
            spans.push(Span::styled(s, Style::default().fg(col)));
        }
        out.push(Line::from(spans));
    }
    out
}

/// 左右对齐的一行：左侧 spans + 右侧文字。
///
/// 右侧按显示宽度算，并截到剩余列数，避免长道号把顶栏挤出屏幕右缘。
fn padded_row(left: Vec<Span<'static>>, right: &str, width: u16) -> Line<'static> {
    let lw: usize = left.iter().map(|s| s.width()).sum();
    let room = (width as usize).saturating_sub(lw + 2);
    let right = text::truncate_width(right, room);
    let rw = text::width(&right);
    let pad = (width as usize).saturating_sub(lw + rw);
    let mut spans = left;
    spans.push(Span::raw(" ".repeat(pad)));
    spans.push(Span::styled(
        right.to_string(),
        Style::default().fg(theme::DIM),
    ));
    Line::from(spans)
}

fn kv(k: &str, v: &str) -> Line<'static> {
    Line::from(vec![
        Span::styled(
            format!("  {}", pad_to(k, 12)),
            Style::default().fg(theme::FAINT),
        ),
        Span::styled(v.to_string(), Style::default().fg(theme::TEXT)),
    ])
}

/// 按显示宽度补空格（中文占 2 列）。
fn pad_to(s: &str, width: usize) -> String {
    text::pad_to(s, width)
}

fn centered(area: Rect, w: u16, h: u16) -> Rect {
    let w = w.min(area.width);
    let h = h.min(area.height);
    Rect {
        x: area.x + (area.width - w) / 2,
        y: area.y + (area.height - h) / 2,
        width: w,
        height: h,
    }
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

#[allow(dead_code)]
fn _unused(_: &AppState, _: LogLine) {}
