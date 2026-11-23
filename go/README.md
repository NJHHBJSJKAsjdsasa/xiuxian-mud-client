# Go 版终端客户端（playcli）

同仓 `src/` 是 Rust 参考实现；这里是 Go 版：`encoding/json` + `golang.org/x/term`
+ `gorilla/websocket`，渲染全是手写 ANSI——**不引任何 TUI 框架**，单二进制，
`go build` 完就能跑。

## 为什么有两套

- **Rust 版**（`src/`）：日常游玩与界面演进的主线，功能最全。
- **Go 版**（`go/`）：改动小、编译快，联调时用来确认"服务端到底下发了什么"，
  也方便在只装了 Go 工具链的机器上临时看一眼。

两套走完全相同的 `/api/*` + `/ws`，没有专用接口，也没有为 Go 版新增字段。
界面口径以 `docs/05-UI.md` 为准：改布局先改文档。

## 构建与运行

```bash
cd go
go build -o bin/playcli .
./bin/playcli --server ws://127.0.0.1:18080
```

| 参数 | 说明 |
|---|---|
| `--server` | 服务端地址；缺省读 `xiuxian-client/playcli.json`，首次为 `ws://127.0.0.1:8080` |
| `--login 道号:密码` | 跳过登录页 |
| `--create 道号` | 登录后自动建号（服务端说"还没角色"时才发） |
| `--page game\|character\|bag\|skills\|social\|market\|sect\|map\|settings` | 起始页 |
| `--move N` | 进主界面后自动走第 N 个出口（从 0 起） |
| `--snapshot out.txt` | 离屏渲染一屏后退出，排查与工单先贴这一屏 |
| `--color true\|256\|off` | 颜色档位；`NO_COLOR` 环境变量优先 |

快照示例：

```bash
./bin/playcli --server ws://127.0.0.1:18080 --login 沈砚:secret123 \
  --create 沈砚 --snapshot /tmp/one-frame.txt --color off -w 110 -h 36
```

## 按键

与 Rust 版一致：`↑↓`（或 `kj`）选路、`Enter` 前往、`Space` 历练、`c` 打坐、
`b` 突破、`l` 看看四周、`/` 说话（`Tab` 换频道、`Esc` 取消）、
`PgUp/PgDn/Home/End` 翻见闻、`F1–F8` 数据页、`Esc` 返回、
`o` 退出登录、`q` 退出、`Ctrl+C` 强退。

登录页：`Tab`/`↑↓` 切换输入框、`Ctrl+R` 切登录/注册、`Enter` 提交。

## 代码结构

| 文件 | 职责 |
|---|---|
| `main.go` | 参数、备屏与主循环、离屏快照 |
| `net.go` | REST 登录注册登出 + WS 会话（握手 / 心跳 / 退避 / 关闭码） |
| `proto.go` | 帧与视图结构、动作常量、关闭码分类 |
| `state.go` | 状态归并、日志环形缓冲、舆图累积 |
| `keys.go` | 转义序列切分、视图状态、toast |
| `screen.go` | 字符屏与 ANSI 输出（宽字符续格、脏行跳过、真彩/256/无色） |
| `scene.go` | ASCII 场景生成 |
| `render.go` / `pages.go` | 主界面七段布局 / F1–F8 数据页 |
| `text.go` / `theme.go` | 显示宽度与截断 / 配色 |

## 测试

```bash
cd go && go test ./...
```

41 项：文本宽度与截断、字符屏续格与脏行、场景网格与确定性、按键解析
（含认不出的 CSI 整段吞掉）、状态合并、8 个页面"每行宽度恰好等于终端宽"的不变式。
