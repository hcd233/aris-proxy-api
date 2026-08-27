# 路由一级前缀分区治理设计

> 日期：2026-08-27
> 分支：`refactor/router-zone-prefixes-2026-08-27`
> 状态：已获用户批准（方案 1；pprof 仅非生产开放；不留旧路径兼容）

## 1. 目标

将后端全部 HTTP 路由按**真实 URL 一级前缀**划分为四个分区，使"某前缀下只存在某种鉴权方式"成为代码结构事实而非口头约定：

| 分区 | 前缀 | 鉴权 |
|------|------|------|
| Web 端 | `/api/web/v1/*` | session JWT（少数公开入口无鉴权，仅限流） |
| CLI（aris 客户端） | `/api/cli/v1/*` | API Key |
| Proxy | `/api/openai/v1/*`、`/api/anthropic/v1/*` | API Key（**前缀不变**） |
| 运维/基础设施 | 根路径：`/health`、`/ready`、`/ssehealth`、`/docs`、`/openapi.json`、`/install.sh` + 新增 pprof | 无鉴权 |

关键决策（用户确认）：

1. **方案 1**：不仅改 URL 前缀，还重组路由注册结构——Web 区改为"前缀级默认 JWT"结构，公开入口放独立无鉴权子组。
2. **归区**：client models 归 CLI；demo 与 share 全部归 Web；proxy 前缀保持 `/api/openai/v1`、`/api/anthropic/v1` 不变。
3. **pprof**：项目使用 fgprof，参照 `/docs` 的做法仅非生产环境注册（生产不暴露）。
4. **兼容性**：不留旧路径重定向，前后端与测试同步改，一次切换。

## 2. 现状 → 目标路径映射

### 2.1 Web 区（`/api/web/v1`）

| 模块 | 旧路径 | 新路径 | 鉴权 |
|------|--------|--------|------|
| oauth2 登录/回调 | `/api/v1/oauth2/login`、`/api/v1/oauth2/callback` | `/api/web/v1/oauth2/login`、`/callback` | 无（限流） |
| token 刷新 | `/api/v1/token/refresh` | `/api/web/v1/token/refresh` | 无（限流） |
| user | `/api/v1/user*` | `/api/web/v1/user*` | JWT |
| demo 登录/状态 | `/api/v1/demo/login`、`/demo/status` | `/api/web/v1/demo/login`、`/status` | 无（限流） |
| demo 其余 | `/api/v1/demo/config*`、`/demo/sessions*` | 同前缀迁移 | JWT |
| apikey | `/api/v1/apikey*` | `/api/web/v1/apikey*` | JWT |
| session（JWT 组） | `/api/v1/session*` | `/api/web/v1/session*` | JWT |
| session 公开分享读 | `/api/v1/session/share/metadata` 等 3 条 | `/api/web/v1/session/share/*` | 无（限流） |
| endpoint/model/upstream/audit/cron/trigger/metrics/dataset | `/api/v1/<module>*` | `/api/web/v1/<module>*` | JWT |
| trace 查询组 | `/api/v1/trace`(list/get/delete)、`/trace/event/list` | `/api/web/v1/trace*` | JWT |

### 2.2 CLI 区（`/api/cli/v1`）

| 旧路径 | 新路径 | 说明 |
|--------|--------|------|
| `/api/v1/models`（`ClientModelsRoutePath = "/model/list"`，挂在 `/api/v1` 下实际为 `/api/v1/model/list`） | 由常量派生：CLI 组挂 `middleware.APIKeyMiddleware`，组内路径改为与客户端 SDK 契约同步更新 | 客户端模型列表 |
| `/api/v1/trace/event`（POST 上报） | `/api/cli/v1/trace/event` | API Key 鉴权的 codex hook 上报 |
| `/api/v1/trace/client/check` | `/api/cli/v1/trace/client/check` | 校验 trace client API key |
| `/install.sh`（根路径） | 保持根路径不变 | 属基础设施分发入口 |

### 2.3 Proxy 区

`/api/openai/v1/*`、`/api/anthropic/v1/*`：代码不动，仅在注册函数的注释中标注分区归属。

### 2.4 Ops 区（根路径）

- `/health`、`/ready`、`/ssehealth`：已有，迁入独立 ops 注册文件
- `/docs`、`/openapi.json`：已有，非生产限定逻辑保留
- `/install.sh`：从 `RegisterAPIRouter` 移到 ops 注册
- **fgprof（`/debug/pprof/*`）**：现状为全局中间件无条件挂载（含生产）。本次收归 ops 注册并加非生产限定
- **pprof（fgprof）**：已存在于 `internal/bootstrap/container.go`（`middleware.FgprofMiddleware()` 全局挂载，默认服务 `/debug/pprof`，当前无环境判断）。本次按 `/docs` 模式改造：从全局中间件链中移出，改为非生产环境才注册（实现方式与 `RegisterDocsRouter` 的环境判断一致），落实生产不暴露

## 3. 代码组织改动

```
internal/router/
├── router.go        # RegisterAPIRouter 只做四个分区的分发编排（deps 结构拆分）
├── web.go           # RegisterWebAPIRouter：/api/web/v1 主组(JWT+统一限流) + 公开子组
├── cli.go           # RegisterCLIAPIRouter：/api/cli/v1(API Key)，合并现 client.go 与 trace report/check
├── health.go        # 并入 ops.go 或改名，op 注册逻辑集中
├── proxy 前缀两个 init 函数   # 保持现状
└── 各模块 initXxxRouter       # 迁移：删除各自的 UseMiddleware(JWT) 行，改为接收已挂好中间件的父组
```

Web 区公开子组清单（同一前缀下、无 JWT、各自带限流）：oauth2、token/refresh、demo login/status、session public share。

## 4. 连带修改面

| 位置 | 改动 |
|------|------|
| `web/src/lib/api-client.ts` | ~74 处 `/api/v1/...` → `/api/web/v1/...`（share/demo 等公开路由同样在新前缀下） |
| `internal/common/constant/string.go` | `ClientModelsAPIPrefix = "/api/v1"` → CLI 新前缀；`RoutePathHealth` 等保持 |
| `internal/common/constant/route.go` | 如有 web 路径常量需同步 |
| `test/e2e/**` | 全部涉及 `/api/v1`、client_models、trace、demo、metrics 等用例改新前缀 |
| aris 客户端仓库（若有本地副本随 SDK 常量联动则同改；否则在文档中记录 breaking change） | base URL / 路径同步 |

明确不做：旧路径 301 重定向、双前缀并存过渡期。

## 5. 验证方式

1. `go build ./...` 编译通过
2. 全量单测通过（`make test` 或等价命令）
3. e2e 用例全绿（重点：client_models、oauth2_route_leak、demo、trace、users）
4. 本地起服后 curl 抽查四区各至少一条路由：
   - `/health` 200
   - `/api/web/v1/user/current` 无 token → 401
   - `/api/cli/v1/model/list` 无 API key → 401
   - 非 prod 环境 `/debug/pprof/` 200
5. web 前端登录 → 列表页联调正常（lint 通过）

## 6. 风险

- **aris 已分发客户端**在服务端切换后无法拉取模型列表，直到客户端更新（用户已接受）
- huma Group 中间件继承语义是本次重构的历史雷区（见 master 最近一次 fix），实施时必须复用"独立 group 注册"模式避免鉴权泄漏到公共路由
