# 路由一级前缀四分区治理 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将全部 HTTP 路由按一级前缀划分为 Web(`/api/web/v1`,JWT)、CLI(`/api/cli/v1`,API Key)、Proxy(不变)、运维根路径(无鉴权)四区，并使鉴权边界成为代码结构事实。

**Architecture:** `RegisterAPIRouter` 收敛为四区编排入口；Web 区拆为 jwtGroup（挂 JWT+统一限流）与 publicGroup（无鉴权）两个平行子组；CLI 区合并 client models 与 trace 上报；fgprof 从全局中间件链移入 ops 注册并加非生产限定。规格见 `docs/superpowers/specs/2026-08-27-router-zone-prefix-design.md`。

**Tech Stack:** Go 1.x + gofiber/fiber v3 + danielgtaylor/huma v2；前端 Next.js（仅改字符串路径）。

## Global Constraints

- 工作目录：`.worktrees/router-zone-prefixes`（分支 `refactor/router-zone-prefixes-2026-08-27` 已就绪）
- Proxy 前缀 `/api/openai/v1`、`/api/anthropic/v1` 保持不变
- 不留旧路径重定向、不做双前缀兼容
- fgprof 仅非生产注册，模式对齐 `RegisterDocsRouter`
- huma 约束（历史雷区，master 29727321）：`huma.Group` 的 `UseMiddleware` 会被其下所有子组路由继承；公开路由绝不能挂在带 JWT 的组的后代上，必须挂在平行的独立组上
- 中文注释、代码风格遵循现有文件

---

### Task 1: 路径契约常量更新

**Files:**
- Modify: `internal/common/constant/string.go:172`

**Interfaces:**
- Produces: `constant.ClientModelsAPIPrefix = "/api/cli/v1"`（`ClientModelsListPath` 自动派生为 `/api/cli/v1/model/list`）

- [x] **Step 1: 修改常量**

```go
	// ClientModelsAPIPrefix / ClientModelsRoutePath 客户端模型分发接口的路径契约：
	// 服务端以 ClientModelsRoutePath 为组内路径注册，客户端以二者拼接后的
	// ClientModelsListPath 请求。任一改名只需改这里，避免两侧脱节（#165 曾因
	// 只改 SDK 路径、未改 group 前缀导致 /api/v1/client/model/list 而 404）。
	// 2026-08-27 路由分区治理：客户端路由归入 CLI 分区，前缀改为 /api/cli/v1。
	ClientModelsAPIPrefix = "/api/cli/v1"
	ClientModelsRoutePath = "/model/list"
```

- [x] **Step 2: 编译验证**

Run: `cd .worktrees/router-zone-prefixes && go build ./internal/common/...`
Expected: 无输出（成功）。全仓库可能因旧前缀断言失败，属预期，后续任务修复。

- [x] **Step 3: Commit**

```bash
rtk git add internal/common/constant/string.go && rtk git commit -m "refactor(constant): 客户端模型分发接口前缀迁移至 CLI 分区"
```

---

### Task 2: Ops 分区（health/install.sh/docs/pprof 收敛 + fgprof 非生产限定）

**Files:**
- Create: `internal/router/ops.go`（由 health.go 扩展改名而来）
- Delete content of: `internal/router/health.go`
- Modify: `internal/router/router.go`（移除 `initHealthRouter` 与 `install.sh` 注册，Task 3 一并处理时保留引用即可）
- Modify: `internal/bootstrap/router.go:78-79`（调用方式变更）
- Modify: `internal/bootstrap/container.go:85`（移除全局 FgprofMiddleware）

**Interfaces:**
- Consumes: `handler.PingHandler`、`handler.TraceHandler`、`middleware.FgprofMiddleware()`、`config.Env == enum.EnvProduction`
- Produces: `router.RegisterOpsRouter(app *fiber.App, humaAPI huma.API, pingHandler handler.PingHandler, traceHandler handler.TraceHandler)` —— Task 3 的 `RegisterAPIRouter` 不再负责根路径路由

- [x] **Step 1: 创建 ops.go 并清空 health.go**

`internal/router/ops.go`：

