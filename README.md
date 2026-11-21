# 客户端工程

这是《九州问道》的客户端，**独立仓库**：`git@github.com:NJHHBJSJKAsjdsasa/xiuxian-mud-client.git`。
服务端在另一个仓库（`xiuxian-mud`），两边只通过协议对齐。

## 形态

**Rust 写的终端客户端（TUI），首版不做网页。** 单二进制、无运行时，三平台（Windows / macOS / Linux）可跑。
协议是 JSON over WebSocket，界面以日志与表格为主，终端是最短路径（理由见 `docs/01-SCOPE.md` §2）。

## 现状

- **只有设计文档**（`docs/`），代码尚未开始（`src/` 在 M1 落盘）。
- 协议以服务端 `docs/05-PROTOCOL.md` 为准，接入流程看 `docs/12-API.md`。

## 先读哪几份

| 顺序 | 文档 | 作用 |
|---|---|---|
| 1 | [`docs/00-INDEX.md`](docs/00-INDEX.md) | 文档目录与边界 |
| 2 | [`docs/01-SCOPE.md`](docs/01-SCOPE.md) | 做什么、不做什么 |
| 3 | [`docs/02-ARCH.md`](docs/02-ARCH.md) | crate 选型、分层与目录结构 |
| 4 | [`docs/03-CONNECTION.md`](docs/03-CONNECTION.md) | 登录、握手、重连 |
| 5 | [`docs/09-ROADMAP.md`](docs/09-ROADMAP.md) | 里程碑与完成定义 |

## 计划中的技术栈

- Rust（edition 2021）+ tokio
- 界面：`ratatui` + `crossterm`
- 传输：`tokio-tungstenite` + `rustls`
- 序列化：`serde` / `serde_json`
- 测试：`cargo test` + 帧回放（JSONL）+ 对服务端脚本输出的对照

## 本地联调

服务端是另一个仓库，先把它跑起来（默认 `8080`）：

```bash
# 服务端仓库
go run ./cmd/server

# 本仓库
cargo run -- --server ws://127.0.0.1:8080
```

服务器地址也可以写在 `config.toml` 的 `server` 项；命令行优先。

## 自查

```bash
python tools/doc-check.py     # 文档矩阵与链接
cargo fmt --check
cargo clippy -- -D warnings
cargo test
```

## 约定

- 不复制服务端数值（伤害、体力、次数、回合上限）；一律从帧里读或引用服务端文档。
- 不本地推导战斗结果；只播服务端给的帧。
- 协议版本 `PROTO` 与 `hello.proto` 必须一致；收到关闭码 `4004` 提示升级，不重连。
- 中文宽度一律走 `unicode-width`（`src/util/text.rs`），不要用 `len()` 做对齐。
