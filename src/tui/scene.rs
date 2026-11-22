//! ASCII 场景：山脊、天气、人影、脚下小路。
//!
//! 生成一张 `width × height` 的字符网格，渲染层直接贴到终端。

use ratatui::style::Color;

use super::theme;

#[derive(Clone, Copy, PartialEq, Eq, Default)]
pub enum Pose {
    #[default]
    Standing,
    Sitting,
    Walking,
}

#[derive(Clone, Copy)]
pub struct Cell {
    pub ch: char,
    pub fg: Color,
}

impl Default for Cell {
    fn default() -> Self {
        Self {
            ch: ' ',
            fg: theme::FAINT,
        }
    }
}

pub struct Scene<'a> {
    pub terrain: &'a str,
    pub weather: &'a str,
    pub daypart: &'a str,
    pub pose: Pose,
}

#[allow(clippy::needless_range_loop)] // 以 (行,列) 当坐标用，比迭代器直观
pub fn build(width: u16, height: u16, s: &Scene<'_>) -> Vec<Vec<Cell>> {
    let w = width.max(8) as usize;
    let h = height.max(4) as usize;
    let mut grid = vec![vec![Cell::default(); w]; h];

    // 天空底色：极暗的渐变（顶部更黑，靠近地平线略亮）
    let sky_top = theme::BG;
    let sky_bot = match s.daypart {
        "dawn" => Color::Rgb(28, 22, 18),
        "dusk" => Color::Rgb(30, 20, 24),
        "night" => Color::Rgb(6, 8, 12),
        _ => Color::Rgb(10, 14, 18),
    };
    for (r, row) in grid.iter_mut().enumerate() {
        let t = r as f32 / h as f32;
        let fg = mix(sky_top, sky_bot, t);
        for c in row.iter_mut() {
            c.fg = fg;
        }
    }

    // 远、中、近三层山脊
    for (base_r, amp_r, seed, color) in [
        (0.46f32, 0.24f32, 1.7f32, theme::FAINT),
        (0.60, 0.19, 3.1, mix(theme::FAINT, theme::DIM, 0.4)),
        (0.72, 0.14, 5.3, mix(theme::DIM, theme::TEXT, 0.25)),
    ] {
        let base = (h as f32 * base_r) as i32;
        let mut prev = 0.0f32;
        for c in 0..w {
            let t = c as f32 / w as f32;
            let wave = (t * 6.5 + seed).sin() * 0.55
                + (t * 14.0 + seed * 2.0).sin() * 0.30
                + (t * 29.0 + seed).sin() * 0.15;
            let hh = ((0.45 + 0.55 * wave) * amp_r * h as f32).max(0.0);
            let top = base - hh as i32;
            if top >= 0 && (top as usize) < h {
                let ch = if hh > prev + 0.05 {
                    '/'
                } else if hh + 0.05 < prev {
                    '\\'
                } else {
                    '_'
                };
                grid[top as usize][c] = Cell { ch, fg: color };
                // 脊线下两行补山体，做出厚度
                for d in 1..=2usize {
                    let r = top as usize + d;
                    if r < base.max(0) as usize && r < h {
                        grid[r][c] = Cell { ch: ':', fg: color };
                    }
                }
            }
            prev = hh;
        }
    }

    // 地势：水/林/镇的存在感
    let ground = (h as f32 * 0.82) as usize;
    match s.terrain {
        "river" | "lake" => {
            for r in ground..h {
                for c in 0..w {
                    grid[r][c] = Cell {
                        ch: if (r + c) % 3 == 0 { '~' } else { ' ' },
                        fg: mix(theme::BG, theme::ACCENT, 0.35),
                    };
                }
            }
        }
        "woods" | "forest" | "bamboo" => {
            for (i, c) in (2..w.saturating_sub(2)).step_by(5).enumerate() {
                let top = ground.saturating_sub(2 + i % 3);
                if top < h {
                    grid[top][c] = Cell {
                        ch: '^',
                        fg: mix(theme::DIM, theme::GOOD, 0.35),
                    };
                    if top + 1 < h {
                        grid[top + 1][c] = Cell {
                            ch: '|',
                            fg: theme::FAINT,
                        };
                    }
                }
            }
        }
        "town" | "gate" => {
            for (i, c) in (3..w.saturating_sub(3)).step_by(7).enumerate() {
                let top = ground.saturating_sub(3 + i % 2);
                for (d, ch) in ['_', '|', '|'].iter().enumerate() {
                    if top + d < h {
                        grid[top + d][c] = Cell {
                            ch: *ch,
                            fg: theme::GOLD,
                        };
                    }
                }
            }
        }
        _ => {}
    }

    // 地面
    for c in 0..w {
        if ground < h {
            grid[ground][c] = Cell {
                ch: '=',
                fg: mix(theme::BG, theme::GOLD, 0.22),
            };
        }
        for r in (ground + 1)..h {
            grid[r][c] = Cell {
                ch: '.',
                fg: mix(theme::BG, theme::GOLD, 0.22),
            };
        }
    }

    // 脚下小路
    let cx = w / 2;
    for r in ground..h {
        let spread = 1 + (r - ground) / 2;
        for c in cx.saturating_sub(spread)..=(cx + spread).min(w - 1) {
            if grid[r][c].ch == '.' || grid[r][c].ch == ' ' {
                grid[r][c] = Cell {
                    ch: '.',
                    fg: mix(theme::BG, theme::TEXT, 0.42),
                };
            }
        }
    }

    // 人影：三行字符
    let body: [&str; 3] = match s.pose {
        Pose::Sitting => [" o ", "/|\\", "==="],
        Pose::Walking => [" o ", "/|\\", " |\\"],
        Pose::Standing => [" o ", "/|\\", "/ \\"],
    };
    let gy = ground.saturating_sub(3);
    for (i, line) in body.iter().enumerate() {
        let r = gy + i;
        if r >= h {
            continue;
        }
        for (j, ch) in line.chars().enumerate() {
            let c = cx.saturating_sub(1) + j;
            if c < w && ch != ' ' {
                grid[r][c] = Cell {
                    ch,
                    fg: theme::TEXT,
                };
            }
        }
    }

    // 天气
    let (wch, wcolor) = match s.weather {
        "rain" | "storm" => ('|', mix(theme::BG, theme::ACCENT, 0.45)),
        "snow" | "frost" | "hail" => ('*', mix(theme::BG, theme::TEXT, 0.55)),
        "sandstorm" => ('~', mix(theme::BG, theme::GOLD, 0.4)),
        "fog" => ('~', mix(theme::BG, theme::DIM, 0.35)),
        _ => (' ', theme::BG),
    };
    if wch != ' ' {
        let count = match s.weather {
            "fog" => w / 4,
            _ => w,
        };
        let top_rows = (h as f32 * 0.78) as usize;
        for i in 0..count {
            let c = (rnd(i as u32, 11) * w as f32) as usize % w;
            let r = (rnd(i as u32, 23) * top_rows.max(1) as f32) as usize % top_rows.max(1);
            if grid[r][c].ch == ' ' || grid[r][c].ch == ':' {
                grid[r][c] = Cell {
                    ch: wch,
                    fg: wcolor,
                };
            }
        }
    }

    grid
}

