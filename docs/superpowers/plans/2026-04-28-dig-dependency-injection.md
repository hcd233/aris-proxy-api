# Dig 依赖注入实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标:** 引入 `go.uber.org/dig`，把 HTTP API 启动阶段的对象构造集中到组合根，保持现有业务行为、启动顺序和关闭顺序不变。

**架构:** 新增 `internal/bootstrap` 作为唯一使用 `dig` 的组合根，负责提供 Fiber、Huma、repository、transport、handler 和路由注册入口。`cmd/server.go` 只调用 bootstrap 获取 app 并注册路由，router 只负责 Huma 路由声明，不再创建 handler 依赖。

**Tech Stack:** Go 1.25.1、Fiber、Huma、`go.uber.org/dig`、标准库 `testing`。

---

## 文件结构

- 新增 `internal/bootstrap/container.go`：创建容器、注册 provider、暴露 `BuildServer()`。
- 新增 `internal/bootstrap/router.go`：定义路由注册参数结构，统一调用 `router.RegisterDocsRouter` 和 `router.RegisterAPIRouter`。
- 新增 `test/unit/bootstrap/bootstrap_test.go`：验证容器可构建 server 并注册路由。
- 修改 `internal/api/fiber.go`：去掉包级 `init()` 构造，改为 `NewFiberApp()` 和 `SetFiberApp()`，保留 `GetFiberApp()` 兼容现有调用点。
- 修改 `internal/api/huma.go`：去掉包级 `init()` 构造，改为 `NewHumaAPI(app *fiber.App)` 和 `SetHumaAPI()`，保留 `GetHumaAPI()` 兼容现有调用点。
- 修改 `internal/router/router.go`：让 `RegisterDocsRouter` 接收 `*fiber.App`，让 `RegisterAPIRouter` 接收 `huma.API` 和 handler 参数。
- 修改 `internal/router/*.go`：让各 `initXxxRouter` 接收对应 handler，不再创建 repository、proxy 或 handler。
- 修改 `cmd/server.go`：用 bootstrap 构建 app 和注册路由，删除直接依赖 `internal/api`、`internal/router` 的启动代码。
- 修改 `go.mod`/`go.sum`：加入 `go.uber.org/dig`。

## Task 1: 引入 dig 依赖

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: 添加依赖**

Run: `go get go.uber.org/dig@latest`

Expected: `go.mod` 出现 `go.uber.org/dig`，`go.sum` 更新。

- [ ] **Step 2: 验证依赖解析**

Run: `go test -count=1 ./test/unit/...`

Expected: 测试通过，或只暴露与后续未实现代码无关的既有失败。

## Task 2: 让 API 实例可由 bootstrap 构造

**Files:**
- Modify: `internal/api/fiber.go`
- Modify: `internal/api/huma.go`

- [ ] **Step 1: 修改 Fiber 构造**

将 `internal/api/fiber.go` 调整为保留全局 getter，同时新增构造和 setter：

```go
package api

import (
	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
)

var fiberApp *fiber.App

func NewFiberApp() *fiber.App {
	return fiber.New(fiber.Config{
		Prefork:                 false,
		ReadTimeout:             config.ReadTimeout,
		WriteTimeout:            config.WriteTimeout,
		IdleTimeout:             constant.IdleTimeout,
		JSONEncoder:             sonic.Marshal,
		JSONDecoder:             sonic.Unmarshal,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          config.TrustedProxies,
		ProxyHeader:             fiber.HeaderXForwardedFor,
	})
}

func SetFiberApp(app *fiber.App) {
	fiberApp = app
}

func GetFiberApp() *fiber.App {
	return fiberApp
}
```

- [ ] **Step 2: 修改 Huma 构造**

将 `internal/api/huma.go` 调整为接收注入的 Fiber app：

```go
package api

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/samber/lo"
)

var humaAPI huma.API

func NewHumaAPI(app *fiber.App) huma.API {
	return humafiber.New(app, huma.Config{
		OpenAPI: &huma.OpenAPI{
			OpenAPI: "3.1.0",
			Info: &huma.Info{
				Title:       "Aris API Tmpl",
				Description: "Aris API Tmpl is a RESTful API Template.",
				Version:     "1.0",
				Contact: &huma.Contact{
					Name:  "hcd233",
					Email: "lvlvko233@qq.com",
					URL:   "https://github.com/hcd233",
				},
				License: &huma.License{
					Name: "Apache 2.0",
					URL:  "https://www.apache.org/licenses/LICENSE-2.0.html",
				},
			},
			Components: &huma.Components{
				Schemas: huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer),
				SecuritySchemes: map[string]*huma.SecurityScheme{
					"jwtAuth": {
						Type:        "apiKey",
						Name:        "Authorization",
						In:          "header",
						Description: "JWT Authentication，Please pass the JWT token in the Authorization header.",
					},
					"apiKeyAuth": {
						Type:        "http",
						Scheme:      "bearer",
						Description: "API Key Authentication, Please pass the API Key as Bearer token in the Authorization header.",
					},
				},
			},
		},
		OpenAPIPath:   lo.If(config.Env != enum.EnvProduction, "/openapi").Else(""),
		DocsPath:      "",
		SchemasPath:   lo.If(config.Env != enum.EnvProduction, "/schemas").Else(""),
		Formats:       huma.DefaultFormats,
		DefaultFormat: "application/json",
	})
}

func SetHumaAPI(api huma.API) {
	humaAPI = api
}

func GetHumaAPI() huma.API {
	return humaAPI
}
```

