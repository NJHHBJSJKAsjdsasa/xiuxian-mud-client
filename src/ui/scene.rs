//! 场景区：程序化水墨山水。
//!
//! 不用贴图：按地形与天气现画，远山三层 + 地形剪影 + 雾带 + 天气粒子。
//! 目的是让"我在哪"一眼可见，而不是读一行地名。

use egui::{Align2, Color32, FontId, Mesh, Pos2, Rect, Shape, Stroke, Ui, pos2, vec2};

use crate::ui::theme;

pub struct Scene<'a> {
    pub name: &'a str,
    pub region: &'a str,
    pub terrain: &'a str,
    pub weather: &'a str,
    pub daypart: &'a str,
    pub grade: Option<&'a str>,
    pub desc: &'a str,
    pub explored: i32,
}

/// 姿态：决定场景里那个人影的姿势。
#[derive(Clone, Copy, PartialEq, Eq, Default)]
pub enum Pose {
    #[default]
    Standing,
    Sitting,
    Walking,
}

/// 在给定矩形里画一整幅场景（不占布局，由调用方叠加 UI）。
pub fn paint(ui: &Ui, rect: Rect, s: &Scene<'_>, pose: Pose) {
    let p = ui.painter_at(rect);
    let (sky_top, sky_bot) = sky_colors(s.daypart, s.weather);

    // 天空：24 条横带，够平滑且比 mesh 好调。
    let bands = 24;
    for i in 0..bands {
        let t0 = i as f32 / bands as f32;
        let t1 = (i + 1) as f32 / bands as f32;
        let c = theme::mix(sky_top, sky_bot, (t0 + t1) * 0.5);
        let y0 = rect.top() + rect.height() * t0;
        let y1 = rect.top() + rect.height() * t1 + 1.0;
        p.rect_filled(
            Rect::from_min_max(pos2(rect.left(), y0), pos2(rect.right(), y1)),
            0,
            c,
        );
    }

    draw_celestial(&p, rect, s.daypart, s.weather);

    // 远山三层：越远越淡，轮廓低。
    let base = rect.top() + rect.height() * 0.74;
    draw_ridge(
        &p,
        rect,
        base - rect.height() * 0.10,
        rect.height() * 0.22,
        0.7,
        theme::mix(sky_bot, Color32::from_rgb(0x0C, 0x12, 0x16), 0.55),
    );
    draw_ridge(
        &p,
        rect,
        base + rect.height() * 0.02,
        rect.height() * 0.16,
        2.3,
        theme::mix(sky_bot, Color32::from_rgb(0x08, 0x0C, 0x10), 0.75),
    );
    draw_ridge(
        &p,
        rect,
        base + rect.height() * 0.12,
        rect.height() * 0.10,
        4.1,
        theme::mix(sky_bot, Color32::from_rgb(0x04, 0x06, 0x08), 0.88),
    );

    draw_terrain(&p, rect, s.terrain);
    draw_path(&p, rect);
    draw_figure(&p, rect, pose);
    draw_weather(&p, rect, s.weather);
    draw_frame(&p, rect);
    draw_caption(&p, rect, s);
}

fn sky_colors(daypart: &str, weather: &str) -> (Color32, Color32) {
    let (top, bot) = match daypart {
        "dawn" => (
            Color32::from_rgb(0x1B, 0x24, 0x2E),
            Color32::from_rgb(0x6E, 0x5A, 0x45),
        ),
        "day" => (
            Color32::from_rgb(0x24, 0x30, 0x3A),
            Color32::from_rgb(0x5F, 0x6B, 0x63),
        ),
        "dusk" => (
            Color32::from_rgb(0x1E, 0x1B, 0x28),
            Color32::from_rgb(0x6B, 0x45, 0x38),
        ),
        "night" => (
            Color32::from_rgb(0x0A, 0x0D, 0x12),
            Color32::from_rgb(0x16, 0x22, 0x2C),
        ),
        _ => (
            Color32::from_rgb(0x18, 0x1F, 0x26),
            Color32::from_rgb(0x3A, 0x45, 0x46),
        ),
    };
    match weather {
        "rain" => (
            theme::mix(top, Color32::from_rgb(0x2A, 0x36, 0x3E), 0.4),
            theme::mix(bot, Color32::from_rgb(0x5A, 0x68, 0x66), 0.55),
        ),
        "snow" | "frost" | "hail" => (
            theme::mix(top, Color32::from_rgb(0x2A, 0x33, 0x3A), 0.35),
            theme::mix(bot, Color32::from_rgb(0x7A, 0x82, 0x84), 0.45),
        ),
        "fog" => (
            theme::mix(top, Color32::from_rgb(0x3A, 0x3F, 0x40), 0.45),
            theme::mix(bot, Color32::from_rgb(0x5E, 0x63, 0x60), 0.55),
        ),
        "sandstorm" => (
            Color32::from_rgb(0x3A, 0x2C, 0x1E),
            Color32::from_rgb(0x6B, 0x53, 0x33),
        ),
        "storm" => (
            Color32::from_rgb(0x12, 0x16, 0x1C),
            Color32::from_rgb(0x2A, 0x2F, 0x38),
        ),
        "wind" => (
            top,
            theme::mix(bot, Color32::from_rgb(0x50, 0x58, 0x52), 0.35),
        ),
        _ => (top, bot),
    }
}