fn mix(a: Color, b: Color, t: f32) -> Color {
    let t = t.clamp(0.0, 1.0);
    let (ar, ag, ab) = rgb(a);
    let (br, bg, bb) = rgb(b);
    let f = |x: u8, y: u8| (x as f32 + (y as f32 - x as f32) * t).round() as u8;
    Color::Rgb(f(ar, br), f(ag, bg), f(ab, bb))
}

fn rgb(c: Color) -> (u8, u8, u8) {
    match c {
        Color::Rgb(r, g, b) => (r, g, b),
        _ => (0, 0, 0),
    }
}

fn rnd(i: u32, salt: u32) -> f32 {
    let x = ((i as f32 + 1.0) * 12.9898 + salt as f32 * 78.233).sin() * 43758.545;
    x.fract().abs()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sc<'a>(terrain: &'a str, weather: &'a str) -> Scene<'a> {
        Scene {
            terrain,
            weather,
            daypart: "day",
            pose: Pose::Standing,
        }
    }

    #[test]
    fn grid_size_matches_request() {
        let g = build(40, 12, &sc("plain", "clear"));
        assert_eq!(g.len(), 12);
        assert_eq!(g[0].len(), 40);
    }

    #[test]
    fn figure_and_ground_are_drawn() {
        let g = build(40, 12, &sc("plain", "clear"));
        let flat: String = g.iter().flatten().map(|c| c.ch).collect();
        assert!(flat.contains('o'), "应画出入影的头");
        assert!(flat.contains('='), "应画出地平线");
    }

    #[test]
    fn weather_overlays_chars() {
        let g = build(40, 12, &sc("plain", "rain"));
        let flat: String = g.iter().flatten().map(|c| c.ch).collect();
        assert!(flat.contains('|'), "雨天应有竖线");
    }
}
