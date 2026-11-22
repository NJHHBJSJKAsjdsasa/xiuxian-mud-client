//! 已探索地图：客户端把每次 `map` / `look` 见到的节点与边记下来。
//!
//! 服务端只下发"当前节点 + 相邻出口"，没有全图坐标；地图因此由客户端累积：
//! 见过一条路就记一条，力导向布局让相连的节点凑在一起，形状像地图而不是星图。

use std::collections::{BTreeMap, HashMap};

use crate::protocol::msg::MapView;

#[derive(Debug, Clone, Default)]
pub struct MapNode {
    pub id: String,
    pub name: String,
    pub terrain: String,
    /// 亲自到过（当前或历史所在地）。
    pub visited: bool,
    /// 内容表里的地图坐标；老服务端不下发时为 false，退回自动布局。
    pub has_geo: bool,
    pub gx: i32,
    pub gy: i32,
}

#[derive(Debug, Clone, Default)]
pub struct MapEdge {
    #[allow(dead_code)]
    pub cost_sec: i64,
    pub ok: bool,
    #[allow(dead_code)]
    pub blocked: bool,
}

#[derive(Default)]
pub struct MapGraph {
    pub nodes: BTreeMap<String, MapNode>,
    /// 无向边：`(较小的 id, 较大的 id)`。
    pub edges: BTreeMap<(String, String), MapEdge>,
    /// 布局缓存（世界坐标）；store 层不引任何 UI 类型。
    pos: BTreeMap<String, (f32, f32)>,
}

impl MapGraph {
    /// 用一帧地图视图更新记忆。
    pub fn observe(&mut self, m: &MapView) {
        if m.loc.is_empty() {
            return;
        }
        let cur = self.nodes.entry(m.loc.clone()).or_default();
        cur.id = m.loc.clone();
        if !m.name.is_empty() {
            cur.name = m.name.clone();
        }
        if !m.terrain.is_empty() {
            cur.terrain = m.terrain.clone();
        }
        // (0,0) 是合法坐标，所以用 Option 区分"没下发"与"原点"。
        if let (Some(x), Some(y)) = (m.x, m.y) {
            cur.gx = x;
            cur.gy = y;
            cur.has_geo = true;
        }
        cur.visited = true;

        for ex in &m.exits {
            if ex.to.is_empty() {
                continue;
            }
            let n = self.nodes.entry(ex.to.clone()).or_default();
            n.id = ex.to.clone();
            if let Some(name) = &ex.name
                && !name.is_empty()
            {
                n.name = name.clone();
            }
            if !ex.terrain.is_empty() {
                n.terrain = ex.terrain.clone();
            }
            if let (Some(x), Some(y)) = (ex.x, ex.y) {
                n.gx = x;
                n.gy = y;
                n.has_geo = true;
            }
            self.edges.insert(
                edge_key(&m.loc, &ex.to),
                MapEdge {
                    cost_sec: ex.cost_sec,
                    ok: ex.ok,
                    blocked: ex.blocked == Some(true),
                },
            );

            // 新节点先摆在邻居附近，方向由 id 哈希决定，避免全叠在原点。
            if !self.pos.contains_key(&ex.to) {
                let base = self.pos.get(&m.loc).copied().unwrap_or((0.0, 0.0));
                let ang = hash01(&ex.to) * std::f32::consts::TAU;
                self.pos.insert(
                    ex.to.clone(),
                    (base.0 + ang.cos() * 170.0, base.1 + ang.sin() * 170.0),
                );
            }
        }
        if !self.pos.contains_key(&m.loc) {
            self.pos.insert(m.loc.clone(), (0.0, 0.0));
        }
        self.relax(&m.loc);
    }