fn draw_celestial(p: &egui::Painter, rect: Rect, daypart: &str, weather: &str) {
    if matches!(
        weather,
        "fog" | "sandstorm" | "storm" | "rain" | "snow" | "hail"
    ) {
        return;
    }
    let (cx, cy, r, color) = match daypart {
        "night" => (
            0.78,
            0.22,
            16.0,
            Color32::from_rgba_unmultiplied(0xE8, 0xE4, 0xD0, 200),
        ),
        "dawn" | "dusk" => (
            0.72,
            0.55,
            20.0,
            Color32::from_rgba_unmultiplied(0xD9, 0x9B, 0x62, 180),
        ),
        _ => (
            0.80,
            0.20,
            14.0,
            Color32::from_rgba_unmultiplied(0xF0, 0xE8, 0xC8, 150),
        ),
    };
    let c = pos2(
        rect.left() + rect.width() * cx,
        rect.top() + rect.height() * cy,
    );
    // 光晕两圈 + 本体
    p.circle_filled(
        c,
        r * 2.4,
        Color32::from_rgba_unmultiplied(255, 255, 255, 8),
    );
    p.circle_filled(
        c,
        r * 1.5,
        Color32::from_rgba_unmultiplied(255, 255, 255, 12),
    );
    p.circle_filled(c, r, color);
}

fn draw_ridge(p: &egui::Painter, rect: Rect, base_y: f32, amp: f32, seed: f32, color: Color32) {
    let n = 26;
    let mut mesh = Mesh::default();
    let mut prev: Option<(Pos2, Pos2)> = None;
    for i in 0..=n {
        let t = i as f32 / n as f32;
        let x = rect.left() + rect.width() * t;
        let wave = (t * 7.0 + seed).sin() * 0.55
            + (t * 15.0 + seed * 2.3).sin() * 0.28
            + (t * 31.0 + seed).sin() * 0.12;
        let y = base_y - (wave + 1.0) * amp * 0.5;
        let top = pos2(x, y);
        let bot = pos2(x, rect.bottom());
        if let Some((pt, pb)) = prev {
            let i0 = mesh.vertices.len() as u32;
            mesh.colored_vertex(pt, color);
            mesh.colored_vertex(pb, color);
            mesh.colored_vertex(top, color);
            mesh.colored_vertex(bot, color);
            mesh.add_triangle(i0, i0 + 1, i0 + 2);
            mesh.add_triangle(i0 + 1, i0 + 3, i0 + 2);
        }
        prev = Some((top, bot));
    }
    p.add(Shape::mesh(mesh));
}

