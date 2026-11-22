# 客户端 · 02 架构

```
doc_id: C-02
version: 3.0.0
updated: 2026-11-22
owns: 技术选型、crate 清单、分层、目录结构、线程模型、资源与字体、模块依赖规则
depends_on: C-01、C-03、C-04、C-05
```

## 1. 分层

```
┌──────────────────────────────────────────┐
│ ui/      egui 绘制与输入（主线程）        │  只读 store，只发 Intent
├──────────────────────────────────────────┤
│ app/     eframe::App::update（每帧一次）  │  drain 事件 → 更新 store → 画一帧
├──────────────────────────────────────────┤
│ store/   客户端状态（单一可变源）         │  由服务端帧驱动，不反向推导
├──────────────────────────────────────────┤
│ net/     连接、握手、重连（tokio 线程）   │  只认协议，不懂业务语义
└──────────────────────────────────────────┘
        ↑ 全部依赖 protocol/（serde 类型）
```

**依赖规则（违反即 review 不通过）**

1. `ui` 不得直接读写 socket，也不得自己发动作；只能发 `Intent`，由 `app` 转成 `NetCmd`。
2. `net` 不得读业务状态；只负责"连、握手、收发、断线重连、关闭码处置"。
3. `store` 是唯一可变状态；`ui` 通过 `&AppState` 只读渲染。**`store` 里不出现任何 `egui` 类型**（含 `Color32`、`RichText`）。
4. 协议类型只在 `protocol/` 定义，字段名与服务端 `05` 一致，不改名。
5. 任何 `unwrap()` 只能在"不可能失败"处出现，并在注释里说明为什么不可能；网络与 JSON 路径一律 `Result`。
6. `ui` 代码不做阻塞调用：登录/建号等请求一律走 `NetCmd`，结果以 `NetEvent` 回来。
7. 每帧只做与"可见内容"成正比的工作：日志按可见行构建，表格按可视区取行（`C-08` §3）。

## 2. crate 清单

| crate | 用途 | 备注 |
|---|---|---|
| `eframe` 0.37（`wgpu` 后端） | 窗口、事件循环、渲染 | 需要 Rust ≥ 1.95、edition 2024 |
| `egui` | 控件与布局 | 与 eframe 同 minor，锁版本 |
| `egui_extras` | 表格（`TableBuilder`）、日期/图片加载 | 与 egui 同版本 |
| `egui_plot` | 交易行价格曲线 | 与 egui 同版本 |
| `tokio`（rt-multi-thread, macros, time, sync） | 运行时、定时器、通道 | 不引入 `full`，按需开 feature |
| `tokio-tungstenite` + `rustls` | WebSocket 客户端 | 不依赖系统 OpenSSL |
| `serde` / `serde_json` | 帧与载荷 | 未知字段默认忽略 |
| `reqwest`（rustls, json） | `register` / `login` / `logout` | 只在 `net/http.rs` 用 |
| `crossbeam-channel` | UI ↔ 网络线程通信 | 比 `std::mpsc` 多 select 能力 |
| `anyhow` / `thiserror` | 错误上下文 | 网络层分类，UI 层兜底 |
| `tracing` + `tracing-subscriber` | 日志落文件 | 排错靠它，不靠 println |
| `dirs` | 配置与日志目录 | `config.toml`、token |
| `toml` | 配置读写 | 只读写一个文件 |
| `time` | 时间格式化 | 不用 `chrono`，只要 `HH:MM:SS` |
| `egui_kittest`（dev） | UI 快照与交互测试 | 官方 UI 测试工具，随 egui 版本 |

不引入：`arboard`（egui 自带复制/粘贴）、`chrono`、任何 WebView / JS 运行时、任何图标字体。

## 3. 目录