```go
// Package router 运维/基础设施分区路由（根路径，无鉴权）
package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

// RegisterOpsRouter 注册运维分区路由：健康检查、就绪检查为常驻；文档与 pprof 仅非生产开放。
//
//	@param app *fiber.App
//	@param humaAPI huma.API
//	@param pingHandler handler.PingHandler
//	@param traceHandler handler.TraceHandler
//	@author centonhuang
//	@update 2026-08-27
func RegisterOpsRouter(app *fiber.App, humaAPI huma.API, pingHandler handler.PingHandler, traceHandler handler.TraceHandler) {
	initHealthRouter(humaAPI, pingHandler)
	registerInstallScript(humaAPI, traceHandler)

	if config.Env != enum.EnvProduction {
		registerDocs(app)
		// pprof(fgprof) 与 /docs 同策略：生产环境不暴露调试端点
		app.Use(middleware.FgprofMiddleware())
	}
}

func initHealthRouter(healthGroup huma.API, pingHandler handler.PingHandler) {
	huma.Register(healthGroup, huma.Operation{
		OperationID: "healthCheck",
		Method:      http.MethodGet,
		Path:        constant.RoutePathHealth,
		Summary:     "HealthCheck",
		Description: "Check the server health",
		Tags:        []string{constant.TagHealth},
	}, pingHandler.HandlePing)

	huma.Register(healthGroup, huma.Operation{
		OperationID: "readinessCheck",
		Method:      http.MethodGet,
		Path:        constant.RoutePathReady,
		Summary:     "ReadinessCheck",
		Description: "Check if the server is ready to accept traffic",
		Tags:        []string{constant.TagHealth},
	}, pingHandler.HandleReady)

	huma.Register(healthGroup, huma.Operation{
		OperationID: "sseHealthCheck",
		Method:      http.MethodGet,
		Path:        constant.RoutePathSSEHealth,
		Summary:     "SSEHealthCheck",
		Description: "Check the server health",
		Tags:        []string{constant.TagHealth},
	}, pingHandler.HandleSSEPing)
}

func registerInstallScript(humaAPI huma.API, traceHandler handler.TraceHandler) {
	huma.Register(humaAPI, huma.Operation{
		OperationID: "installTraceScript", Method: http.MethodGet, Path: "/install.sh",
		Summary:     "InstallTraceScript",
		Description: "Return the self-contained Aris client install script",
		Tags:        []string{constant.TagTrace},
	}, traceHandler.HandleInstallScript)
}

func registerDocs(app *fiber.App) {
	app.Get("/docs", func(c fiber.Ctx) error {
		html := `<!doctype html>
<html>
  <head>
    <title>Aris Mem API Reference</title>
    <meta charset="utf-8" />
    <meta
      name="viewport"
      content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <script
      id="api-reference"
      data-url="/openapi.json"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`
		return c.Type("html").SendString(html)
	})
}
```

然后整文件删除 `internal/router/health.go`（内容已并入 ops.go），并从 `router.go` 中删除原 `RegisterDocsRouter` 整个函数及 `RegisterAPIRouter` 内的 `initHealthRouter(...)` 调用与 `install.sh` 注册块（`RegisterAPIRouter` 其余部分在 Task 3 重写，此处先保证编译）。

- [x] **Step 2: 修改 bootstrap**

`internal/bootstrap/container.go`：删除中间件链中的 `middleware.FgprofMiddleware(),` 一行及其上方必要注释（其余保持）。

`internal/bootstrap/router.go` 的 `registerRoutes` 中：

```go
	// 原:
	// if config.Env != appenum.EnvProduction {
	// 	router.RegisterDocsRouter(params.App)
	// }
	router.RegisterOpsRouter(params.App, params.HumaAPI, params.PingHandler, params.TraceHandler)
```

同时删除 `appenum` import 若不再被使用（编译器会提示）。

- [x] **Step 3: 编译验证**

Run: `go build ./...`
Expected: 若 router.go 尚有 initHealthRouter 引用残留则修复后再验证通过。

- [x] **Step 4: Commit**

```bash
rtk git add -A internal/router internal/bootstrap && rtk git commit -m "refactor(router): 收敛运维分区路由并将 fgprof 收归非生产限定"
```

---