fn draw_terrain(p: &egui::Painter, rect: Rect, terrain: &str) {
    let ink = Color32::from_rgba_unmultiplied(0x08, 0x0B, 0x0E, 220);
    let ink_soft = Color32::from_rgba_unmultiplied(0x08, 0x0B, 0x0E, 150);
    let ground = rect.top() + rect.height() * 0.82;

    // 地面：一条略起伏的暗带，压住山脚。
    let mut mesh = Mesh::default();
    let mut prev: Option<(Pos2, Pos2)> = None;
    for i in 0..=20 {
        let t = i as f32 / 20.0;
        let x = rect.left() + rect.width() * t;
        let y = ground - (t * 5.0).sin() * 6.0;
        let top = pos2(x, y);
        let bot = pos2(x, rect.bottom());
        if let Some((pt, pb)) = prev {
            let i0 = mesh.vertices.len() as u32;
            mesh.colored_vertex(pt, ink);
            mesh.colored_vertex(pb, ink);
            mesh.colored_vertex(top, ink);
            mesh.colored_vertex(bot, ink);
            mesh.add_triangle(i0, i0 + 1, i0 + 2);
            mesh.add_triangle(i0 + 1, i0 + 3, i0 + 2);
        }
        prev = Some((top, bot));
    }
    p.add(Shape::mesh(mesh));

    match terrain {
        "woods" | "forest" | "bamboo" => {
            let tree_count = if terrain == "bamboo" { 12 } else { 7 };
            for i in 0..tree_count {
                let t = (i as f32 + 0.7) / tree_count as f32;
                let x = rect.left() + rect.width() * t;
                let scale = 0.7 + rnd(i as u32, 3) * 0.6;
                if terrain == "bamboo" {
                    let h = 54.0 * scale;
                    p.line_segment(
                        [pos2(x, ground + 6.0), pos2(x + 2.0, ground + 6.0 - h)],
                        Stroke::new(2.0_f32, ink_soft),
                    );
                    for k in 1..4 {
                        let yy = ground + 6.0 - h * (k as f32 / 4.0);
                        p.line_segment(
                            [pos2(x, yy), pos2(x + 9.0, yy - 4.0)],
                            Stroke::new(1.0_f32, ink_soft),
                        );
                    }
                } else {
                    let h = 46.0 * scale;
                    let w = 18.0 * scale;
                    p.add(Shape::convex_polygon(
                        vec![
                            pos2(x, ground + 6.0 - h),
                            pos2(x - w, ground + 6.0 - h * 0.35),
                            pos2(x + w, ground + 6.0 - h * 0.35),
                        ],
                        ink_soft,
                        Stroke::NONE,
                    ));
                    p.line_segment(
                        [pos2(x, ground + 6.0), pos2(x, ground + 6.0 - h * 0.4)],
                        Stroke::new(2.0_f32, ink),
                    );
                }
            }
        }
        "water" | "river" | "lake" | "spring" | "marsh" => {
            let water_top = ground + 4.0;
            p.rect_filled(
                Rect::from_min_max(
                    pos2(rect.left(), water_top),
                    pos2(rect.right(), rect.bottom()),
                ),
                0.0,
                Color32::from_rgba_unmultiplied(0x1C, 0x33, 0x3A, 200),
            );
            for i in 0..7 {
                let y = water_top + 10.0 + i as f32 * 9.0;
                let w = rect.width() * (0.18 + rnd(i, 7) * 0.3);
                let x = rect.left() + rect.width() * rnd(i, 11) * 0.7;
                p.line_segment(
                    [pos2(x, y), pos2(x + w, y)],
                    Stroke::new(
                        1.0_f32,
                        Color32::from_rgba_unmultiplied(0xBF, 0xD8, 0xD4, 40),
                    ),
                );
            }
        }
        "cliff" | "peak" | "slope" | "valley" => {
            let cx = rect.left() + rect.width() * 0.72;
            p.add(Shape::convex_polygon(
                vec![
                    pos2(cx - 70.0, rect.bottom()),
                    pos2(cx - 26.0, ground - 96.0),
                    pos2(cx + 4.0, ground - 120.0),
                    pos2(cx + 34.0, ground - 78.0),
                    pos2(cx + 78.0, rect.bottom()),
                ],
                ink,
                Stroke::NONE,
            ));
            p.line_segment(
                [
                    pos2(cx + 4.0, ground - 118.0),
                    pos2(cx + 22.0, ground - 60.0),
                ],
                Stroke::new(
                    1.0_f32,
                    Color32::from_rgba_unmultiplied(0xE6, 0xE2, 0xD8, 26),
                ),
            );
        }
        "town" | "gate" | "ruin" | "shrine" => {
            for i in 0..4 {
                let x = rect.left() + rect.width() * (0.18 + i as f32 * 0.16);
                let h = 26.0 + rnd(i, 5) * 26.0;
                p.rect_filled(
                    Rect::from_min_max(pos2(x, ground + 2.0 - h), pos2(x + 34.0, ground + 2.0)),
                    1.0,
                    ink_soft,
                );
                p.add(Shape::convex_polygon(
                    vec![
                        pos2(x - 5.0, ground + 2.0 - h),
                        pos2(x + 17.0, ground + 2.0 - h - 12.0),
                        pos2(x + 39.0, ground + 2.0 - h),
                    ],
                    ink,
                    Stroke::NONE,
                ));
            }
        }
        "scree" => {
            for i in 0..8 {
                let x = rect.left() + rect.width() * (0.1 + rnd(i, 13) * 0.8);
                let s = 8.0 + rnd(i, 17) * 14.0;
                p.add(Shape::convex_polygon(
                    vec![
                        pos2(x, ground - s),
                        pos2(x - s * 0.7, ground + 4.0),
                        pos2(x + s * 0.8, ground + 4.0),
                    ],
                    ink_soft,
                    Stroke::NONE,
                ));
            }
        }
        "lava" => {
            p.rect_filled(
                Rect::from_min_max(pos2(rect.left(), ground), pos2(rect.right(), rect.bottom())),
                0.0,
                Color32::from_rgba_unmultiplied(0x3A, 0x14, 0x0C, 230),
            );
            for i in 0..6 {
                let x = rect.left() + rect.width() * rnd(i, 19);
                let y = ground + 10.0 + rnd(i, 23) * 30.0;
                p.line_segment(
                    [pos2(x, y), pos2(x + 40.0 + rnd(i, 29) * 50.0, y + 6.0)],
                    Stroke::new(
                        2.0_f32,
                        Color32::from_rgba_unmultiplied(0xE0, 0x6A, 0x2A, 120),
                    ),
                );
            }
        }
        "desert" => {
            for i in 0..3 {
                let y = ground + 10.0 + i as f32 * 14.0;
                p.line_segment(
                    [pos2(rect.left(), y), pos2(rect.right() * 0.55, y - 10.0)],
                    Stroke::new(
                        1.0_f32,
                        Color32::from_rgba_unmultiplied(0xC9, 0xA2, 0x27, 30),
                    ),
                );
            }
        }
        _ => {}
    }
}