```
client/
├── Cargo.toml
├── Cargo.lock             入库（二进制项目）
├── docs/                  设计文档（本目录）
├── assets/
│   └── fonts/             内置中文字体子集（SIL OFL，随包附 LICENSE）
├── tools/doc-check.py     文档自查
└── src/
    ├── main.rs            启动、配置、日志、字体、eframe::run_native
    ├── app.rs             eframe::App 实现：drain → Intent → NetCmd → 绘制
    ├── protocol/
    │   ├── frame.rs       In / Out 帧、t / a / n / d
    │   ├── action.rs      53 个动作名常量
    │   ├── msg.rs         19 种服务端消息（serde tag = "t"）
    │   ├── err.rs         错误码与关闭码
    │   └── view.rs        state / map / battle / live 等视图结构
    ├── net/
    │   ├── mod.rs         NetCmd / NetEvent
    │   ├── ws.rs          连接、hello、心搏、重连状态机
    │   └── http.rs        register / login / logout
    ├── store/
    │   ├── mod.rs         AppState、revision、订阅
    │   ├── player.rs      角色 / 背包 / 技能
    │   ├── world.rs       节点 / 分线 / 天气
    │   ├── fight.rs       战斗与实时切磋
    │   ├── social.rs      聊天 / 好友 / 邮件 / 队伍 / 宗门 / 市场
    │   └── logbuf.rs      日志环形缓冲（5000 行）
    ├── ui/
    │   ├── mod.rs         root：顶栏 + 导航 + 主视图 + 输入栏
    │   ├── theme.rs       颜色 / 字号 / 圆角 / 间距令牌
    │   ├── fonts.rs       FontDefinitions 装载与回退链
    │   ├── icons.rs       自绘图标（Painter 几何图形，不用图标字体）
    │   ├── logview.rs     日志虚拟滚动、自动贴底、右键复制
    │   ├── widgets/       状态条、出口行、血条、倒计时环、Toast、确认框
    │   └── views/         login / create / fatal / game / character / bag / skills / social / market / sect / map / settings
    └── util/
        ├── time.rs        服务端时间偏移、倒计时
        └── text.rs        截断、数字千分位、中文标点规整
```

## 4. 线程模型

```
主线程（winit / eframe）                         后台线程（tokio runtime）
┌───────────────────────────────┐                ┌───────────────────────────┐
│ update()                      │  NetCmd  ────► │ ws 写循环                 │
│   1. drain NetEvent           │                │ ws 读循环 → NetEvent      │
│   2. 更新 store / 生成 Intent │  ◄──── NetEvent│ tick（1s / 倒计时）        │
│   3. ui::root(ctx, &state)    │                │ http（登录 / 建号）        │
│   4. request_repaint 策略     │                └───────────────────────────┘
└───────────────────────────────┘
```

- **egui 必须在主线程**：`eframe::run_native` 持有窗口与事件循环，所有 `ui` 代码都在 `update()` 里跑。
- **tokio 在后台**：`main` 里建 runtime 并 `runtime.spawn(net::run(rx_cmd, tx_event, ctx.clone()))`；线程不退出，随进程结束。
- **唤醒**：网络线程收到帧后 `ctx.request_repaint()`。`egui::Context` 是 `Arc` 且 `Send + Sync`，可以跨线程 clone。
- **不共享锁**：`store` 只在主线程读写；网络线程不持有 `&mut AppState`。事件携带已解析的 `protocol::msg::In`，合并逻辑全在主线程。
- **背压**：`NetCmd` 通道有界 64；`NetEvent` 有界 1024，满了说明 UI 卡住（属 bug，记 warn 并丢弃 `log` 类事件，**不丢 `state` / `battle` / `over`**）。

## 5. 即时模式的写法约束

egui 每帧重跑整棵 UI 代码，所以"状态"必须留在 `AppState` 与少量 UI 局部字段里，不能指望控件自己保存：

| 数据 | 放哪 | 说明 |
|---|---|---|
| 业务数据 | `store::AppState` | 服务端帧驱动，UI 只读 |
| 当前页 / 滚动位置 / 筛选 | `ui::UiState` | 纯界面状态，不进 `store` |
| 输入框文本 / 焦点 | `ui::UiState` 的 `String` + `egui::Id` | `TextEdit` 绑定 `&mut String` |
| 每行日志的排版结果 | `ui::logview` 的按行缓存 | 缓存 `Arc<Galley>`，避免每帧重排 5000 行 |
| 动画进度（战报淡入） | `ui::UiState` 的 `f32` + 起始时刻 | 用 `ctx.input(|i| i.time)` 算，不用 wall clock |

硬规则：

1. 不在 `update()` 里 `sleep`、`block_on`、读文件（配置只在启动与显式保存时读写）。
2. 不用 `std::thread::spawn` 起临时线程做业务；需要异步就走 `NetCmd`。
3. 控件 ID 显式给：列表行用 `ui.push_id(key, ...)`，否则滚动时状态会串行。
4. 长列表一律虚拟化：`ScrollArea::show_rows` / `TableBuilder::body.rows`，不 `for` 全量建控件。

## 6. 渲染后端与兜底

- 默认 `wgpu`（Windows→DX12/Vulkan，macOS→Metal，Linux→Vulkan/GL）。
- 老机器或远程桌面：`--renderer glow` 走 OpenGL 3.3；仍失败则启动时弹原生错误框，提示更新显卡驱动。
- 不静默降级到软件渲染：软件渲染下 60fps 撑不住，体验比报错更糟。