    /// 力导向微调：斥力让节点散开，边像弹簧把相连的拉近，当前节点钉在原点。
    /// 只在 `observe` 后跑一次（不是每帧），所以迭代次数可以给足。
    fn relax(&mut self, current: &str) {
        let ids: Vec<String> = self.nodes.keys().cloned().collect();
        let n = ids.len();
        if n < 2 {
            return;
        }
        let mut p: Vec<(f32, f32)> = ids
            .iter()
            .map(|id| self.pos.get(id).copied().unwrap_or((0.0, 0.0)))
            .collect();
        let index: BTreeMap<&str, usize> = ids
            .iter()
            .enumerate()
            .map(|(i, id)| (id.as_str(), i))
            .collect();
        let edges: Vec<(usize, usize)> = self
            .edges
            .keys()
            .filter_map(|(a, b)| Some((*index.get(a.as_str())?, *index.get(b.as_str())?)))
            .collect();

        // 把当前节点挪到原点，避免每走一步整张图"跳一下"。
        if let Some(&ci) = index.get(current) {
            let (cx, cy) = p[ci];
            for q in &mut p {
                q.0 -= cx;
                q.1 -= cy;
            }
        }

        let iterations = if n > 60 { 12 } else { 60 };
        for _ in 0..iterations {
            let mut dx = vec![0.0f32; n];
            let mut dy = vec![0.0f32; n];

            // 斥力：两两排斥，近处更强。
            for i in 0..n {
                for j in (i + 1)..n {
                    let mut vx = p[i].0 - p[j].0;
                    let mut vy = p[i].1 - p[j].1;
                    let mut d2 = vx * vx + vy * vy;
                    if d2 < 1.0 {
                        vx = 0.7 + i as f32 * 0.01;
                        vy = 0.7 + j as f32 * 0.01;
                        d2 = vx * vx + vy * vy;
                    }
                    let d = d2.sqrt();
                    let f = (12000.0 / d2).min(26.0);
                    let ux = vx / d;
                    let uy = vy / d;
                    dx[i] += ux * f;
                    dy[i] += uy * f;
                    dx[j] -= ux * f;
                    dy[j] -= uy * f;
                }
            }

            // 弹簧：边越短越推、越长越拉，目标长度 150。
            for (i, j) in &edges {
                let vx = p[*j].0 - p[*i].0;
                let vy = p[*j].1 - p[*i].1;
                let d = (vx * vx + vy * vy).sqrt().max(0.5);
                let f = (d - 150.0) * 0.06;
                let ux = vx / d;
                let uy = vy / d;
                dx[*i] += ux * f;
                dy[*i] += uy * f;
                dx[*j] -= ux * f;
                dy[*j] -= uy * f;
            }

            for i in 0..n {
                if ids[i] == current {
                    p[i] = (0.0, 0.0);
                    continue;
                }
                // 向心，防止孤立节点飞太远。
                dx[i] -= p[i].0 * 0.010;
                dy[i] -= p[i].1 * 0.010;
                let step = (dx[i] * dx[i] + dy[i] * dy[i]).sqrt().max(0.001);
                let cap = step.min(18.0);
                p[i].0 += dx[i] / step * cap;
                p[i].1 += dy[i] / step * cap;
            }
        }

        for (id, q) in ids.iter().zip(p) {
            self.pos.insert(id.clone(), q);
        }
    }

    pub fn is_empty(&self) -> bool {
        self.nodes.is_empty()
    }

    #[allow(dead_code)]
    pub fn clear(&mut self) {
        self.nodes.clear();
        self.edges.clear();
        self.pos.clear();
    }

    /// 与某节点相邻的节点（含不可通行的，由调用方判断 `ok`）。
    pub fn neighbors_of(&self, id: &str) -> Vec<(String, MapEdge)> {
        let mut out = Vec::new();
        for ((a, b), e) in &self.edges {
            if a == id {
                out.push((b.clone(), e.clone()));
            } else if b == id {
                out.push((a.clone(), e.clone()));
            }
        }
        out
    }

    /// 返回布局：服务端给了坐标就按真实排布，没给就退回力导向缓存。
    pub fn layout(&self, current: &str) -> HashMap<String, (f32, f32)> {
        if self.nodes.get(current).map(|n| n.has_geo).unwrap_or(false) {
            return self.layout_geo(current);
        }
        let (cx, cy) = self.pos.get(current).copied().unwrap_or((0.0, 0.0));
        self.nodes
            .keys()
            .map(|id| {
                let (x, y) = self.pos.get(id).copied().unwrap_or((0.0, 0.0));
                (id.clone(), (x - cx, y - cy))
            })
            .collect()
    }

