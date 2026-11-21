# 客户端 · 02 架构

```
doc_id: C-02
version: 2.0.0
updated: 2026-11-21
owns: 技术选型、crate 清单、分层、目录结构、模块间依赖规则
depends_on: C-01、C-03、C-04
```

## 1. 分层

```
┌─────────────────────────────────────┐
│ ui/      终端渲染与输入（ratatui）    │  只读 store，只发 Intent
├─────────────────────────────────────┤
│ app/     事件循环（select! 三路）     │  键盘 / 网络 / tick
├─────────────────────────────────────┤
│ store/   客户端状态（单一源）         │  由服务端帧驱动，不反向推导
├─────────────────────────────────────┤
│ net/     连接、握手、重连（tokio）    │  只认协议，不懂业务语义
└─────────────────────────────────────┘
        ↑ 全部依赖 protocol/（serde 类型）
```

**依赖规则（违反即 review 不通过）**

1. `ui` 不得直接读写 socket；只能发 `Intent`，由 `app` 转成动作帧。
2. `net` 不得读业务状态；只负责"连、握手、收发、断线重连、关闭码处置"。
3. `store` 是唯一可变状态；`ui` 通过 `&AppState` 只读渲染。
4. 协议类型只在 `protocol/` 定义，字段名与服务端 `05` 一致，不改名。
5. 任何 `unwrap()` 只能在"不可能失败"处出现，并在注释里说明为什么不可能；网络与 JSON 路径一律 `Result`。

## 2. crate 清单

| crate | 用途 | 备注 |
|---|---|---|
| `tokio`（rt-multi-thread, macros, time, sync） | 运行时、定时器、通道 | 不引入 `full`，按需开 feature |
| `tokio-tungstenite` + `rustls` | WebSocket 客户端 | 不依赖系统 OpenSSL |
| `serde` / `serde_json` | 帧与载荷 | 未知字段默认忽略 |
| `ratatui` + `crossterm` | TUI 渲染、键鼠事件 | 首版锁定到同一大版本 |
| `unicode-width` | CJK 宽度 | 中文对齐的成败在这一处 |
| `reqwest`（rustls, json） | `register` / `login` / `logout` | 只用于三个 REST |
| `anyhow` / `thiserror` | 错误上下文 | 网络层用 thiserror 分类，UI 层用 anyhow 兜底 |
| `tracing` + `tracing-appender` | 日志落文件 | 排错靠它，不靠 println |
| `arboard`（可选，M3） | 复制道号 / 战报 | 失败要静默降级 |
| `dirs` | 配置与日志目录 | 存 token 与服务器地址 |

## 3. 目录

```
client/
├── Cargo.toml
├── Cargo.lock            入库（二进制项目）
├── docs/                 设计文档（本目录）
├── tools/doc-check.py    文档自查
└── src/
    ├── main.rs           启动、配置、日志、进入 app::run
    ├── protocol/
    │   ├── frame.rs      In / Out 帧、t / a / n / d
    │   ├── action.rs     53 个动作名常量
    │   ├── msg.rs        19 种服务端消息（serde tag = "t"）
    │   ├── err.rs        错误码与关闭码
    │   └── view.rs       state / map / battle / live 等视图结构
    ├── net/
    │   ├── mod.rs        NetCmd / NetEvent
    │   ├── ws.rs         连接、hello、心搏、重连状态机
    │   └── http.rs       register / login / logout
    ├── store/
    │   ├── mod.rs        AppState、订阅与版本号
    │   ├── player.rs     角色 / 背包 / 技能
    │   ├── world.rs      节点 / 分线 / 天气
    │   ├── fight.rs      战斗与实时切磋
    │   └── social.rs     聊天 / 好友 / 邮件 / 队伍 / 宗门 / 市场
    ├── app/
    │   ├── mod.rs        tokio::select! 事件循环
    │   ├── intent.rs     用户意图 → 动作帧
    │   └── input.rs      键位映射
    ├── ui/
    │   ├── mod.rs        draw()
    │   ├── layout.rs     主布局与标签页
    │   ├── log.rs        日志环形缓冲与富文本
    │   ├── map.rs        节点图（ASCII / 方块字符）
    │   ├── battle.rs     战斗面板
    │   └── panels/       角色 / 背包 / 技能 / 社交 / 市场 / 宗门 / 拍卖
    └── util/
        ├── time.rs       服务端时间偏移、倒计时
        └── text.rs       CJK 截断与填充
```

## 4. 并发模型

```
        ┌────────────┐   NetEvent (mpsc)   ┌────────────┐
        │ net::ws    │ ──────────────────► │ app loop   │
        │ read/write │ ◄────────────────── │ select!    │
        └────────────┘   NetCmd  (mpsc)    └─────┬──────┘
                                                 │ 更新
                                           ┌─────▼──────┐
                                           │ store      │
                                           └─────┬──────┘
                                                 │ &AppState
                                           ┌─────▼──────┐
                                           │ ui::draw   │ 每 16–33ms
                                           └────────────┘
```

- `net::ws` 内部两条任务：读循环（解析帧 → `NetEvent`）、写循环（`NetCmd` → socket）。
- `app` 用 `tokio::select!` 三路：键盘事件（阻塞式读放到 `spawn_blocking` 或单独线程）、`NetEvent`、`tick`。
- 渲染与逻辑分离：`tick` 只推进倒计时/动画帧号，`draw` 读取当前状态画一帧。

## 5. 关键取舍

| 取舍 | 选择 | 理由 |
|---|---|---|
| UI 框架 | ratatui（即时模式） | 面板多但结构简单；即时模式不用维护组件树 |
| 节点图 | ASCII/方块字符 + 颜色 | 终端里画线代价低；鼠标命中用坐标换算 |
| 日志 | 环形缓冲（VecDeque，5000 行） | 与 `C-08` 的上限一致，O(1) 淘汰 |
| 配置 | `config.toml` + 环境变量覆盖 | 服务器地址、token 存储位置、日志级别 |
| token | 本地文件（权限 600）+ 可选不落盘 | TUI 没有浏览器存储；落盘要显式提示 |
| 错误处理 | 网络/JSON 全 `Result`；UI 只展示可读文案 | panic 会毁掉整个终端界面 |

## 6. 与服务端的一致性

- 协议版本：`proto::PROTO = 1`，`hello` 回带；收到 4004 停止重连并提示升级。
- 动作名/错误码：从 `protocol::action` / `protocol::err` 常量引用，不写字符串字面量。
- 帧字段：允许服务端新增字段（`serde` 默认忽略）；**字段改名视为破坏性变更**，要升 `proto`。
- 时间：所有倒计时用服务端时间（`offset = ready.server_now - SystemTime::now()`），不读本地时区。
