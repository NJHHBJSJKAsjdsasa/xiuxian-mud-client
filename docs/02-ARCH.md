# 客户端 · 02 架构

```
doc_id: C-02
version: 5.4.0
updated: 2026-11-24
owns: 技术选型、依赖、分层、目录结构、并发模型、模块依赖规则
depends_on: C-01、C-03、C-04、C-05
```

## 1. 分层

```
┌────────────────────────────────────────────┐
│ render / pages / widgets  绘制盒子与页面    │  只读 state，不发请求
├────────────────────────────────────────────┤
│ app      状态机：按键 → 意图 → 网络命令     │  事件 → state → 触发重绘
├────────────────────────────────────────────┤
│ keys     Escape 序列切分、视图状态、toast   │  纯数据，无终端依赖
├────────────────────────────────────────────┤
│ state    客户端状态（单一可变源）           │  由服务端帧驱动，不反向推导
├────────────────────────────────────────────┤
│ net      REST 登录 + WS 长连接              │  只认协议，不懂业务
├────────────────────────────────────────────┤
│ proto    帧与视图结构                       │  与 05-PROTOCOL 一一对应
├────────────────────────────────────────────┤
│ screen   字符屏与 ANSI 输出                 │  宽字符、脏行、颜色档位
└────────────────────────────────────────────┘
```

依赖方向：`render → {app, state, widgets, screen}`，`app → {state, net, keys}`，
`net → proto`，`screen` 不依赖任何上层。下层不引上层。

## 2. 依赖

| 依赖 | 用途 | 备注 |
|---|---|---|
| `golang.org/x/term` | raw mode、窗口尺寸 | 唯一与终端模式相关的依赖 |
| `github.com/gorilla/websocket` | WS 长连接 | 成熟、无额外依赖 |
| `golang.org/x/sys` | 控制台 VT 开关；Windows 桌面窗口的 user32/gdi32/kernel32 调用 | 不是 TUI 框架；只调系统 API |
| 标准库 | HTTP、JSON、时间、数学 | REST 与序列化不引第三方 |

**没有 TUI 框架**（不引 tview / bubbletea / termui）：布局、边框、量表、按键
全部手写，二进制小、行为可预测、SSH 里表现一致。

## 3. 目录

```
client/
├── Makefile          make build / test / check（Windows 同时产出 .exe）
├── go.mod / go.sum
├── main.go          参数、备屏、主循环、离屏快照
├── config.go        playcli.json 读写
├── proto.go         帧、视图结构、动作常量、关闭码
├── net.go           REST 登录注册登出 + WS 会话（握手/心跳/退避）
├── state.go         状态归并、日志环形缓冲、舆图累积
├── keys.go          按键切分、视图状态、toast
├── app.go           状态机：事件处理、按键、tick
├── screen.go        字符屏、ANSI 输出、颜色档位
├── widgets.go       盒子、量表、列表行、键值行、标题栏
├── render.go        主界面（标题栏 + 状态/去处/见闻 + 目标/按键）
├── pages.go         数据页（角色/背包/技能/宗门/舆图/设置）
├── text.go          显示宽度、截断、补齐、千分位、时间格式
├── theme.go         调色板、日志色、方位箭头
├── gui_windows.go   Windows 桌面窗口：Win32/GDI 双缓冲、DPI、鼠标拖舆图
├── gui_other.go     非 Windows：runGUI 返回不支持
├── export_dll.go    DLL 构建入口（//export RunGame，仅 -tags dll）
├── vt_windows.go    Windows 控制台开 VT
├── vt_other.go      Unix 空实现
├── cmd/launcher/    Windows 原生登录启动器（独立 exe）
├── assets/          启动器横幅与图标（随发布目录）
├── data/manifest.json  名称 / 版本 / 游戏路径 / 默认服务器
├── packaging/       发布目录组装脚本与 README
├── docs/            设计文档（本目录）
├── tools/doc-check.py  文档自查
└── *_test.go        144 项单测
```

