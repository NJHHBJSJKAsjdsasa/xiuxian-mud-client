//! 舆图：已探索节点的拓扑图。
//!
//! 服务端不给全图坐标，所以这里画的是"走过的地方"：按 BFS 深度分层，
//! 当前节点居中，可拖动、滚轮缩放，点相邻节点直接走。

use egui::{Align2, Color32, FontId, Rect, Sense, Stroke, Ui, pos2, vec2};

use crate::store::mapgraph::{MapGraph, terrain_glyph};
use crate::ui::theme;

pub struct MapUiState {
    pub pan: egui::Vec2,
    pub zoom: f32,
    /// 只画当前节点几圈之内（避免整张图缩成一团）。
    pub max_depth: usize,
    /// 首次进入时把视图缩放到合适大小。
    fitted: bool,
}

impl Default for MapUiState {
    fn default() -> Self {
        Self {
            pan: egui::Vec2::ZERO,
            zoom: 1.0,
            max_depth: 2,
            fitted: false,
        }
    }
}

pub fn show(
    ui: &mut Ui,
    graph: &MapGraph,
    current: &str,
    st: &mut MapUiState,
) -> Option<(String, bool)> {
    if graph.is_empty() {
        ui.centered_and_justified(|ui| {
            ui.label(
                egui::RichText::new("还没有走过路，舆图上一片空白。")
                    .font(theme::serif_id(15.0))
                    .color(theme::DIM),
            );
        });
        return None;
    }

    let size = ui.available_size();
    let (rect, resp) = ui.allocate_exact_size(size, Sense::click_and_drag());
    if st.zoom <= 0.0 {
        st.zoom = 1.0;
    }
    let world = graph.layout(current);
    if !st.fitted {
        st.fitted = true;
        st.pan = egui::Vec2::ZERO;
        // 固定 1.0：眼前这圈邻居刚好铺开，远的节点拖过去看。
        // 自动 fit 一旦被"洞天/秘境"这类深处节点撑大，近处就只剩几个点。
        st.zoom = 1.0;
    }

    // 拖动平移
    if resp.dragged() {
        st.pan += resp.drag_delta();
    }

    // 滚轮缩放（以鼠标为中心）
    if resp.hovered() {
        let scroll = ui.ctx().input(|i| i.raw_scroll_delta.y);
        if scroll.abs() > 0.1 {
            let old = st.zoom;
            st.zoom = (st.zoom * (1.0 + scroll * 0.0018)).clamp(0.3, 2.4);
            if let Some(p) = ui.ctx().pointer_hover_pos() {
                let c = rect.center() + st.pan;
                st.pan += (p - c) * (1.0 - st.zoom / old);
            }
        }
    }

    let p = ui.painter_at(rect);
    p.rect_filled(rect, 3, theme::CARD);
    draw_grid(&p, rect, st);

    let center = rect.center() + st.pan;
    // 距离压缩：跨区域/洞天这类远处节点按 tanh 收进画面，方位保留、距离感略损。
    // 不做这一步，一条"去落霞谷口"的边就能把整张图拉成一团线。
    let cap = (rect.width().min(rect.height()) * 0.42).max(90.0);
    let squash = |w: egui::Pos2| -> egui::Vec2 {
        let d = w.to_vec2().length();
        if d < 1.0 {
            return egui::Vec2::ZERO;
        }
        let nd = cap * (d / (cap * 1.6)).tanh();
        w.to_vec2() / d * nd
    };
    let to_screen = |w: egui::Pos2| center + squash(w) * st.zoom;
    // 只画近处几圈；远处的记忆节点不挤在一屏里。
    let dist = graph.distances_from(current);
    let visible = |id: &str| dist.get(id).copied().unwrap_or(99) <= st.max_depth;

    // 边先画，节点压在上面。
    for ((a, b), e) in &graph.edges {
        if !visible(a) || !visible(b) {
            continue;
        }
        let (Some(pa), Some(pb)) = (world.get(a), world.get(b)) else {
            continue;
        };
        let (sa, sb) = (to_screen(*pa), to_screen(*pb));
        let color = if e.blocked {
            theme::BAD.linear_multiply(0.55)
        } else if e.ok {
            theme::LINE.linear_multiply(1.15)
        } else {
            theme::LINE.linear_multiply(0.6)
        };
        if e.ok && !e.blocked {
            p.line_segment([sa, sb], Stroke::new(1.2_f32, color));
        } else {
            dashed_line(&p, sa, sb, color);
        }
        if st.zoom > 0.75 {
            let mid = sa + (sb - sa) * 0.5;
            p.text(
                mid,
                Align2::CENTER_CENTER,
                format!("{}s", e.cost_sec),
                FontId::proportional(9.0),
                theme::FAINT,
            );
        }
    }

    // 命中检测用的屏幕距离
    let mut clicked: Option<(String, bool)> = None;
    if resp.clicked()
        && let Some(pos) = resp.interact_pointer_pos()
    {
        let mut best: Option<(f32, String, bool)> = None;
        for (id, w) in &world {
            let s = to_screen(*w);
            let d = (s - pos).length();
            if d < 18.0 {
                let walkable = graph
                    .neighbors_of(current)
                    .iter()
                    .any(|(n, e)| n == id && e.ok && !e.blocked);
                if best.as_ref().map(|(bd, _, _)| d < *bd).unwrap_or(true) {
                    best = Some((d, id.clone(), walkable));
                }
            }
        }
        if let Some((_, id, walkable)) = best {
            clicked = Some((id, walkable));
        }
    }

    for (id, w) in &world {
        if !visible(id) {
            continue;
        }
        let node = graph.nodes.get(id);
        let s = to_screen(*w);
        let is_cur = id == current;
        let visited = node.map(|n| n.visited).unwrap_or(false);
        let name = node
            .map(|n| n.name.clone())
            .filter(|s| !s.is_empty())
            .unwrap_or_else(|| "未名".into());
        let terrain = node.map(|n| n.terrain.as_str()).unwrap_or("");

        if is_cur {
            p.circle_filled(s, 16.0, theme::ACCENT.linear_multiply(0.18));
            p.circle_stroke(s, 16.0, Stroke::new(1.5_f32, theme::ACCENT));
            p.circle_filled(s, 5.5, theme::ACCENT);
        } else if visited {
            p.circle_filled(s, 12.0, theme::PANEL_HI);
            p.circle_stroke(s, 12.0, Stroke::new(1.2_f32, theme::LINE));
            p.circle_filled(s, 3.0, theme::DIM);
        } else {
            p.circle_stroke(s, 10.0, Stroke::new(1.0_f32, theme::FAINT));
        }

        let label_color = if is_cur {
            theme::ACCENT
        } else if visited {
            theme::TEXT
        } else {
            theme::FAINT
        };
        let label = if name.chars().count() > 7 {
            format!("{}…", name.chars().take(6).collect::<String>())
        } else {
            name
        };
        p.text(
            pos2(s.x, s.y + 19.0),
            Align2::CENTER_TOP,
            label,
            FontId::proportional(if is_cur { 13.0 } else { 11.5 }),
            label_color,
        );
        if !terrain.is_empty() && st.zoom > 0.65 {
            p.text(
                pos2(s.x + 14.0, s.y - 13.0),
                Align2::LEFT_BOTTOM,
                terrain_glyph(terrain),
                FontId::proportional(9.5),
                theme::FAINT,
            );
        }
    }

    // 图例与操作提示
    p.text(
        rect.left_bottom() + vec2(12.0, -10.0),
        Align2::LEFT_BOTTOM,
        "◎ 当前   ● 到过   ○ 只是望见",
        FontId::proportional(10.5),
        theme::FAINT,
    );
    p.text(
        rect.right_bottom() + vec2(-12.0, -10.0),
        Align2::RIGHT_BOTTOM,
        format!(
            "拖动平移 · 滚轮缩放 · 点相邻处前往   已记 {} 处 · 画到 {} 圈",
            graph.nodes.len(),
            st.max_depth
        ),
        FontId::proportional(10.5),
        theme::FAINT,
    );

    // 悬停提示：坐标反查太吵，这里只在指针停在节点上时改光标。
    if resp.hovered()
        && let Some(pos) = ui.ctx().pointer_hover_pos()
        && world
            .values()
            .any(|w| (to_screen(*w) - pos).length() < 14.0)
    {
        ui.ctx().set_cursor_icon(egui::CursorIcon::PointingHand);
    }

    clicked
}