- [ ] **Step 3: 运行编译检查**

Run: `go test -count=1 ./test/unit/...`

Expected: 通过。此时尚未改 server，旧启动路径可能需要后续任务接上 setter。

## Task 3: 改造 router 为纯路由注册

**Files:**
- Modify: `internal/router/router.go`
- Modify: `internal/router/health.go`
- Modify: `internal/router/token.go`
- Modify: `internal/router/oauth2.go`
- Modify: `internal/router/user.go`
- Modify: `internal/router/apikey.go`
- Modify: `internal/router/session.go`
- Modify: `internal/router/openai.go`
- Modify: `internal/router/anthropic.go`

- [ ] **Step 1: 定义路由依赖参数**

在 `internal/router/router.go` 中新增：

```go
type APIRouterDependencies struct {
	PingHandler      handler.PingHandler
	TokenHandler     handler.TokenHandler
	Oauth2Handler    handler.Oauth2Handler
	UserHandler      handler.UserHandler
	APIKeyHandler    handler.APIKeyHandler
	SessionHandler   handler.SessionHandler
	OpenAIHandler    handler.OpenAIHandler
	AnthropicHandler handler.AnthropicHandler
}
```

并修改签名：

```go
func RegisterDocsRouter(app *fiber.App) { ... }

func RegisterAPIRouter(humaAPI huma.API, deps APIRouterDependencies) { ... }
```

- [ ] **Step 2: 修改聚合注册逻辑**

`RegisterAPIRouter` 改为使用传入的 `humaAPI` 和 `deps`：

```go
func RegisterAPIRouter(humaAPI huma.API, deps APIRouterDependencies) {
	apiGroup := huma.NewGroup(humaAPI, "/api")
	v1Group := huma.NewGroup(apiGroup, "/v1")

	initHealthRouter(humaAPI, deps.PingHandler)

	tokenGroup := huma.NewGroup(v1Group, "/token")
	initTokenRouter(tokenGroup, deps.TokenHandler)

	oauth2Group := huma.NewGroup(v1Group, "/oauth2")
	initOauth2Router(oauth2Group, deps.Oauth2Handler)

	userGroup := huma.NewGroup(v1Group, "/user")
	initUserRouter(userGroup, deps.UserHandler)

	apikeyGroup := huma.NewGroup(v1Group, "/apikey")
	initAPIKeyRouter(apikeyGroup, deps.APIKeyHandler)

	sessionGroup := huma.NewGroup(v1Group, "/session")
	initSessionRouter(sessionGroup, deps.SessionHandler)

	openaiGroup := huma.NewGroup(apiGroup, "/openai/v1")
	initOpenAIRouter(openaiGroup, deps.OpenAIHandler)

	anthropicGroup := huma.NewGroup(apiGroup, "/anthropic/v1")
	initAnthropicRouter(anthropicGroup, deps.AnthropicHandler)
}
```

- [ ] **Step 3: 修改子路由函数签名**

各子路由函数改为接收 handler。例如：

```go
func initOpenAIRouter(openaiGroup huma.API, openaiHandler handler.OpenAIHandler) { ... }
```

删除子路由文件中用于构造依赖的 import，例如 `repository`、`transport`、`jwt`、`oauth2`、`apikeyservice`。

- [ ] **Step 4: 运行编译检查**

Run: `go test -count=1 ./test/unit/...`

Expected: 如果 `cmd/server.go` 尚未适配，可能出现 `RegisterAPIRouter` 调用签名错误。该错误将在 Task 5 修复。

## Task 4: 新增 bootstrap 容器

**Files:**
- Create: `internal/bootstrap/container.go`
- Create: `internal/bootstrap/router.go`

- [ ] **Step 1: 创建 server 结构和 BuildServer**

`internal/bootstrap/container.go` 新增：