## 4. 并发模型

- **主 goroutine**：`select` 三路——按键批次、网络事件、250ms 定时器。
  每轮：`drain` 事件 → `tick` 时间相关状态 → 绘制一帧。
- **网络 goroutine**：`runNet` 串起登录、建连、会话、退避重连；写操作全在这一个
  goroutine 里，避免 `gorilla/websocket` 被并发写。
- **读 goroutine**：`wsSession` 里单独起一个只做 `ReadMessage` 的协程，把结果投进
  channel；会话结束时关闭 `done`，读协程立刻退出，不留僵尸。
- **两条通道**：命令 `chan netCmd`（缓冲 64）、事件 `chan netEvent`（缓冲 2048）；
- **登录启动器**：独立进程 `launcher.exe` 读取 `data/manifest.json` 与 `assets/`，用原生 Win32
  控件登录/注册；账号库 `accounts.json` 用 Windows DPAPI（本机用户绑定）加密密码，最多 12 个账号、
  最近使用置顶；成功后按清单里的 `game` 路径以 `CREATE_NO_WINDOW` 调用系统 `rundll32` 启动宿主 DLL
  `game/gamehost.dll`（宿主启动即 `FreeConsole`），宿主用 `LoadLibrary` 加载同目录
  `game/playcli.dll` 并调用导出的 `RunGame`；token 走 `XIUXIAN_TOKEN` 环境变量，密码不进入命令行。
  启动器同时把自身路径写入 `XIUXIAN_LAUNCHER`；游戏里选“返回登录窗口”时关闭游戏并重启该启动器，
  没有该变量时回退到游戏内登录页。发布包唯一的 exe 是登录启动器，游戏宿主与本体都是 DLL。
  启动器不设 `HideWindow`，它会让宿主第一次 `ShowWindow` 被强制成 `SW_HIDE`；拉起后轮询
  子进程的可见 `JiuzhouWenDao` 窗口，窗口可见才退出，进程提前退出或超时则保留启动器报错。
- **桌面窗口**：GUI 线程 `runtime.LockOSThread` 跑 Win32 消息循环，250ms `WM_TIMER`
  里执行同一个 `drain → tick → render`；键盘/鼠标消息转换成同一套 `key` 与 `uiState`，
  不引入第二套状态机。所有 GDI 句柄都在该线程创建与释放。默认 `WS_POPUP` 无边框，
  设置页切换时只改外框位、保留 `WS_VISIBLE`，并保持客户区尺寸不变。客户区内部再留
  14px 纯黑外框（DPI 缩放），绘制与鼠标坐标都先扣掉该偏移；黑框按下时记录抓取点，
  移动时按客户区坐标增量 `SetWindowPos`，框内点击仍走原命中测试。
  事件投递用 `select/default`，满了丢弃——界面每 250ms 重绘，丢一条不会卡住网络。
- 断线重连在**网络 goroutine** 里退避，主循环只把状态画成"重连 5s"。

## 5. 字符屏的写法约束

- **整屏重绘**，但**按行跳过未变化的内容**：每行算 FNV 签名与上一帧比较，
  相同就不重写；空闲时一帧输出 0 字节。
- **一次 flush**：整屏拼进 `bufio.Writer`，一次写出，不逐行 flush。
- **宽字符占两格**：写入宽度 2 的字时把后一格标记为续格（`ch == 0`），
  渲染时跳过，否则终端会把续格当空格多打一列。
- **行尾清残影**：每行先算最后一个非空白格，只写到那里，再补 `ESC[K`。
- **定宽对齐用 `padWidth`**（按显示列补空格），不用 `fmt` 的 `%-8s`（按字符数补）。
- 不阻塞：主循环不做网络等待，绘制也不做 I/O。

## 6. 终端兼容与降级