/// 人影：墨色剪影，坐/站/行走三态。比例按"身高 = 100 单位"定，
/// 头小、肩宽、下摆开，才不会画成一根石碑。
fn draw_figure(p: &egui::Painter, rect: Rect, pose: Pose) {
    let k = (rect.height() / 220.0).clamp(1.1, 3.0);
    let cx = rect.center().x;
    let ground = rect.top() + rect.height() * 0.86;
    let ink = Color32::from_rgba_unmultiplied(0x05, 0x08, 0x0B, 238);
    let rim = Color32::from_rgba_unmultiplied(0x7F, 0xD1, 0xC1, 40);

    // 地面投影
    p.add(Shape::ellipse_filled(
        pos2(cx, ground + 3.0 * k),
        vec2(30.0 * k, 6.0 * k),
        Color32::from_rgba_unmultiplied(0x00, 0x00, 0x00, 80),
    ));

    match pose {
        Pose::Sitting => {
            // 盘坐：腿是一横椭圆，身子矮一截。
            p.add(Shape::ellipse_filled(
                pos2(cx, ground - 5.0 * k),
                vec2(26.0 * k, 8.0 * k),
                ink,
            ));
            p.add(Shape::convex_polygon(
                vec![
                    pos2(cx - 11.0 * k, ground - 8.0 * k),
                    pos2(cx + 11.0 * k, ground - 8.0 * k),
                    pos2(cx + 10.0 * k, ground - 52.0 * k),
                    pos2(cx - 10.0 * k, ground - 52.0 * k),
                ],
                ink,
                Stroke::NONE,
            ));
            // 头 + 颈
            p.circle_filled(pos2(cx, ground - 64.0 * k), 7.5 * k, ink);
            p.line_segment(
                [pos2(cx, ground - 56.0 * k), pos2(cx, ground - 52.0 * k)],
                Stroke::new(3.0_f32 * k, ink),
            );
            // 肩线微光
            p.line_segment(
                [
                    pos2(cx - 10.0 * k, ground - 50.0 * k),
                    pos2(cx + 10.0 * k, ground - 50.0 * k),
                ],
                Stroke::new(1.2_f32, rim),
            );
        }
        Pose::Walking => {
            let lean = 4.0 * k;
            p.add(Shape::convex_polygon(
                vec![
                    pos2(cx - 12.0 * k + lean, ground - 76.0 * k),
                    pos2(cx + 12.0 * k + lean, ground - 76.0 * k),
                    pos2(cx + 19.0 * k, ground - 2.0 * k),
                    pos2(cx + 3.0 * k, ground - 2.0 * k),
                ],
                ink,
                Stroke::NONE,
            ));
            // 迈出的腿
            p.line_segment(
                [
                    pos2(cx + 8.0 * k, ground - 10.0 * k),
                    pos2(cx + 16.0 * k, ground),
                ],
                Stroke::new(6.0_f32 * k, ink),
            );
            p.circle_filled(pos2(cx + lean, ground - 86.0 * k), 7.5 * k, ink);
            p.line_segment(
                [
                    pos2(cx + lean, ground - 78.0 * k),
                    pos2(cx + lean, ground - 74.0 * k),
                ],
                Stroke::new(3.0_f32 * k, ink),
            );
            p.line_segment(
                [
                    pos2(cx - 11.0 * k + lean, ground - 74.0 * k),
                    pos2(cx + 11.0 * k + lean, ground - 74.0 * k),
                ],
                Stroke::new(1.2_f32, rim),
            );
        }
        Pose::Standing => {
            // 肩宽 12、下摆宽 18：上窄下宽才是长袍，不是柱子。
            p.add(Shape::convex_polygon(
                vec![
                    pos2(cx - 12.0 * k, ground - 76.0 * k),
                    pos2(cx + 12.0 * k, ground - 76.0 * k),
                    pos2(cx + 18.0 * k, ground),
                    pos2(cx - 18.0 * k, ground),
                ],
                ink,
                Stroke::NONE,
            ));
            // 双臂
            p.line_segment(
                [
                    pos2(cx - 11.0 * k, ground - 72.0 * k),
                    pos2(cx - 15.0 * k, ground - 34.0 * k),
                ],
                Stroke::new(4.5_f32 * k, ink),
            );
            p.line_segment(
                [
                    pos2(cx + 11.0 * k, ground - 72.0 * k),
                    pos2(cx + 15.0 * k, ground - 34.0 * k),
                ],
                Stroke::new(4.5_f32 * k, ink),
            );
            // 头 + 颈
            p.circle_filled(pos2(cx, ground - 88.0 * k), 7.5 * k, ink);
            p.line_segment(
                [pos2(cx, ground - 80.0 * k), pos2(cx, ground - 76.0 * k)],
                Stroke::new(3.0_f32 * k, ink),
            );
            // 肩线 + 腰带
            p.line_segment(
                [
                    pos2(cx - 12.0 * k, ground - 74.0 * k),
                    pos2(cx + 12.0 * k, ground - 74.0 * k),
                ],
                Stroke::new(1.2_f32, rim),
            );
            p.line_segment(
                [
                    pos2(cx - 14.0 * k, ground - 40.0 * k),
                    pos2(cx + 14.0 * k, ground - 40.0 * k),
                ],
                Stroke::new(
                    1.0_f32,
                    Color32::from_rgba_unmultiplied(0x7F, 0xD1, 0xC1, 22),
                ),
            );
        }
    }
}