### Task 3: 分区编排入口重构（router.go）

**Files:**
- Modify: `internal/router/router.go`

**Interfaces:**
- Consumes: `router.RegisterOpsRouter`（Task 2）、各模块 `initXxxRouter`（签名暂不变）
- Produces: `router.RegisterAPIRouter(humaAPI huma.API, deps APIRouterDependencies)` 仅做分区编排；内部调用 Task 4 的 `RegisterWebAPIRoutes(webRoot huma.API, deps APIRouterDependencies)` 与 Task 5 的 `RegisterCLIAPIRoutes(cliGroup huma.API, deps APIRouterDependencies)`

- [x] **Step 1: 重写 RegisterAPIRouter 函数体**

保留 `APIRouterDependencies` 结构体不动。函数体替换为：

```go
// RegisterAPIRouter 注册 API 路由（分区编排入口）。
//
// 四个分区，各自唯一鉴权方式：
//   - Web:   /api/web/v1/*            session JWT（公开入口在区内 public 子组，仅限流）
//   - CLI:   /api/cli/v1/*            API Key
//   - Proxy: /api/openai/v1、/api/anthropic/v1   API Key（前缀对外契约不变）
//   - Ops:   根路径健康检查/文档/pprof 等          无鉴权（见 RegisterOpsRouter）
//
// huma 约束（master 29727321）：父组的 UseMiddleware 会并入所有子组路由，
// 因此公开路由必须挂在与 jwtGroup 平行的独立子组上。
//
//	@author centonhuang
//	@update 2026-08-27
func RegisterAPIRouter(humaAPI huma.API, deps APIRouterDependencies) {
	// ── Web 分区 ──
	webRoot := huma.NewGroup(humaAPI, "/api/web/v1")
	RegisterWebAPIRoutes(webRoot, deps)

	// ── CLI 分区 ──
	cliGroup := huma.NewGroup(humaAPI, "/api/cli/v1")
	RegisterCLIAPIRoutes(cliGroup, deps)

	// ── Proxy 分区（前缀不变） ──
	openaiGroup := huma.NewGroup(humaAPI, "/api/openai/v1")
	initOpenAIRouter(openaiGroup, deps.OpenAIHandler, deps.DB, deps.Cache)

	anthropicGroup := huma.NewGroup(humaAPI, "/api/anthropic/v1")
	initAnthropicRouter(anthropicGroup, deps.AnthropicHandler, deps.DB, deps.Cache)
}
```

- [x] **Step 2: 编译验证（会因 web/cli 未建而失败，属预期）**

Run: `go build ./internal/router/...` Expected: undefined: RegisterWebAPIRoutes / RegisterCLIAPIRoutes → 由 Task 4、Task 5 消除。

- [x] **Step 3: Commit（连同 Task 4、5 合并提交亦可，此处单独提交需先完成后续任务）**

建议 Task 3~5 完成后统一提交一次：
```bash
rtk git add internal/router && rtk git commit -m "refactor(router): 路由按一级前缀划分四分区编排"
```

---

### Task 4: Web 分区路由重组（web.go + 各模块去鉴权行）

**Files:**
- Create: `internal/router/web.go`
- Modify: `internal/router/{oauth2,token,user,demo,apikey,session,endpoint,model,upstream,audit,cron,trigger,metrics,dataset}.go`
- Modify: `internal/router/trace.go`（仅查询组迁出至 web；report/check 归 Task 5）

**Interfaces:**
- Consumes: 各 `initXxxRouter` 现有签名不变；`deps` 字段
- Produces: `RegisterWebAPIRoutes(webRoot huma.API, deps APIRouterDependencies)`

- [x] **Step 1: 创建 web.go**

