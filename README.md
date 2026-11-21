# 客户端工程

这是《九州问道》的客户端，**独立仓库**：`git@github.com:NJHHBJSJKAsjdsasa/xiuxian-mud-client.git`。
服务端在另一个仓库（`xiuxian-mud`），两边只通过协议对齐。

## 现状

- **只有设计文档**（`client/docs/`），代码尚未开始（`src/` 在 M1 落盘）。
- 协议以服务端 `docs/05-PROTOCOL.md` 为准，接入流程看 `docs/12-API.md`。

## 先读哪几份

| 顺序 | 文档 | 作用 |
|---|---|---|
| 1 | [`docs/00-INDEX.md`](docs/00-INDEX.md) | 文档目录与边界 |
| 2 | [`docs/01-SCOPE.md`](docs/01-SCOPE.md) | 做什么、不做什么 |
| 3 | [`docs/02-ARCH.md`](docs/02-ARCH.md) | 技术选型与目录结构 |
| 4 | [`docs/03-CONNECTION.md`](docs/03-CONNECTION.md) | 登录、握手、重连 |
| 5 | [`docs/09-ROADMAP.md`](docs/09-ROADMAP.md) | 里程碑与完成定义 |

## 计划中的技术栈

- TypeScript + Vite
- 界面：原生 DOM + CSS（面板多但结构简单）
- 节点图：Canvas 2D
- 测试：帧回放（JSONL）+ 对服务端脚本输出的对照

## 本地联调

服务端是另一个仓库，先把它跑起来（默认 `8080`）：

```bash
# 服务端仓库
go run ./cmd/server
# 本仓库
npm run dev
```

Vite 代理把 `/api` 与 `/ws` 指到 `http://127.0.0.1:8080`（可用 `XIANXIA_API` 覆盖）；
生产环境由同一个反向代理同源转发，客户端不直连跨域地址。

## 自查

```bash
python tools/doc-check.py     # 文档矩阵与链接
```

## 约定

- 不复制服务端数值（伤害、体力、次数、回合上限）；一律从帧里读或引用服务端文档。
- 不本地推导战斗结果；只播服务端给的帧。
- 协议版本 `PROTO` 与 `hello.proto` 必须一致；收到关闭码 `4004` 提示升级，不重连。