## 7. 资源与字体

- **中文必须内置**：egui 0.37 起 `default_fonts` 不再是默认 feature，且系统字体在不同平台差异大。
  仓库内放 `assets/fonts/NotoSansSC-Regular.subset.otf` 与 `Medium.subset.otf`，用 `include_bytes!` 编进二进制。
- 子集范围：GB2312 一级+二级汉字（6763）、ASCII、常用中文标点、`·—…「」《》`、`⏵ ✕ ✓` 等界面符号；预估 2–3MB/字重。
  裁剪命令记在 `assets/fonts/README.md`（`pyftsubset --text-file=charset.txt`）。
- 回退链：`SC Regular` → egui 默认字体（拉丁与符号）→ 系统字体。缺字时不许出现"豆腐块"：`ui::fonts` 启动自检，
  用一段覆盖全字符集的样例文本测 `ctx.fonts` 的 `has_glyph`，缺字记 warn 并列出码位。
- 许可证：SIL OFL 1.1，`assets/fonts/LICENSE.txt` 随仓库；打包时不需要额外分发字体文件（已内嵌）。

## 8. 主题令牌（唯一色源）

`ui/theme.rs` 定义语义色，`ui` 各处只引令牌，不写字面量：

| 令牌 | 深色（默认） | 用途 |
|---|---|---|
| `bg` | `#14161A` | 窗口底 |
| `panel` | `#1B1E24` | 面板/卡片 |
| `panel_hi` | `#232830` | 悬停/选中 |
| `line` | `#2E343D` | 分隔线、边框 |
| `text` | `#E6E8EB` | 正文 |
| `text_dim` | `#9AA0A6` | 次要信息 |
| `accent` | `#45C8DC` | 主色（青，交互/高亮） |
| `good` | `#4CAF50` | 正向（气血、成功） |
| `bad` | `#E5484D` | 负向（受伤、错误） |
| `warn` | `#C9A227` | 旁白、警告、门槛不足 |
| `gold` | `#D4A64A` | 灵石、奖励 |

日志类型到令牌的映射固定在 `ui/logview.rs`：`sys→text_dim`、`nar→warn`、`combat→text`、`good→good`、`bad→bad`。
浅色主题在 M5 提供，令牌名不变，只换值。

## 9. 配置与本地文件

| 文件 | 位置 | 内容 |
|---|---|---|
| `config.toml` | `dirs::config_dir()/xiuxian-client/` | `server`、`ui_scale`、`theme`、`log_timestamps`、`last_name` |
| `token` | 同目录，权限 600 | 登录 token；勾"记住我"才写 |
| `logs/client.log` | `dirs::data_local_dir()/xiuxian-client/logs/` | tracing 滚动日志（7 天） |
| `logs/decode.log` | 同上 | 解析失败的原始帧（最多 10 条/会话） |

命令行覆盖：`--server`、`--renderer`、`--replay`、`--snapshot`、`--log-level`。

## 10. 关键取舍

| 取舍 | 选择 | 理由 |
|---|---|---|
| UI 框架 | egui（即时模式） | 面板多但结构浅；不需要组件树与消息映射；帧驱动天然对齐 |
| 布局 | 手写 `SidePanel` / `CentralPanel` / `TopBottomPanel` | 比引入 docking 库稳定；M5 再考虑可拖拽分栏 |
| 日志 | 环形缓冲 5000 行 + 可见行排版缓存 | `C-08` 的上限；不整屏重排 |
| 图标 | `Painter` 画几何图形 | 不额外带图标字体；缩放不失真 |
| 图表 | `egui_plot` | 价格曲线只需要折线与悬停提示 |
| 错误处理 | 网络/JSON 全 `Result`；UI 只展示可读文案 | panic 会让窗口直接消失，比终端更难看 |
| 发布 | 单 exe + 内嵌字体与图标 | 用户侧零安装；不需要资源目录 |

## 11. 与服务端的一致性

- 协议版本：`protocol::PROTO = 1`，`hello` 回带；收到 4004 停止重连并提示升级。
- 动作名/错误码：从 `protocol::action` / `protocol::err` 常量引用，不写字符串字面量。
- 帧字段：允许服务端新增字段（`serde` 默认忽略）；**字段改名视为破坏性变更**，要升 `proto`。
- 时间：所有倒计时用服务端时间（`offset = ready.server_now - now_ms()`），不读本地时区。