```go
// Package router Web 分区路由（/api/web/v1，session JWT 鉴权）
package router

import (
	"github.com/danielgtaylor/huma/v2"

	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// RegisterWebAPIRoutes 注册 Web 分区路由。
//
// 结构约束（huma Group.Middlewares 继承语义，见 master 29727321 的教训）：
//   - jwtGroup：统一挂 JwtMiddleware + demoAccess 统一限流；各业务模块子组挂在其下
//   - publicGroup：同前缀下的平行组，不含 JWT；公开入口（oauth2/token 刷新/
//     demo 登录/分享公开读）必须且只能注册到这里
//
// 注意：模块级自有限流（如 userManage/modelManage）仍留在各自 init 函数内。
//
//	@author centonhuang
//	@update 2026-08-27
func RegisterWebAPIRoutes(
	webRoot huma.API,
	deps APIRouterDependencies,
	db *gorm.DB,
	cache *redis.Client,
	accessSigner jwt.TokenSigner,
	demoAccessor demoport.DemoModuleAccessor,
	demoSubmitter demoport.DemoSubmitter,
) { ... }
```

——**修正**：依赖已整体在 `deps` 中，签名收敛为两参以避免冗余（deps 已含 db/cache/accessSigner/demo 字段）；实际实现采用：

```go
func RegisterWebAPIRoutes(webRoot huma.API, deps APIRouterDependencies) {
	jwtGroup := huma.NewGroup(webRoot)
	jwtGroup.UseMiddleware(
		middleware.JwtMiddleware(deps.DB, deps.Cache, deps.AccessSigner),
		middleware.TokenBucketRateLimiterMiddleware(deps.Cache, "demoAccess", "", constant.PeriodDemoAccess, constant.LimitDemoAccess, middleware.WithPermissionFilter(enum.PermissionDemo)),
	)
	publicGroup := huma.NewGroup(webRoot)

	// 公开入口
	oauth2Group := huma.NewGroup(publicGroup, "/oauth2")
	initOauth2Router(oauth2Group, deps.Oauth2Handler, deps.Cache)
	initTokenRouter(publicGroup, deps.TokenHandler, deps.Cache)
	initDemoRouter(publicGroup, jwtGroup, deps.DemoHandler, deps.Cache, accessSignerOf(deps), accessor(deps), submitter(deps))
	initSessionPublicRouter(publicGroup, deps.SessionHandler, deps.Cache)

	// JWT 业务子组
	initUserRouter(huma.NewGroup(jwtGroup, "/user"), deps.UserHandler, deps.DB, deps.Cache, accessSignerOf(deps))
	initAPIKeyRouter(huma.NewGroup(jwtGroup, "/apikey"), deps.APIKeyHandler, deps.DB, deps.Cache, accessSignerOf(deps))
	initSessionJWTRouter(huma.NewGroup(jwtGroup, "/session"), deps.SessionHandler, deps.DB, deps.Cache, accessSignerOf(deps), deps.DemoModuleAccessor, deps.DemoAuditSubmitter)
	initEndpointRouter(huma.NewGroup(jwtGroup, "/endpoint"), deps.EndpointHandler, deps.DB, deps.Cache, accessSignerOf(deps))
	initModelRouter(huma.NewGroup(jwtGroup, "/model"), deps.ModelHandler, deps.DB, deps.Cache, accessSignerOf(deps))
	initUpstreamRouter(huma.NewGroup(jwtGroup, "/upstream"), deps.UpstreamHandler, deps.DB, deps.Cache, accessSignerOf(deps), deps.DemoModuleAccessor, deps.DemoAuditSubmitter)
	initAuditRouter(huma.NewGroup(jwtGroup, "/audit"), deps.AuditHandler, deps.CronHandler, deps.DB, deps.Cache, accessSignerOf(deps), deps.DemoModuleAccessor, deps.DemoAuditSubmitter)
	initCronRouter(huma.NewGroup(jwtGroup, "/cron"), deps.CronHandler, deps.DB, deps.Cache, accessSignerOf(deps), deps.DemoModuleAccessor, deps.DemoAuditSubmitter)
	initTriggerRouter(huma.NewGroup(jwtGroup, "/trigger"), deps.TriggerHandler, deps.DB, deps.Cache, accessSignerOf(deps), deps.DemoModuleAccessor, deps.DemoAuditSubmitter)
	initMetricsRouter(huma.NewGroup(jwtGroup, "/metrics"), deps.MetricsHandler, deps.DB, deps.Cache, accessSignerOf(deps), deps.DemoModuleAccessor, deps.DemoAuditSubmitter)
	initDatasetRouter(huma.NewGroup(jwtGroup, "/dataset"), deps.DatasetHandler, deps.DB, deps.Cache, accessSignerOf(deps))
	initTraceQueryRouter(huma.NewGroup(jwtGroup, "/trace"), deps.TraceHandler, deps.DB, deps.Cache, accessSignerOf(deps))
}
```