/// 脚下的路：一条向画面下方放大的浅带，配合人影给出"纵深"和"可前进"的暗示。
fn draw_path(p: &egui::Painter, rect: Rect) {
    let cx = rect.center().x;
    let ground = rect.top() + rect.height() * 0.86;
    let h = rect.bottom() - ground;
    if h < 10.0 {
        return;
    }
    let w0 = rect.height() * 0.05;
    let w1 = rect.height() * 0.22;
    p.add(Shape::convex_polygon(
        vec![
            pos2(cx - w0, ground),
            pos2(cx + w0, ground),
            pos2(cx + w1, rect.bottom()),
            pos2(cx - w1, rect.bottom()),
        ],
        Color32::from_rgba_unmultiplied(0x39, 0x36, 0x2C, 46),
        Stroke::NONE,
    ));
    // 路面两条边线
    for s in [-1.0f32, 1.0] {
        p.line_segment(
            [pos2(cx + s * w0, ground), pos2(cx + s * w1, rect.bottom())],
            Stroke::new(
                1.0_f32,
                Color32::from_rgba_unmultiplied(0xC9, 0xB8, 0x96, 22),
            ),
        );
    }
}

fn draw_weather(p: &egui::Painter, rect: Rect, weather: &str) {
    match weather {
        "rain" | "storm" => {
            for i in 0..60 {
                let x = rect.left() + rect.width() * rnd(i, 31);
                let y = rect.top() + rect.height() * rnd(i, 37);
                p.line_segment(
                    [pos2(x, y), pos2(x - 3.0, y + 12.0 + rnd(i, 41) * 8.0)],
                    Stroke::new(
                        1.0_f32,
                        Color32::from_rgba_unmultiplied(0xBF, 0xD4, 0xDC, 55),
                    ),
                );
            }
        }
        "snow" | "frost" | "hail" => {
            for i in 0..46 {
                let x = rect.left() + rect.width() * rnd(i, 43);
                let y = rect.top() + rect.height() * rnd(i, 47);
                let r = 1.2 + rnd(i, 53) * 1.6;
                p.circle_filled(
                    pos2(x, y),
                    r,
                    Color32::from_rgba_unmultiplied(0xF0, 0xF4, 0xF6, 70),
                );
            }
        }
        "sandstorm" => {
            for i in 0..40 {
                let y = rect.top() + rect.height() * rnd(i, 59);
                let x = rect.left() + rect.width() * rnd(i, 61);
                p.line_segment(
                    [pos2(x, y), pos2(x + 26.0, y + 2.0)],
                    Stroke::new(
                        1.0_f32,
                        Color32::from_rgba_unmultiplied(0xC9, 0xA2, 0x27, 40),
                    ),
                );
            }
        }
        "fog" => {
            for i in 0..4 {
                let y = rect.top() + rect.height() * (0.35 + i as f32 * 0.13);
                p.rect_filled(
                    Rect::from_min_max(pos2(rect.left(), y), pos2(rect.right(), y + 34.0)),
                    8.0,
                    Color32::from_rgba_unmultiplied(0xC8, 0xCE, 0xCC, 22),
                );
            }
        }
        _ => {}
    }
}