fn draw_grid(p: &egui::Painter, rect: Rect, st: &MapUiState) {
    let step = 44.0 * st.zoom.clamp(0.5, 1.8);
    if step < 14.0 {
        return;
    }
    let color = Color32::from_rgba_unmultiplied(0x2A, 0x33, 0x3C, 28);
    let ox = (rect.left() + st.pan.x).rem_euclid(step);
    let oy = (rect.top() + st.pan.y).rem_euclid(step);
    let mut x = rect.left() + ox;
    while x < rect.right() {
        p.line_segment(
            [pos2(x, rect.top()), pos2(x, rect.bottom())],
            Stroke::new(1.0_f32, color),
        );
        x += step;
    }
    let mut y = rect.top() + oy;
    while y < rect.bottom() {
        p.line_segment(
            [pos2(rect.left(), y), pos2(rect.right(), y)],
            Stroke::new(1.0_f32, color),
        );
        y += step;
    }
}

fn dashed_line(p: &egui::Painter, a: egui::Pos2, b: egui::Pos2, color: Color32) {
    let v = b - a;
    let len = v.length();
    if len < 1.0 {
        return;
    }
    let dir = v / len;
    let mut t = 0.0;
    while t < len {
        let t2 = (t + 5.0).min(len);
        p.line_segment([a + dir * t, a + dir * t2], Stroke::new(1.0_f32, color));
        t += 9.0;
    }
}