实现注记：
1. `accessSignerOf`/`accessor`/`submitter` 是伪码占位——直接用 `deps.AccessSigner`、`deps.DemoModuleAccessor`、`deps.DemoAuditSubmitter`，不引入辅助函数（ponytail）。
2. `initDemoRouter` 需改造为双根签名 `initDemoRouter(publicRoot, jwtRoot huma.API, ...)`：login/status 挂 `publicRoot+"/demo"`，config/sessions 挂 `jwtRoot+"/demo"`（见 Step 3）。
3. `deps.AccessSigner` 类型是 `jwt.TokenSigner`（bootstrap 注入 identityservice 实现时已有 name tag 绑定），与现 init 函数参数类型一致。
4. `initTokenRouter` 第一参从 tokenGroup 改传 publicGroup：其内部只注册 `/refresh` 一条路由，无需子组包装。

- [x] **Step 2: 各模块 init 函数去除重复中间件行**

规则 A（删两行）：凡包含下面两行的函数，将两行删除（JWT 与 demoAccess 限流已上收至 jwtGroup）：
```go
xxx.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))
xxx.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(cache, "demoAccess", "", constant.PeriodDemoAccess, constant.LimitDemoAccess, middleware.WithPermissionFilter(enum.PermissionDemo)))
```
适用：`session.go(initSessionJWTRouter)`、`endpoint.go`、`model.go`、`upstream.go`、`audit.go`、`cron.go`、`metrics.go`、`dataset.go`、`trigger.go`。

规则 B（删一行留自有限流）：仅含 JWT 行、另有专属限流的函数，只删 JWT 行，保留专属限流：
- `user.go/initUserRouter`：删 `JwtMiddleware` 行，保留 `userManage` 限流
- `apikey.go/initAPIKeyRouter`：删 `JwtMiddleware` 行，保留 `apikeyManage` 限流
- `demo.go`：见 Step 3
- 函数参数（db/cache/signer）一律保留不删，最小化 diff（Go 允许未用形参）

- [x] **Step 3: demo.go 双根改造**

```go
// initDemoRouter 初始化 Demo 分组视图路由：
// login/status 为公开入口挂 publicRoot；config/sessions 受 JWT 保护挂 jwtRoot。
func initDemoRouter(publicRoot huma.API, jwtRoot huma.API, demoHandler handler.DemoHandler, cache *redis.Client, demoAccessor demoport.DemoModuleAccessor, auditSubmitter demoport.DemoSubmitter) {
	demoPublicGroup := huma.NewGroup(publicRoot, "/demo")
	huma.Register(demoPublicGroup, /* login: 原 demo.go:22 路由定义原样搬入 */)
	huma.Register(demoPublicGroup, /* status: 原 demo.go:35 路由定义原样搬入 */)

	demoConfigGroup := huma.NewGroup(jwtRoot, "/demo/config")
	huma.Register(demoConfigGroup, /* 原 demo.go:52 GetDemoConfig 原样搬入 */)
	huma.Register(demoConfigGroup, /* 原 demo.go:64 UpdateDemoConfig 原样搬入 */)

	demoSessionsGroup := huma.NewGroup(jwtRoot, "/demo/sessions")
	huma.Register(demoSessionsGroup, /* 原 demo.go:82 list 原样搬入 */)
	huma.Register(demoSessionsGroup, /* 原 demo.go:93/104 两条约简同上 */)
}
```

注意原 config/sessions 组内的 `JwtMiddleware` 与 demoAccess 限流行删除（规则 A），login/status 的专属限流（demoLogin/demoStatus）保留在各自路由 Middlewares 里。

- [x] **Step 4: trace.go 拆分查询侧**

`trace.go` 改为仅含查询路由，签名更名并去掉 report 部分：