- 进入时 `term.MakeRaw` + 备用屏 + 隐藏光标；退出（含异常路径）恢复，`defer` 兜底。
- Windows 必须开 `ENABLE_VIRTUAL_TERMINAL_PROCESSING`，否则 ANSI 原样显示（`vt_windows.go`）。
- 真彩优先；`--color 256` 用 xterm-256 近似；`--color off` 或 `NO_COLOR` 输出纯文本。
- 终端小于 80×24 不报错：窄屏走单行状态摘要，见闻压到 3 行。
- 每帧重取窗口尺寸，拖动窗口即时重排。

## 7. 字体与宽度

- **不打包字体**：用终端自己的等宽字体，要求等宽 CJK（更纱黑体、Consolas + 中文回退均可）。
- `textWidth` 判定 East Asian Wide/Fullwidth 为 2 列，组合符为 0 列；
  制表符 `─│┌┐└┘` 与方块 `█░` 按 1 列处理。
- 不做全角/半角转换；IME 由终端负责，客户端只收已上屏字符。

## 8. 颜色令牌（唯一色源）

`theme.go` 是唯一色源，别处不写 RGB 字面量。

| 常量 | 值 | 用途 |
|---|---|---|
| `colBG` | `#10141A` | 整屏底 |
| `colPanel` | `#1E252E` | 盒子 / 标题栏底 |
| `colText` | `#E2E2D6` | 正文 |
| `colDim` / `colFaint` | `#78828A` / `#464E56` | 弱化两级（标签 / 边框） |
| `colAccent` | `#7FD1C1` | 交互、真气、选中底 |
| `colGold` | `#D9B36A` | 收益、修为、目的地 |
| `colBad` | `#C4553D` | 损失、气血、错误 |
| `colGood` | `#6FBF73` | 体力、成功 |
| `colWarn` | `#C9A227` | 重连、提示 |
| `colPVP` | `#A98BC8` | PVP 日志 |

日志类型到颜色的映射只在 `tone()` 一处。

## 9. 配置与本地文件

| 文件 | 位置 | 内容 |
|---|---|---|
| `playcli.json` | `os.UserConfigDir()/xiuxian-client/` | `server`、`lastName`、`color`、`windowMode`（缺省无边框） |
| `accounts.json` | 同上 | 启动器账号库：`remember`、`accounts[]`（`name`/`server`/DPAPI `passEnc`），最多 12 个 |

游戏配置不存密码；启动器仅在勾选“记住密码”时保存 DPAPI 密文。
命令行覆盖：`--server`、`--login`、`--create`、`--page`、`--move`、
`--snapshot`、`--color`、`--w`、`--h`。

## 10. 关键取舍

| 取舍 | 选择 | 理由 |
|---|---|---|
| UI 框架 | 手写 ANSI | 消除框架版本与行为差异；二进制只依赖 WS 与 term |
| 布局 | 标题栏 + 三盒 + 两行 | 终端里固定结构最稳；不做浮动面板 |
| 场景 | 不做字符画 | 一屏的信息密度优先，地形与天气用文字表达 |
| 刷新 | 整屏重绘 + 按行签名跳过 | 逻辑简单，空闲零输出 |
| 日志 | 环形缓冲 5000 行 + 只渲染可见行 | 见 `C-08` |
| 图表 | 不做 | 终端画不出有意义的曲线；用表格代替 |
| 错误处理 | 网络/JSON 错误转成状态文案 | panic 会打乱终端（raw mode + 备用屏），先恢复再退出 |

## 11. 与服务端的一致性

- 协议版本：`protoVersion = 1`，`hello` 回带；收到 4004 停止重连并提示升级。
- 动作名/错误码：用 `proto.go` 的常量，不写字符串字面量。
- 帧字段：允许服务端新增（`encoding/json` 忽略未知字段）；**字段改名视为破坏性变更**，要升 `proto`。
- 时间：倒计时用服务端时间（`offset = ready.serverNow - nowMs()`），不读本地时区。
