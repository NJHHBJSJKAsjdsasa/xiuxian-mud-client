# 客户端工程

这是《九州问道》的客户端，**独立仓库**：`git@github.com:NJHHBJSJKAsjdsasa/xiuxian-mud-client.git`。
服务端在另一个仓库（`xiuxian-mud`），两边只通过协议对齐。

## 形态

**Rust 写的原生桌面 GUI（egui / eframe），不是网页，也不是终端界面。**
单 exe、无运行时、无 WebView；Windows / macOS / Linux 三平台可跑。
界面主体是日志 + 表格 + 曲线，窗口里才有富文本、列排序、缩放与顺手的鼠标操作（理由见 `docs/01-SCOPE.md` §2）。

## 现状

- **只有设计文档**（`docs/`），代码尚未开始（`src/` 在 M1 落盘）。
- 协议以服务端 `docs/05-PROTOCOL.md` 为准，接入流程看 `docs/12-API.md`。

## 先读哪几份

| 顺序 | 文档 | 作用 |
|---|---|---|
| 1 | [`docs/00-INDEX.md`](docs/00-INDEX.md) | 文档目录与边界 |
| 2 | [`docs/01-SCOPE.md`](docs/01-SCOPE.md) | 做什么、不做什么 |
| 3 | [`docs/02-ARCH.md`](docs/02-ARCH.md) | crate 选型、分层、线程模型、目录结构 |
| 4 | [`docs/03-CONNECTION.md`](docs/03-CONNECTION.md) | 登录、握手、重连 |
| 5 | [`docs/05-UI.md`](docs/05-UI.md) | 窗口布局、组件、主题、键位 |
| 6 | [`docs/09-ROADMAP.md`](docs/09-ROADMAP.md) | 里程碑与完成定义 |

## 计划中的技术栈

- Rust（edition 2024）+ tokio
- 界面：`eframe` / `egui` 0.37（wgpu 后端，可切 glow）；`egui_extras` 表格、`egui_plot` 曲线
- 传输：`tokio-tungstenite` + `rustls`
- 序列化：`serde` / `serde_json`
- 测试：`cargo test` + `egui_kittest` 快照 + 帧回放（JSONL）+ 服务端脚本对照

## 本地联调

服务端是另一个仓库，先把它跑起来（默认 `8080`）：

```bash
# 服务端仓库
go run ./cmd/server

# 本仓库
cargo run -- --server ws://127.0.0.1:8080
```

老机器或远程桌面渲染失败时：`cargo run -- --renderer glow`。
服务器地址也可以写在 `config.toml` 的 `server` 项；命令行优先。

## 自查

```bash
python tools/doc-check.py     # 文档矩阵与链接
cargo fmt --check
cargo clippy --all-targets -- -D warnings
cargo test
```

## 约定

- 不复制服务端数值（伤害、体力、次数、回合上限）；一律从帧里读或引用服务端文档。
- 不本地推导战斗结果；只播服务端给的帧。
- 协议版本 `PROTO` 与 `hello.proto` 必须一致；收到关闭码 `4004` 提示升级，不重连。
- 中文用内置字体子集，缺字不许出现"豆腐块"；新字先进 `assets/fonts/charset.txt` 再重新裁剪。
- 界面只在有状态变化/动画/倒计时时重绘；不要无条件 `request_repaint()`。
