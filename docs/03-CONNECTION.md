# 客户端 · 03 连接：握手、版本、重连

```
doc_id: C-03
version: 4.1.2
updated: 2026-11-24
owns: 登录流程、协议版本握手、心搏、重连状态机、关闭码处置、终端生命周期
depends_on: 服务端 05（P-REST-01/02、P-MSG-01/05、关闭码表）、12（快速开始）
```

## 1. 启动顺序

```
启动
 ├─ 读 playcli.json（server / lastName / color）
 ├─ 进备用屏（alternate screen），画登录页
 └─ 登录页等待输入（密码不落盘，不自动恢复 WS）
        │
        ├─ REST 登录     POST /api/login {name, pass} → 200 {token, hasChar}
        │      │
        │      ├─ WS 连接   ws(s)://<host>/ws （gorilla/websocket，握手 8 秒超时）
        │      │      │
        │      │      ├─ hello  {"t":"hello","d":{"token":"...","proto":1}}
        │      │      │
        │      │      ├─ ready  已建号 {proto, minProto, player, world, catalog, chatHist, serverNow}
        │      │      │            未建号 {proto, minProto, hasChar:false, catalog:null}
        │      │      │      │
        │      │      │      ├─ hasChar=false → 建号页（create → ready）
        │      │      │      └─ hasChar=true  → 主界面
        │      │      │
        │      │      └─ 4004 → 模态错误页"客户端版本不兼容"，只留退出
        │      │
        │      └─ 失败 → 登录页内联错误（不弹系统对话框）
```

- 登录页与主界面是**同一屏的两个页面**（`viewLogin / viewCreate / viewMain / viewFatal`），不反复进出备用屏。
- 登录中的按钮进入 loading 态（禁点 + 转圈），请求 8 秒超时后恢复可点并显示原因。
- `ready.serverNow` 用来算时钟偏移：`offset = serverNow - nowMs()`，之后所有倒计时都用服务端时间。

## 2. 协议版本（4004）

| 情况 | 服务端行为 | 客户端行为 |
|---|---|---|
| `proto` 在 `[minProto, proto]` | 正常 `ready` | 进入游戏 |
| 越界 | 先 `err`（`E-BADREQ`，正文带双方版本），再 `4004` 关闭 | **不重连**；切到 `viewFatal`，显示服务端支持区间与本机版本，只留"退出"和"检查更新" |

- 4004 与 4003 必须分开处理：4003 是 token 失效 → 清 token、回登录；4004 是客户端要更新 → 清 token 也没用。
- 收到 4004 后不要做退避重试，否则服务端日志里会刷出一串同样的握手失败。

## 3. 心搏与超时

- 服务端 1001 表示心跳超时/写失败：读循环超过 60 秒没有任何帧（含 ping 回包），视为半开连接。
- 客户端每 25 秒发一次 `{"t":"ping"}`；连续两次无响应，主动断开并按重连策略处理，不等 TCP 超时。
- 超时口径：`hello` 由服务端 5 秒计时；连接握手 8 秒；`ready` 不单设超时，走 60 秒读超时；REST 8 秒；动作不等回包，靠后续 `state` / `log` 对齐。
- 系统休眠/唤醒后 `nowMs()` 会跳变：唤醒时若 `nowMs() - lastFrameMs > 60s`，直接判定连接已死并重连，
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
| 4004 | 协议版本不兼容 | **不重连**，切 `viewFatal` |
| 4029 | 频率超限被踢 | 60 秒后重连；期间禁用输入，顶栏显示倒计时 |

重连成功后：

- 用新 `ready` **整体替换** `store` 里的玩家与服务端快照（不要合并旧值）。
- 秘境：服务端口径是 5 分钟内重连回同一层同进度；重连后拉一次 `dg info` 即回到界面。
- 实时切磋、队伍、赶路：服务端重启即散。重连后若 `live` / `party info` 为空，直接回普通界面并提示一句，不假装还在。

界面表现（细节在 `C-05` §7）：

- 顶栏状态灯：绿=在线、黄=重连中（带第 N 次）、红=已断开/致命错误。
- 重连期间所有动作控件置灰，输入框仍可打字（内容保留），但不发送。

## 5. 限流与背压

- `E-RATE` 按服务端回执处理：同一窗口最多回一条，收到后把动作置灰到冷却结束。
- 不内置第二份动作冷却表：以服务端 `err.code=E-COOLDOWN` 为准。
- 收到 `err` 且 `code=E-COOLDOWN` 时，用 `extra.remainMs` 刷新倒计时。
- 写通道有界（容量 64）：满了丢弃当前命令（含动作）并提示；不承诺动作必达。

## 6. 终端生命周期

- **退出**：按 `q`（或 `Ctrl+C`）时发 `{"t":"close"}` 或 WS Close(1000)，最多等 300ms，
  然后 `runtime.shutdown_timeout(200ms)`。不等待也能退出，但服务端会多留一条超时记录。
- **挂后台**：终端切走后仍按 250ms 重绘；不做省电降频（CPU 占用本就低于 1%）。
- **崩溃**：`defer` 里先恢复终端（`term.Restore` + 退出备用屏 + 显示光标），
  再把错误打到 stderr——终端程序不能弹窗，也不该留下一个乱码屏幕。

## 7. 实现落点（Go）

| 关注点 | 落点 | 说明 |
|---|---|---|
| 连接与重连 | `net.go` 的 `runNet` / `wsSession` | 一个 goroutine 管连接生命周期，重连时重建连接 |
| REST | `net.go` 的 `postJSON` / `login` / `logout` | 标准库 `net/http`，8s 超时；只三个端点 |
| 帧编解码 | `proto.go` 的 `inFrame` 与各视图结构 | `encoding/json` 直接映射，未知字段忽略 |
| 关闭码 | `proto.go` 的 `classifyClose` | 常量与 `switch`，不散落数字 |
| 唤醒 UI | `evs chan netEvent` | 网络只往有界队列（2048）里塞事件；主循环 select 到就重绘 |
| 超时 | `net.Dialer.HandshakeTimeout` + `lastRx` 检查 | 握手 8s；60s 没收到任何帧按心跳超时断开 |
| 登录态 | `keys.go` 的 `uiState` | `viewKind` + `loginForm`（loading / err / focus） |

## 8. 待确认的实现细节

- `ping` 是否需要业务层 `pong`：当前服务端以控制帧心跳为主，客户端按"两次无响应即重连"处理，不依赖业务层 pong。
- 断线期间用户输入：一律丢弃（动作栏置灰），避免重连后补发一串旧操作。