```go
// initTraceQueryRouter 初始化 Trace 查询分组视图路由（Web 分区，JWT 鉴权）。
// 上报与 key 校验属 CLI 分区，见 initTraceReportRouter。
func initTraceQueryRouter(traceGroup huma.API, traceHandler handler.TraceHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
	queryGroup := huma.NewGroup(traceGroup, "")
	queryGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner)) // ← 此行最终也删（规则 A，jwtGroup 已统一挂载）
	// 原 trace.go:27/35/43/51 四条 JWT 路由定义原样保留
}
```
（原 `initTraceRouter` 中 reportGroup 三条路由与 `/client/check` 移入 Task 5；函数名相应更名后，web.go 引用同步为 `initTraceQueryRouter`。）

- [x] **Step 5: 编译 + 离线回归测试**

Run: `go build ./... && go test ./test/e2e/client_models/... ./internal/...`
Expected: BUILD OK；离线用例中路径断言失败者在 Task 7 修复，本步允许少量 FAIL，但不得有编译错误。

- [x] **Step 6: Commit（与 Task 3/5 合并为一个提交）**

```bash
rtk git add internal/router && rtk git commit -m "refactor(router): Web 分区前置 /api/web/v1 并重组 JWT/公开双组结构"
```

---

### Task 5: CLI 分区路由合并（cli.go）

**Files:**
- Create: `internal/router/cli.go`
- Delete: `internal/router/client.go`（内容并入）
- Modify: `internal/router/trace.go`（report/check 移入）

**Interfaces:**
- Consumes: `handler.ClientHandler.HandleListModels`、`handler.TraceHandler.{HandleReportTraceEvent,HandleCheckTraceClientAPIKey}`、`middleware.APIKeyMiddleware(db)`
- Produces: `RegisterCLIAPIRoutes(cliGroup huma.API, deps APIRouterDependencies)`

- [x] **Step 1: 创建 cli.go**

```go
// Package router CLI 分区路由（/api/cli/v1，API Key 鉴权）
package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

// RegisterCLIAPIRoutes 注册 CLI 分区路由（aris 客户端）。
//
// 路径契约：model/list 以 constant.ClientModelsAPIPrefix + ClientModelsRoutePath
// 对外承诺（客户端 SDK 同源派生），前缀已在常量层迁移至 /api/cli/v1。
//
//	@author centonhuang
//	@update 2026-08-27
func RegisterCLIAPIRoutes(cliGroup huma.API, deps APIRouterDependencies) {
	cliGroup.UseMiddleware(middleware.APIKeyMiddleware(deps.DB))

	huma.Register(cliGroup, huma.Operation{
		OperationID: "listClientModels",
		Method:      http.MethodGet,
		Path:        constant.ClientModelsRoutePath,
		Summary:     "ListClientModels",
		Description: "List enabled models with capabilities for aris client configuration",
		Tags:        []string{constant.TagClient},
		Security:    []map[string][]string{{constant.SecuritySchemeAPIKey: {}}},
	}, deps.ClientHandler.HandleListModels)

	huma.Register(cliGroup, huma.Operation{
		OperationID: "reportTraceEvent", Method: http.MethodPost, Path: "/trace/event",
		Summary: "ReportTraceEvent", Description: "Report a codex hook event (API key auth)",
		Tags:     []string{constant.TagTrace},
		Security: []map[string][]string{{constant.SecuritySchemeAPIKey: {}}},
	}, deps.TraceHandler.HandleReportTraceEvent)

	huma.Register(cliGroup, huma.Operation{
		OperationID: "checkTraceClientAPIKey", Method: http.MethodGet, Path: "/trace/client/check",
		Summary: "CheckTraceClientAPIKey", Description: "Validate the trace client API key",
		Tags:     []string{constant.TagTrace},
		Security: []map[string][]string{{constant.SecuritySchemeAPIKey: {}}},
	}, deps.TraceHandler.HandleCheckTraceClientAPIKey)
}
```

- [x] **Step 2: 清理**

- 整文件删除 `internal/router/client.go`
- `trace.go` 中 `initTraceRouter` 已更名为 `initTraceQueryRouter`（Task 4 Step 4）并删除 reportGroup 部分

- [x] **Step 3: 编译验证**

Run: `go build ./...` Expected: 成功（e2e 断言失败留给 Task 7）。

