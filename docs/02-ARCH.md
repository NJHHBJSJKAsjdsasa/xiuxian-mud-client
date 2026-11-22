# 客户端 · 02 架构

```
doc_id: C-02
version: 4.0.0
updated: 2026-11-22
owns: 技术选型、crate 清单、分层、目录结构、线程模型、模块依赖规则
depends_on: C-01、C-03、C-04、C-05
```

## 1. 分层

```
┌──────────────────────────────────────────┐
│ tui/     ratatui 绘制 + crossterm 事件    │  只读 view/store，不发请求
├──────────────────────────────────────────┤
│ app/     状态机：按键 → Intent → 网络命令 │  网络事件 → store → 触发重绘
├──────────────────────────────────────────┤
│ view/    页面与输入状态（纯数据）          │  不含任何渲染框架类型
├──────────────────────────────────────────┤
│ store/   客户端状态（单一可变源）          │  由服务端帧驱动，不反向推导
├──────────────────────────────────────────┤
│ net/     REST 登录 + WS 长连接            │  只认协议，不懂业务
├──────────────────────────────────────────┤
│ protocol/ 帧与视图结构                    │ 与 05-PROTOCOL 一一对应
└──────────────────────────────────────────┘
```

依赖方向：`tui → app → {view, store, net} → protocol`。下层不引上层。

## 2. crate 清单

| crate | 用途 | 备注 |
|---|---|---|
| `ratatui` 0.29 | 布局、段落、样式、离屏 `TestBackend` | 只画字符，不碰显卡 |
| `crossterm` 0.28 | 原始模式、按键事件、备用屏 | 三平台一致 |
| `tokio` | 异步运行时 | 网络线程 |
| `tokio-tungstenite` + `rustls` | WebSocket | 不依赖系统 OpenSSL |
| `reqwest`（rustls, json） | 登录/注册/登出 | 只发三次 HTTP |
| `serde` / `serde_json` | 协议编解码 | 未知字段忽略 |
| `crossbeam-channel` | 网络 → UI 的事件队列 | 有界 4096，满了丢 |
| `toml` | `config.toml` | 服务器地址、上次道号 |
| `tracing` | 日志 | 写到 stderr，不干扰画面 |
| `time` | 日期格式 | 倒计时与时刻 |
| `unicode-width` | 显示列宽 | 与 ratatui 同一套算法，中文按 2 列对齐 |
| `dirs` | 配置目录 | 三平台 |

依赖里没有渲染后端、图形库、资源打包工具：`cargo tree` 只有上面这些 crate，
终端能力全部来自 `crossterm`；字形、列宽、颜色都由终端自己决定。

## 3. 目录

```
client/
├── Cargo.toml
├── Cargo.lock             入库（二进制项目）
├── docs/                  设计文档（本目录）
├── tools/doc-check.py     文档自查
├── tools/ansi2png.py      --snapshot 的 ANSI 转 PNG
└── src/
    ├── main.rs            参数、配置、日志、进 tui::run 或 tui::snapshot
    ├── config.rs          config.toml 读写
    ├── app.rs             状态机：drain_net / on_key / intent / tick
    ├── view.rs            View / Page / Intent / UiState / Toast（纯数据）
    ├── protocol/
    │   ├── frame.rs       帧信封
    │   ├── action.rs      动作名常量
    │   ├── msg.rs         服务端视图（state/map/battle/ready…）
    │   └── err.rs         错误码与关闭码
    ├── net/
    │   ├── mod.rs         NetCmd / NetEvent / ConnStatus
    │   ├── ws.rs          连接、hello、心跳、重连状态机
    │   └── http.rs        register / login / logout
    ├── store/
    │   ├── mod.rs         AppState（单一可变源）
    │   ├── player.rs      角色
    │   ├── world.rs       节点
    │   ├── mapgraph.rs    已探索地图（节点/边/布局）
    │   └── logbuf.rs      日志环形缓冲（5000 行）
    ├── tui/
    │   ├── mod.rs         终端初始化、事件循环、离屏快照
    │   ├── render.rs      七段布局、数据页、舆图、Toast
    │   ├── scene.rs       ASCII 场景生成（山/雨/人影/地面）
    │   └── theme.rs       颜色、箭头、字符血条
    └── util/
        ├── time.rs        服务端时间偏移、倒计时
        └── text.rs        截断、千分位
```

## 4. 线程模型

- **主线程**：crossterm 事件轮询（30ms 超时）→ `app.on_key` → 绘制一帧。
  没有渲染线程、没有 vsync，终端自己决定刷屏频率。
- **网络线程**：tokio 多线程运行时，`net::ws::run` 一个任务串起登录、握手、心跳、重连。
- **两条通道**：`NetCmd` 用 tokio unbounded（UI 同步发、网络异步收）；
  `NetEvent` 用 crossbeam 有界 4096（网络发、UI 每帧 try_recv）。