fn draw_frame(p: &egui::Painter, rect: Rect) {
    p.rect_stroke(
        rect.shrink(0.5),
        3.0,
        Stroke::new(1.0_f32, theme::LINE),
        egui::StrokeKind::Inside,
    );
    // 四角短饰线
    let l = 16.0;
    let c = theme::ACCENT.linear_multiply(0.55);
    for (corner, dx, dy) in [
        (rect.left_top(), 1.0, 1.0),
        (rect.right_top(), -1.0, 1.0),
        (rect.left_bottom(), 1.0, -1.0),
        (rect.right_bottom(), -1.0, -1.0),
    ] {
        let o = corner + vec2(6.0 * dx, 6.0 * dy);
        p.line_segment([o, o + vec2(l * dx, 0.0)], Stroke::new(1.0_f32, c));
        p.line_segment([o, o + vec2(0.0, l * dy)], Stroke::new(1.0_f32, c));
    }
}

fn draw_caption(p: &egui::Painter, rect: Rect, s: &Scene<'_>) {
    // 顶部渐变压暗：标题压在天空上也要读得清。
    let bands = 18;
    let h = 132.0;
    for i in 0..bands {
        let t = i as f32 / bands as f32;
        let a = (150.0 * (1.0 - t) * (1.0 - t)) as u8;
        let y0 = rect.top() + h * t;
        let y1 = rect.top() + h * (t + 1.0 / bands as f32) + 1.0;
        p.rect_filled(
            Rect::from_min_max(pos2(rect.left(), y0), pos2(rect.right(), y1)),
            0,
            Color32::from_rgba_unmultiplied(0x04, 0x06, 0x08, a),
        );
    }

    p.text(
        pos2(rect.left() + 26.0, rect.top() + 20.0),
        Align2::LEFT_TOP,
        s.name,
        theme::serif_id(30.0),
        theme::TEXT,
    );

    let mut meta = String::new();
    if !s.region.is_empty() {
        meta.push_str(s.region);
    }
    if let Some(g) = s.grade {
        if !meta.is_empty() {
            meta.push_str(" · ");
        }
        meta.push_str(g);
    }
    meta.push_str(&format!(" · 探索 {}/20", s.explored));
    p.text(
        pos2(rect.left() + 28.0, rect.top() + 58.0),
        Align2::LEFT_TOP,
        meta,
        FontId::proportional(11.5),
        theme::GOLD,
    );

    if !s.desc.is_empty() {
        p.text(
            pos2(rect.left() + 28.0, rect.top() + 76.0),
            Align2::LEFT_TOP,
            crate::util::text::truncate_chars(s.desc, 48),
            FontId::proportional(13.0),
            theme::mix(theme::TEXT, theme::BG, 0.28),
        );
    }
}

/// 确定性伪随机：同一帧内不抖动，也不引入 rand。
fn rnd(i: u32, salt: u32) -> f32 {
    let x = ((i as f32 + 1.0) * 12.9898 + salt as f32 * 78.233).sin() * 43758.545;
    x.fract().abs()
}