```go
package bootstrap

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/api"
	"go.uber.org/dig"
)

type Server struct {
	App     *fiber.App
	HumaAPI huma.API
}

func BuildServer() (*Server, error) {
	container := dig.New()
	if err := provide(container); err != nil {
		return nil, err
	}
	var server *Server
	if err := container.Invoke(func(s *Server) {
		server = s
	}); err != nil {
		return nil, err
	}
	return server, nil
}

func newServer(app *fiber.App, humaAPI huma.API) *Server {
	api.SetFiberApp(app)
	api.SetHumaAPI(humaAPI)
	return &Server{App: app, HumaAPI: humaAPI}
}
```

- [ ] **Step 2: 注册基础 API providers**

在同一文件中添加 `provide`，先注册 Fiber、Huma、Server：

```go
func provide(container *dig.Container) error {
	providers := []any{
		api.NewFiberApp,
		api.NewHumaAPI,
		newServer,
	}
	for _, provider := range providers {
		if err := container.Provide(provider); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 3: 创建路由注册入口**

`internal/bootstrap/router.go` 新增：

```go
package bootstrap

import (
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/router"
)

type RouteParams struct {
	dig.In

	Server           *Server
	PingHandler      handler.PingHandler
	TokenHandler     handler.TokenHandler
	Oauth2Handler    handler.Oauth2Handler
	UserHandler      handler.UserHandler
	APIKeyHandler    handler.APIKeyHandler
	SessionHandler   handler.SessionHandler
	OpenAIHandler    handler.OpenAIHandler
	AnthropicHandler handler.AnthropicHandler
}

func RegisterRoutes(container *dig.Container) error {
	return container.Invoke(func(params RouteParams) {
		if config.Env != enum.EnvProduction {
			router.RegisterDocsRouter(params.Server.App)
		}
		router.RegisterAPIRouter(params.Server.HumaAPI, router.APIRouterDependencies{
			PingHandler:      params.PingHandler,
			TokenHandler:     params.TokenHandler,
			Oauth2Handler:    params.Oauth2Handler,
			UserHandler:      params.UserHandler,
			APIKeyHandler:    params.APIKeyHandler,
			SessionHandler:   params.SessionHandler,
			OpenAIHandler:    params.OpenAIHandler,
			AnthropicHandler: params.AnthropicHandler,
		})
	})
}
```

实际实现时需要在 `router.go` 中导入 `go.uber.org/dig`。

## Task 5: 注册业务 providers

**Files:**
- Modify: `internal/bootstrap/container.go`

- [ ] **Step 1: 添加 provider helper**

在 `internal/bootstrap/container.go` 中扩展 provider 列表，包含 repository、transport、平台、签名器、handler dependencies 和 handlers。

需要导入：

```go
import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	apikeyservice "github.com/hcd233/aris-proxy-api/internal/domain/apikey/service"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/oauth2"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)
```

- [ ] **Step 2: 注册 constructors**

`providers` 至少包含：

```go
repository.NewUserRepository,
repository.NewAPIKeyRepository,
repository.NewSessionReadRepository,
repository.NewEndpointRepository,
repository.NewEndpointReadRepository,
repository.NewAudioDirCreator,
transport.NewOpenAIProxy,
transport.NewAnthropicProxy,
apikeyservice.NewAPIKeyGenerator,
jwt.GetAccessTokenSigner,
jwt.GetRefreshTokenSigner,
newOauth2Platforms,
newTokenDependencies,
newOauth2Dependencies,
newUserDependencies,
newAPIKeyDependencies,
newSessionDependencies,
newOpenAIDependencies,
newAnthropicDependencies,
handler.NewPingHandler,
handler.NewTokenHandler,
handler.NewOauth2Handler,
handler.NewUserHandler,
handler.NewAPIKeyHandler,
handler.NewSessionHandler,
handler.NewOpenAIHandler,
handler.NewAnthropicHandler,
```

- [ ] **Step 3: 解决同类型依赖冲突**

如果 `dig` 因多个 provider 返回相同接口类型报错，使用具名 provider 或 provider wrapper。`EndpointRepository`、`EndpointReadRepository`、`UserRepository` 等接口类型不同，正常情况下无需命名。两个 token signer 返回相同接口类型，必须命名：

```go
if err := container.Provide(jwt.GetAccessTokenSigner, dig.Name("accessSigner")); err != nil {
	return err
}
if err := container.Provide(jwt.GetRefreshTokenSigner, dig.Name("refreshSigner")); err != nil {
	return err
}
```

对应依赖结构体 provider 使用 `dig.In`：

```go
type tokenDependencyParams struct {
	dig.In
	UserRepo      identity.UserRepository
	AccessSigner  identityservice.TokenSigner `name:"accessSigner"`
	RefreshSigner identityservice.TokenSigner `name:"refreshSigner"`
}

