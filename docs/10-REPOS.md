# 客户端 · 10 两仓操作手册（客户端视角）

```
doc_id: C-10
version: 3.0.0
updated: 2026-11-22
owns: 客户端仓的克隆、密钥、日常循环、与服务端的协议同步、构建发布与排错
depends_on: C-00、C-02、C-03、服务端 docs/05、服务端 docs/13
```

## 1. 拓扑

| 仓库 | 本地路径 | 远端 |
|---|---|---|
| 客户端（本仓） | `client/`（工作区根） | `git@github-client:NJHHBJSJKAsjdsasa/xiuxian-mud-client.git` |
| 服务端 | 同级 `../xianxia/` | `git@github-xiuxian:NJHHBJSJKAsjdsasa/xiuxian-mud.git` |

两个仓库完全分离，没有 submodule。协议是唯一耦合面。

## 2. 密钥

| 别名 | 私钥 | 用在 |
|---|---|---|
| `github-client` | `~/.ssh/id_ed25519_client` | 本仓（部署密钥，Allow write） |
| `github-xiuxian` | `~/.ssh/id_ed25519_xiuxian` | 服务端仓 |

- 部署密钥是**单仓**的：拿服务端那把推本仓一定 `Permission denied (publickey)`。
- `~/.ssh/config` 里必须有 `IdentitiesOnly yes`，否则 ssh 会先试别的 key。
- 本机 `github.com:443` 不通，走 SSH 22；HTTPS 推会超时。

## 3. 克隆与首次运行

```bash
git clone git@github-client:NJHHBJSJKAsjdsasa/xiuxian-mud-client.git client
cd client
python tools/doc-check.py     # 文档矩阵与链接
cargo build                   # 首次拉依赖（eframe 编译约 3–6 分钟）
cargo run -- --server ws://127.0.0.1:8080
```

联调需要服务端在跑（另一个仓库）：`go run ./cmd/server`，默认 `127.0.0.1:8080`。
客户端用配置项 `server` 指定地址；不带则读 `config.toml` 或默认本机。

Linux 首次编译要先装图形依赖（eframe 官方要求）：

```bash
sudo apt-get install libxkbcommon-dev libxcb-render0-dev libxcb-shape0-dev libxcb-xfixes0-dev
```

## 4. 日常循环

```bash
cd client
python tools/doc-check.py     # 文档（改文档必跑）
cargo fmt --check
cargo clippy --all-targets -- -D warnings
cargo test                    # 含 --test ui / --test replay
git add -A && git commit -m "feat: ..." && git push origin main
```

- 提交信息中文，`feat:` / `fix:` / `docs:` / `refactor:` 前缀。
- 改了 `docs/` 任一份，必须同步 `docs/00-INDEX.md` 的矩阵与版本，否则 `doc-check` 红。
- 改了协议消费方式（不是服务端字段），在提交信息里引用服务端对应提交号，例如 `server@e5effdf`。
- `Cargo.lock` 入库（二进制项目），不要加进 `.gitignore`。
- 改 UI 的 PR 附一张 `--snapshot` 截图（改前/改后各一张），比文字描述省时间。

## 5. 协议同步（与服务端对齐）

字段与数值的唯一来源是服务端仓库的 `docs/05` 与 `docs/04`；本仓 `C-03`/`C-06`/`C-07` 只写怎么用。

| 服务端变更 | 本仓要做什么 |
|---|---|
| 新增动作 / 新增可选字段 | 不用改，`serde` 忽略未知字段；按需补界面 |
| 字段改名 / 改含义 / 删除 | 服务端升 `proto`；本仓改消费代码与 `C-03`/`C-06` 对应章节，并升 `PROTO` 常量 |
| 新增关闭码 / 错误码 | 在 `C-03` 的处置表补一行，`protocol/err.rs` 补常量与 `match` 分支 |
| 数值变化（伤害、回合上限） | 客户端不内置阈值：本来就该从帧/文档读，不改代码 |

破坏性变更的兜底：连接时 `hello.proto` 越界会被服务端以 **4004** 断开；本仓必须**不重连**并提示升级。

## 6. 构建与发布

```bash
cargo build --release                                       # 本机
cargo build --release --target x86_64-pc-windows-msvc        # Windows
cargo build --release --target x86_64-unknown-linux-musl     # Linux（静态部分；图形库仍需系统提供）
cargo build --release --target aarch64-apple-darwin         # Apple Silicon
```

| 平台 | 产物 | 备注 |
|---|---|---|
| Windows | `xiuxian-client.exe`（约 12–20MB） | 用 `winres`/`embed-resource` 写入图标与版本信息 |
| macOS | `xiuxian-client.app` | `cargo-bundle` 打包；未签名时用户需右键打开或 `xattr -dr com.apple.quarantine` |
| Linux | 单二进制 + `xiuxian-client.desktop` | 不提供 AppImage 也能用；桌面文件只为图标与菜单项 |

- 字体与图标都编进二进制：发布**不需要**附带 `assets/` 目录。
- 发布前确认四件事：`PROTO` 落在服务端支持区间；默认 `server` 指向生产地址；release 包实测冷启动与待机 CPU；
  `doc-check`、`cargo test`、`clippy` 全绿。
- 回滚：换回上一个二进制；不涉及数据库。
- Windows 用户第一次运行可能触发 SmartScreen：文档里写明"更多信息 → 仍要运行"（不签名）。

## 7. 排错

| 现象 | 原因 | 处置 |
|---|---|---|
| 窗口一闪就退 | GPU 初始化失败（远程桌面/老驱动） | 命令行加 `--renderer glow`；仍失败看 `logs/panic.log` |
| 界面全黑但能听到操作 | wgpu 后端与驱动不兼容 | 同上切 glow；或更新显卡驱动 |
| 文字全是方块 | 字体子集没编进去 / 缺字 | 确认 `assets/fonts/*.otf` 入库；设置页看缺字码位清单 |
| 文字发虚 | 系统缩放与手动缩放叠加过头 | 设置里把手动档调回 100%，只留系统缩放 |
| 点击位置偏移 | 用了 `ctx.pixels_per_point` 手算坐标 | 布局一律用 `ui.available_width()` 等相对量，不写绝对像素 |
| 待机风扇转 | 某处无条件 `request_repaint()` | 全局搜 `request_repaint`，去掉无状态变化的那处 |
| `Permission denied (publickey)` | 用了服务端别名，或部署密钥没加/没勾写权限 | 查 `git remote -v` 与 `ssh -T git@github-client` |
| 推上去被拒（non-fast-forward） | 远端有新提交 | `git pull --rebase origin main` 后再推 |
| 连上就被断开（4004） | 客户端 `PROTO` 与服务端不兼容 | 升级客户端；不要改重连逻辑绕过 |
| 连上就被断开（4003） | token 失效 | 清 token 回登录 |
| 4001 | 同账号在别处登录 | 提示并停止重连 |
| `doc-check` 红 | 改了文档没同步矩阵/版本 | 按提示补 `C-00` 行与版本 |

## 8. 镜像说明

服务端仓 `docs/13-REPOS.md` 是本文档的镜像（服务端视角），两份各自保留最简命令，不互相复制全文：
服务端写"服务端怎么改怎么推"，本仓写"客户端怎么改怎么推"；共同部分（密钥、协议变更顺序）在两份里都保留最短可执行版本。