    /// 按内容表坐标摊开：一格约 6px，y 轴取反（服务端 y 向北为正）。
    fn layout_geo(&self, current: &str) -> HashMap<String, (f32, f32)> {
        const SCALE: f32 = 11.0;
        let cur = self.nodes.get(current);
        let (cx, cy) = cur.map(|n| (n.gx, n.gy)).unwrap_or((0, 0));
        let mut out: HashMap<String, (f32, f32)> = HashMap::new();
        for (id, n) in &self.nodes {
            if n.has_geo {
                out.insert(
                    id.clone(),
                    ((n.gx - cx) as f32 * SCALE, -(n.gy - cy) as f32 * SCALE),
                );
            }
        }
        // 个别节点没坐标（老数据/GM 手写漏了）：贴在邻居的平均位置上，别丢。
        for id in self.nodes.keys() {
            if out.contains_key(id) {
                continue;
            }
            let mut sx = 0.0;
            let mut sy = 0.0;
            let mut cnt = 0.0;
            for (nb, _) in self.neighbors_of(id) {
                if let Some((px, py)) = out.get(&nb) {
                    sx += px;
                    sy += py;
                    cnt += 1.0;
                }
            }
            let jitter = hash01(id) * std::f32::consts::TAU;
            let p = if cnt > 0.0 {
                (
                    sx / cnt + jitter.cos() * 26.0,
                    sy / cnt + jitter.sin() * 26.0,
                )
            } else {
                (jitter.cos() * 40.0, jitter.sin() * 40.0)
            };
            out.insert(id.clone(), p);
        }
        out
    }
}

fn edge_key(a: &str, b: &str) -> (String, String) {
    if a <= b {
        (a.to_string(), b.to_string())
    } else {
        (b.to_string(), a.to_string())
    }
}

fn hash01(s: &str) -> f32 {
    let mut h: u32 = 2_166_136_261;
    for b in s.as_bytes() {
        h ^= *b as u32;
        h = h.wrapping_mul(16_777_619);
    }
    (h % 10_000) as f32 / 10_000.0
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::protocol::msg::{ExitView, MapView};

    fn exit(to: &str, cost: i64) -> ExitView {
        ExitView {
            to: to.into(),
            name: Some(format!("地名{to}")),
            terrain: "plain".into(),
            cost_sec: cost,
            ok: true,
            ..Default::default()
        }
    }

    #[test]
    fn observe_records_nodes_and_edges() {
        let mut g = MapGraph::default();
        g.observe(&MapView {
            loc: "a".into(),
            name: "甲".into(),
            terrain: "town".into(),
            exits: vec![exit("b", 3), exit("c", 5)],
            ..Default::default()
        });
        assert_eq!(g.nodes.len(), 3);
        assert_eq!(g.edges.len(), 2);
        assert!(g.nodes["a"].visited);
        assert!(!g.nodes["b"].visited);

        g.observe(&MapView {
            loc: "b".into(),
            name: "乙".into(),
            terrain: "forest".into(),
            exits: vec![exit("a", 3)],
            ..Default::default()
        });
        assert!(g.nodes["b"].visited);
        assert_eq!(g.edges.len(), 2);
    }

    #[test]
    fn layout_pins_current_at_origin_and_separates_neighbors() {
        let mut g = MapGraph::default();
        g.observe(&MapView {
            loc: "a".into(),
            name: "甲".into(),
            exits: vec![exit("b", 3), exit("c", 5)],
            ..Default::default()
        });
        let pos = g.layout("a");
        assert_eq!(pos.len(), 3);
        assert!(pos["a"].0.abs() < 1.0 && pos["a"].1.abs() < 1.0);
        assert!(
            ((pos["b"].0 - pos["c"].0).powi(2) + (pos["b"].1 - pos["c"].1).powi(2)).sqrt() > 20.0
        );
        // 邻居不该被弹到很远。
        assert!((pos["b"].0.hypot(pos["b"].1)) < 400.0);
    }
}