func newTokenDependencies(params tokenDependencyParams) handler.TokenDependencies {
	return handler.TokenDependencies{
		UserRepo:      params.UserRepo,
		AccessSigner:  params.AccessSigner,
		RefreshSigner: params.RefreshSigner,
	}
}
```

- [ ] **Step 4: 新增各 handler dependency provider**

为每个 `handler.XxxDependencies` 写一个小函数，只做字段映射。例如：

```go
func newOauth2Platforms() handler.Oauth2Platforms {
	return handler.Oauth2Platforms{
		constant.OAuthProviderGithub: oauth2.NewGithubPlatform(),
		constant.OAuthProviderGoogle: oauth2.NewGooglePlatform(),
	}
}
```

OpenAI 和 Anthropic dependency provider 都复用 endpoint repo/read repo 和 proxies。

## Task 6: 接入 server 启动流程

**Files:**
- Modify: `cmd/server.go`

- [ ] **Step 1: 替换 app 获取和路由注册**

删除 `internal/api` 和 `internal/router` 的直接 import，新增：

```go
"github.com/hcd233/aris-proxy-api/internal/bootstrap"
```

将：

```go
app := api.GetFiberApp()
```

替换为：

```go
server, err := bootstrap.BuildServer()
if err != nil {
	logger.Logger().Error("[Server] Build server failed", zap.Error(err))
	os.Exit(1)
}
app := server.App
```

- [ ] **Step 2: 替换路由注册**

将原来的 docs 和 API 注册：

```go
if config.Env != enum.EnvProduction {
	router.RegisterDocsRouter()
}
router.RegisterAPIRouter()
```

替换为：

```go
if err := bootstrap.RegisterRoutes(server.Container); err != nil {
	logger.Logger().Error("[Server] Register routes failed", zap.Error(err))
	os.Exit(1)
}
```

如果 `Server` 结构不暴露 `Container`，则改为 `bootstrap.BuildServer()` 内部完成 route registration，或让 `BuildServer` 返回 `Server{App, HumaAPI, Container}`。

- [ ] **Step 3: 保持中间件注册顺序**

确保全局 middleware 仍在 route registration 之前执行。最终顺序应是：build server -> `app.Use(...)` -> `bootstrap.RegisterRoutes(...)` -> `app.Listen(...)`。

## Task 7: 添加 bootstrap 回归测试

**Files:**
- Create: `test/unit/bootstrap/bootstrap_test.go`

- [ ] **Step 1: 写容器构建测试**

新增测试：

```go
package bootstrap

import (
	"testing"

	appbootstrap "github.com/hcd233/aris-proxy-api/internal/bootstrap"
)

func TestBuildServer(t *testing.T) {
	server, err := appbootstrap.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}
	if server == nil {
		t.Fatal("BuildServer() returned nil server")
	}
	if server.App == nil {
		t.Fatal("BuildServer() returned nil Fiber app")
	}
	if server.HumaAPI == nil {
		t.Fatal("BuildServer() returned nil Huma API")
	}
}
```

- [ ] **Step 2: 写路由注册测试**

同文件新增：

```go
func TestRegisterRoutes(t *testing.T) {
	server, err := appbootstrap.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}
	if err := appbootstrap.RegisterRoutes(server.Container); err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}
}
```

如果最终不暴露 `Container`，测试应调用最终公开的 route registration API。

- [ ] **Step 3: 运行聚焦测试**

Run: `go test -v -count=1 ./test/unit/bootstrap/`

Expected: PASS。

## Task 8: 全量验证和清理

**Files:**
- Modify as needed: touched Go files

- [ ] **Step 1: 格式化**

Run: `gofmt -w cmd/server.go internal/api/fiber.go internal/api/huma.go internal/bootstrap/*.go internal/router/*.go test/unit/bootstrap/bootstrap_test.go`

Expected: 无错误。

- [ ] **Step 2: 运行聚焦测试**

Run: `go test -v -count=1 ./test/unit/bootstrap/`

Expected: PASS。

- [ ] **Step 3: 运行单元测试**

Run: `go test -count=1 ./test/unit/...`

Expected: PASS。

- [ ] **Step 4: 运行全量测试**

Run: `go test -count=1 ./...`

Expected: PASS；E2E 默认因缺少 `BASE_URL` 和 `API_KEY` skip。

- [ ] **Step 5: 运行规范扫描**

Run: `make lint-conv`

Expected: PASS。

- [ ] **Step 6: 检查 git diff**

Run: `git diff --stat` 和 `git diff -- go.mod go.sum cmd/server.go internal/api internal/bootstrap internal/router test/unit/bootstrap docs/superpowers`

Expected: diff 只包含本计划范围内的文件。

## 自检

- spec 覆盖：计划覆盖依赖加入、bootstrap 组合根、Fiber/Huma provider、router 去构造化、server 接入、测试和验证。
- 占位扫描：没有保留 TBD/TODO；涉及最终结构分支的地方明确要求按最终公开 API 调整。
- 类型一致性：计划中 `Server`、`RouteParams`、`APIRouterDependencies`、handler 接口和 provider 函数都与现有包边界一致。
