# 客户端 · 03 连接：握手、版本、重连

```
doc_id: C-03
version: 3.0.0
updated: 2026-11-22
owns: 登录流程、协议版本握手、心搏、重连状态机、关闭码处置、窗口生命周期
depends_on: 服务端 05（P-REST-01/02、P-MSG-01/05、关闭码表）、12（快速开始）
```

## 1. 启动顺序

```
启动
 ├─ 读 config.toml（server / ui_scale / last_name / token 是否存在）
 ├─ 装字体与主题，开窗口（1280×800），显示登录页
 └─ 有 token 且勾了记住我 → 直接尝试 WS 恢复
        │
        ├─ REST 登录     POST /api/login {name, pass} → 200 {token, hasChar}
        │      │
        │      ├─ WS 连接   ws(s)://<host>/ws （tokio-tungstenite + rustls，5 秒超时）
        │      │      │
        │      │      ├─ hello  {"t":"hello","d":{"token":"...","proto":1}}
        │      │      │
        │      │      ├─ ready  {proto, minProto, hasChar, player?, catalog, serverNow, ...}
        │      │      │      │
        │      │      │      ├─ hasChar=false → 建号页（create → ready）
        │      │      │      └─ hasChar=true  → 主界面
        │      │      │
        │      │      └─ 4004 → 模态错误页"客户端版本不兼容"，只留退出
        │      │
        │      └─ 失败 → 登录页内联错误（不弹系统对话框）
```

- 登录页与主界面是**同一个窗口的两个页面**（`ui::View::Login / Create / Main / Fatal`），不频繁开关窗口。
- 登录中的按钮进入 loading 态（禁点 + 转圈），请求 8 秒超时后恢复可点并显示原因。
- `ready.server_now` 用来算时钟偏移：`offset = server_now - now_ms()`，之后所有倒计时都用服务端时间。
- `--replay frames.jsonl` 时跳过整个 REST/WS 流程，直接进主界面并回放（`C-08` §6）。

## 2. 协议版本（4004）

| 情况 | 服务端行为 | 客户端行为 |
|---|---|---|
| `proto` 在 `[minProto, proto]` | 正常 `ready` | 进入游戏 |
| 越界 | 先 `err`（`E-BADREQ`，正文带双方版本），再 `4004` 关闭 | **不重连**；切到 `View::Fatal`，显示服务端支持区间与本机版本，只留"退出"和"检查更新" |

- 4004 与 4003 必须分开处理：4003 是 token 失效 → 清 token、回登录；4004 是客户端要更新 → 清 token 也没用。
- 收到 4004 后不要做退避重试，否则服务端日志里会刷出一串同样的握手失败。

## 3. 心搏与超时

- 服务端 1001 表示心跳超时/写失败：读循环超过 60 秒没有任何帧（含 ping 回包），视为半开连接。
- 客户端每 25 秒发一次 `{"t":"ping"}`；连续两次无响应，主动断开并按重连策略处理，不等 TCP 超时。
- 所有等待都要有超时：`hello` 5 秒、`ready` 10 秒、REST 8 秒、单次动作 8 秒（超时按"未知结果"处理，刷新 `state` 对齐）。
- 系统休眠/唤醒后 `now_ms()` 会跳变：唤醒时若 `now_ms() - last_frame_ms > 60s`，直接判定连接已死并重连，
  不等 ping 超时（笔记本合盖一晚上回来，界面不能停在"在线"）。

## 4. 重连状态机

```
CONNECTING ──open──► HELLO ──ready──► ONLINE ──close(code)──► BACKOFF
     ▲                                  │                        │
     └──────────── 计时到 ◄─────────────┴────────────────────────┘
```

| 关闭码 | 含义 | 处置 |
|---|---|---|
| 1000 | 正常关闭（主动退出） | 不重连，回登录页 |
| 1001 | 心跳超时 / 写失败 | 指数退避：1s → 2s → 4s → 8s，上限 30s |
| 1009 | 帧过大 | 不重连，提示并记录日志（客户端发了异常帧，属 bug） |
| 1011 | 服务端内部错误（读档失败等） | 退避重连；连续 3 次提示"服务异常，稍后再试" |
| 4001 | 顶号 | **不重连**，提示"账号在别处登录"，回登录页 |
| 4003 | token 失效 | 清 token，回登录 |
| 4004 | 协议版本不兼容 | **不重连**，切 `View::Fatal` |
| 4029 | 频率超限被踢 | 60 秒后重连；期间禁用输入，顶栏显示倒计时 |

重连成功后：

- 用新 `ready` **整体替换** `store` 里的玩家与服务端快照（不要合并旧值）。
- 秘境：服务端口径是 5 分钟内重连回同一层同进度；重连后拉一次 `dg info` 即回到界面。
- 实时切磋、队伍、赶路：服务端重启即散。重连后若 `live` / `party info` 为空，直接回普通界面并提示一句，不假装还在。

界面表现（细节在 `C-05` §7）：

- 顶栏状态灯：绿=在线、黄=重连中（带第 N 次）、红=已断开/致命错误。
- 重连期间所有动作控件置灰，输入框仍可打字（内容保留），但不发送。

## 5. 限流与背压

- `E-RATE` 静默处理：连点时不要弹提示，只把动作置灰到冷却结束。
- 客户端自带动作冷却表（从 `05` 抄），本地先拦一刀，减少无意义的 `err`。
- 收到 `err` 且 `code=E-COOLDOWN` 时，用 `extra.retryMs`（若有）或本地表刷新倒计时。
- 写通道有界（容量 64）：满了就丢弃重复的 `ping`，但**动作帧不允许丢**——丢动作比慢更糟。

## 6. 窗口生命周期

- **关窗**：`eframe` 的 `on_exit` 里发 `{"t":"close"}` 或 WS Close(1000)，最多等 300ms，
  然后 `runtime.shutdown_timeout(200ms)`。不等待也能退出，但服务端会多留一条超时记录。
- **最小化**：默认不降频渲染（egui 本来就不渲染隐藏窗口的帧）；托盘图标不在首版范围。
- **崩溃**：装 `std::panic::set_hook`，把 panic 信息写 `logs/panic.log`，再用 `rfd` 弹原生错误框，
  避免"窗口一闪就没了"没有任何线索。编译期开 `panic = "abort"` 的 release profile 只用于最终包，
  开发包保留 unwind 以便看到 backtrace。

## 7. 实现落点（Rust）

| 关注点 | 落点 | 说明 |
|---|---|---|
| 连接与重连 | `src/net/ws.rs` | 一个 task 管连接生命周期，重连时重建流与任务 |
| REST | `src/net/http.rs` | `reqwest` + rustls；只三个端点 |
| 帧编解码 | `src/protocol/frame.rs` | `In { t, n, a, d }` / `Out`；`serde_json` 直接映射 |
| 关闭码 | `src/protocol/err.rs` | 常量与 `match`，不散落数字 |
| 唤醒 UI | `src/net/ws.rs` | 每帧 `ctx.request_repaint()`；批量事件合并成一次唤醒 |
| 超时 | `tokio::time::timeout` | hello / ready / REST / 单动作各有独立超时 |
| 登录态 | `src/store/mod.rs` + `src/ui/views/login.rs` | `View` 枚举 + `LoginState`（Idle/Loading/Error） |

## 8. 待确认的实现细节

- `ping` 是否需要业务层 `pong`：当前服务端以控制帧心跳为主，客户端按"两次无响应即重连"处理，不依赖业务层 pong。
- 断线期间用户输入：一律丢弃（动作栏置灰），避免重连后补发一串旧操作。