- **节奏**：主循环固定 30ms 一轮，网络事件与按键都在这一轮处理；一次最多消费 512 条事件，避免一帧卡死。
- 断线重连在**网络线程**里做退避，主线程只负责把状态显示成"重连中"。

## 5. 字符网格的写法约束

- **全量重绘**：每轮画整屏。终端只有字符，增量更新的收益抵不过状态同步的复杂度。
- **布局用 `Layout`**：七段固定；场景段用 `Min(5)` 吃掉剩余高度，窄终端不乱。
- **只用 ASCII 画场景**：`/ \ _ : . = ~ ^ | * o`，保证任何终端等宽对齐；
  中文只出现在 UI 文案里，由 ratatui 按 unicode-width 排版。
- **宽字符**：中文占两列。需要定宽对齐时用 `pad_to`（按显示宽度补空格），
  不用 `format!("{:<8}")`（它按字符数补）。
- **不阻塞**：事件轮询 30ms 超时；网络从不 `await` 在 UI 线程里。

## 6. 终端兼容与降级

- 需要 UTF-8 与备用屏（`EnterAlternateScreen`）；Windows Terminal / iTerm2 / xterm 均满足。
- 真彩优先；`COLORTERM` 不是 `truecolor` 时 ratatui 自动降到 256 色，观感变差但可用。
- 终端小于 80×24 时不报错，只把场景压到 5 行、见闻压到 3 行。
- 退出时恢复原始模式与光标（`disable_raw_mode` + `LeaveAlternateScreen`），
  异常路径也走 `Drop` 兜底。

## 7. 字体与宽度

- **不打包字体**：用终端自己的等宽字体。要求等宽 CJK（更纱黑体、等距更纱、
  Menlo、Consolas + 中文回退均可）。
- 中文宽度按 2 列计算，与 ratatui 一致；不做全角/半角转换。
- 不需要 IME 处理：终端负责输入法，客户端只收已上屏字符。

## 8. 颜色令牌（唯一色源）

`tui/theme.rs` 是唯一的色源，别处不写 RGB 字面量。

| 常量 | 值 | 用途 |
|---|---|---|
| `BG` | `#050709` | 底色 |
| `PANEL` | `#0A0D10` | 顶栏/提示条 |
| `TEXT` | `#E2E2D6` | 正文 |
| `DIM` / `FAINT` | `#78828A` / `#464E56` | 弱化两级 |
| `ACCENT` | `#7FD1C1` | 交互、真气 |
| `GOLD` | `#D9B36A` | 收益、修为、地名 |
| `BAD` | `#C4553D` | 损失、气血、错误 |
| `GOOD` | `#6FBF73` | 回复、成功 |
| `WARN` | `#C9A227` | 重连、提示 |

日志类型到颜色的映射只在 `theme::tone` 一处。

## 9. 配置与本地文件

| 文件 | 位置 | 内容 |
|---|---|---|
| `config.toml` | `dirs::config_dir()/xiuxian-client/` | `server`、`last_name`、`show_timestamps` |
| `token` | 同目录，权限 600 | 登录 token；勾"记住我"才写 |
| `logs/client.log` | `dirs::data_local_dir()/xiuxian-client/logs/` | tracing 滚动日志（7 天） |
| `logs/decode.log` | 同上 | 解析失败的原始帧（最多 10 条/会话） |

命令行覆盖：`--server`、`--login`、`--create`、`--page`、`--move`、`--snapshot`、`--log-level`。

## 10. 关键取舍

| 取舍 | 选择 | 理由 |
|---|---|---|
| UI 框架 | ratatui（立即模式 + 双缓冲） | 布局、段落、颜色都是现成的；`TestBackend` 能离屏出图 |
| 布局 | 七段 `Layout::vertical` | 终端里固定结构最稳；不做浮动面板 |
| 场景 | 纯函数生成字符网格 | 无资源、可测试；同一尺寸下结果确定 |
| 日志 | 环形缓冲 5000 行 + 每帧只渲染可见行 | `C-08` 的上限 |
| 图表 | 不做 | 终端画不出有意义的曲线；M5 用文本表格代替 |
| 错误处理 | 网络/JSON 全 `Result`；UI 只展示可读文案 | panic 会打乱终端状态（原始模式 + 备用屏），必须先恢复再退出 |
| 发布 | 单二进制，无资源 | 拷走就能跑，SSH 也能跑 |

## 11. 与服务端的一致性

- 协议版本：`protocol::PROTO = 1`，`hello` 回带；收到 4004 停止重连并提示升级。
- 动作名/错误码：从 `protocol::action` / `protocol::err` 常量引用，不写字符串字面量。
- 帧字段：允许服务端新增字段（`serde` 默认忽略）；**字段改名视为破坏性变更**，要升 `proto`。
- 时间：所有倒计时用服务端时间（`offset = ready.server_now - now_ms()`），不读本地时区。