- [x] **Step 4: Commit（与 Task 3/4 同一提交）**

---

### Task 6: Web 前端路径批量迁移

**Files:**
- Modify: `web/src/lib/api-client.ts`（约 74 处）

- [x] **Step 1: 批量替换**

```bash
sed -i '' 's|/api/v1/|/api/web/v1/|g' web/src/lib/api-client.ts
```

- [x] **Step 2: 验证零残留**

Run: `grep -n '"/api/v1' web/src/lib/api-client.ts | wc -l` Expected: `0`；
Run: `rg "'/api/openai|'/api/anthropic'" web/src/lib/api-client.ts | wc -l` Expected: `0`（前端本就不直连 proxy）

- [x] **Step 3: 前端静态检查**

Run: `cd web && npx tsc --noEmit`（或项目既有 `pnpm lint`）Expected: 通过（纯字符串改动，理论上无影响面）

- [x] **Step 4: Commit**

```bash
rtk git add web/src/lib/api-client.ts && rtk git commit -m "refactor(web): API 路径迁移至 /api/web/v1 分区前缀"
```

---

### Task 7: e2e/单测路径迁移与全量验证

**Files:**
- Modify: `test/e2e/**`（23 个文件涉及旧前缀）

**Interfaces:**
- Consumes: `constant.ClientModelsListPath`（已是新值）

- [x] **Step 1: 批量替换 + CLI 路径回改**

```bash
# 1) 全局迁移到 web 分区
grep -rl '"/api/v1' test/e2e --include='*.go' | xargs sed -i '' 's|/api/v1/|/api/web/v1/|g'

# 2) CLI 专属路径回改（上报与 key 校验属 CLI 分区）
grep -rl '"/api/web/v1/trace/event"\|"/api/web/v1/trace/client/check"' test/e2e --include='*.go' \
  | xargs sed -i '' \
      -e 's|/api/web/v1/trace/event|/api/cli/v1/trace/event|g' \
      -e 's|/api/web/v1/trace/client/check|/api/cli/v1/trace/client/check|g'
```

- [x] **Step 3: 手工核对特例**

- `test/e2e/client_models/client_models_test.go:51,103` 本地注册用例：`Path: "/api/web/v1/model/list"` → 改回 `constant.ClientModelsListPath`（与服务端同源）；`:130` 在线用例拼串处同理
- `test/e2e/client_models/oauth2_route_leak_test.go:82`：请求路径改为 `/api/web/v1/oauth2/login?platform=github`
- `test/e2e/client_models/client_route_test.go` 使用 `constant.ClientModelsListPath`，应自动跟随 Task 1；如无自动跟随则手工修正
- 全部测试中的注释文字里出现的旧路径说明同步顺一遍（可读性）

- [x] **Step 4: 全量单测 + 构建**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: 全绿。若有 fail：逐条核对新旧映射表（见 spec 第 2 节）修复。

- [x] **Step 5: 运行态抽查（可选，本地起服）**

Run: 起 dev server 后依次 curl：
```bash
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/health                 # 200
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/api/web/v1/user/current # 401
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/api/cli/v1/model/list   # 401
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/debug/pprof/            # 200（非生产）
```
任何 500/404 视为分区编排错误，回查 Task 3~5。

- [x] **Step 6: ponytail-review 自审 + 最终提交**

对照 diff 检查：投机抽象（辅助函数应已被否决）、重复限流挂载（同一请求不应经过两个相同 name 的 demoAccess 令牌桶）、遗留死代码（client.go 已删、router.go 无多余 import）。然后：

```bash
rtk git add -A && rtk git commit -m "test(e2e): 用例路径随分区迁移并全量转绿"
```

---

## Self-Review 记录

- Spec 覆盖：四区（T3/T4/T5）、fgprof 非生产（T2）、前端迁移（T6）、不留兼容=无重定向代码、验证清单（T7 Step4/5）✔
- 类型一致性：`RegisterWebAPIRoutes`/`RegisterCLIAPIRoutes`/`RegisterOpsRouter` 的调用点与产出签名一致 ✔
- 占位符：web.go 中已明确"伪码占位→实际写法"的注记 1~4；demo/trace 的"原样搬入"均给出源行号锚点 ✔
