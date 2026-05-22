# GoFiber v3 升级迁移实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 aris-proxy-api 从 gofiber/fiber/v2 升级到 gofiber/fiber/v3，同步更新 huma 适配器，用 SendStreamWriter 替代 fasthttp.StreamWriter。

**Architecture:** 一次性全面升级，不做 v2/v3 共存。按依赖层从底到顶：先升级 go.mod 依赖 → 修改核心 App/Config → 迁移中间件 → 迁移 handler/usecase/util → 迁移测试。

**Tech Stack:** Go 1.25.1, gofiber/fiber/v3, danielgtaylor/huma/v2 v2.38.0+, gofiber/contrib/v3/fgprof

---

### Task 1: 升级 go.mod 依赖

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: 替换 fiber v2 为 v3 并升级 huma**

```bash
go get github.com/gofiber/fiber/v3@latest
go get github.com/danielgtaylor/huma/v2@v2.38.0
```

- [ ] **Step 2: 替换 fgprof 为 v3 版本**

```bash
go get github.com/gofiber/contrib/v3/fgprof@latest
```

- [ ] **Step 3: 移除不再需要的直接依赖**

```bash
go mod tidy
```

验证 `go.mod` 中不再包含 `github.com/gofiber/fiber/v2` 和 `github.com/gofiber/contrib/fgprof v1`。

- [ ] **Step 4: 确认编译错误**

```bash
go build ./...
```

Expected: 编译失败（import 路径仍是 v2），这是预期行为，后续 Task 逐步修复。

---

### Task 2: 迁移 Fiber App 初始化和 Huma API 创建

**Files:**
- Modify: `internal/api/fiber.go`
- Modify: `internal/api/huma.go`

- [ ] **Step 1: 修改 `internal/api/fiber.go`**

将 `fiber/v2` import 改为 `fiber/v3`，迁移 Config 字段：

```go
package api

import (
	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
)

func NewFiberApp() *fiber.App {
	return fiber.New(fiber.Config{
		EnablePrefork:             false,
		ReadTimeout:               config.ReadTimeout,
		WriteTimeout:              config.WriteTimeout,
		IdleTimeout:               constant.IdleTimeout,
		JSONEncoder:               sonic.Marshal,
		JSONDecoder:               sonic.Unmarshal,
		DisableHeaderNormalizing:  true,
		TrustProxy:                true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies:     config.TrustedProxies,
			ProxyHeader: fiber.HeaderXForwardedFor,
		},
	})
}
```

关键变更：
- `Prefork` → `EnablePrefork`
- `EnableTrustedProxyCheck` + `TrustedProxies` + `ProxyHeader` → `TrustProxy: true` + `TrustProxyConfig{Proxies, ProxyHeader}`

- [ ] **Step 2: 修改 `internal/api/huma.go`**

将 `fiber/v2` import 改为 `fiber/v3`：

```go
package api

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/samber/lo"
)

func NewHumaAPI(app *fiber.App) huma.API {
	return humafiber.New(app, huma.Config{
		OpenAPI: &huma.OpenAPI{
			OpenAPI: constant.OpenAPIVersion,
			Info: &huma.Info{
				Title:       constant.APITitle,
				Description: constant.APIDescription,
				Version:     constant.APIVersion,
				Contact: &huma.Contact{
					Name:  constant.ContactName,
					Email: constant.ContactEmail,
					URL:   constant.ContactURL,
				},
				License: &huma.License{
					Name: constant.LicenseName,
					URL:  constant.LicenseURL,
				},
			},
			Components: &huma.Components{
				Schemas: huma.NewMapRegistry(constant.OpenAPISchemasPrefix, huma.DefaultSchemaNamer),
				SecuritySchemes: map[string]*huma.SecurityScheme{
					constant.SecuritySchemeJWT: {
						Type:        constant.SecurityTypeAPIKey,
						Name:        constant.HeaderAuthorization,
						In:          constant.SecurityInHeader,
						Description: constant.JWTDescription,
					},
					constant.SecuritySchemeAPIKey: {
						Type:        constant.SecurityTypeHTTP,
						Scheme:      constant.SecuritySchemeBearer,
						Description: constant.APIKeyDescription,
					},
				},
			},
		},
		OpenAPIPath:   lo.If(config.Env != enum.EnvProduction, constant.OpenAPIDocsPath).Else(""),
		DocsPath:      "",
		SchemasPath:   lo.If(config.Env != enum.EnvProduction, constant.OpenAPISchemasPath).Else(""),
		Formats:       huma.DefaultFormats,
		DefaultFormat: constant.DefaultFormatJSON,
	})
}
```

注意：`humafiber.New()` 在 v2.38.0 中默认接受 `*fiber/v3.App`。

---

### Task 3: 迁移 Bootstrap 容器和 Server 启动

**Files:**
- Modify: `internal/bootstrap/container.go`
- Modify: `cmd/server.go`

- [ ] **Step 1: 修改 `internal/bootstrap/container.go` 的 import**

将 `"github.com/gofiber/fiber/v2"` 改为 `"github.com/gofiber/fiber/v3"`。

- [ ] **Step 2: 修改 `cmd/server.go`**

import 变更：`"github.com/gofiber/fiber/v2"` → `"github.com/gofiber/fiber/v3"`

关键变更：
- `app.Listen(listenAddr)` 在 v3 中签名变为 `app.Listen(addr, ...fiber.ListenConfig)`，但 addr 参数仍兼容，可直接传 `listenAddr`
- `app.ShutdownWithTimeout(constant.FiberShutdownTimeout)` → `app.Shutdown()` (v3 的 Shutdown 接受 context，不再有 ShutdownWithTimeout)

gracefulShutdown 函数中：
```go
if err := app.ShutdownWithTimeout(constant.FiberShutdownTimeout); err != nil {
```
改为：
```go
shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), constant.FiberShutdownTimeout)
defer shutdownCancel()
if err := app.Shutdown(shutdownCtx); err != nil {
```

注意：`context` 包已在该文件 import 中。

---

### Task 4: 迁移 Logger（WithFCtx 签名变更）

**Files:**
- Modify: `internal/logger/logger.go`

- [ ] **Step 1: 修改 import 和 WithFCtx 签名**

`*fiber.Ctx` → `fiber.Ctx`

```go
import (
    // ...
    "github.com/gofiber/fiber/v3"
    // ...
)

func WithFCtx(c fiber.Ctx) *zap.Logger {
    // 函数体不变，Locals 方法在接口上同样存在
```

---

### Task 5: 迁移中间件文件（7 个文件）

**Files:**
- Modify: `internal/middleware/recover.go`
- Modify: `internal/middleware/cors.go`
- Modify: `internal/middleware/compress.go`
- Modify: `internal/middleware/fgprof.go`
- Modify: `internal/middleware/trace.go`
- Modify: `internal/middleware/guard.go`
- Modify: `internal/middleware/log.go`
- Modify: `internal/middleware/apikey.go`

- [ ] **Step 1: 修改 `internal/middleware/recover.go`**

import 变更：
- `"github.com/gofiber/fiber/v2"` → `"github.com/gofiber/fiber/v3"`
- `"github.com/gofiber/fiber/v2/middleware/recover"` → `"github.com/gofiber/fiber/v3/middleware/recover"`

签名变更 `*fiber.Ctx` → `fiber.Ctx`：
```go
StackTraceHandler: func(c fiber.Ctx, e any) {
```

- [ ] **Step 2: 修改 `internal/middleware/cors.go`**

import 变更：
- `"github.com/gofiber/fiber/v2"` → `"github.com/gofiber/fiber/v3"`
- `"github.com/gofiber/fiber/v2/middleware/cors"` → `"github.com/gofiber/fiber/v3/middleware/cors"`

- [ ] **Step 3: 修改 `internal/middleware/compress.go`**

import 变更：
- `"github.com/gofiber/fiber/v2"` → `"github.com/gofiber/fiber/v3"`
- `"github.com/gofiber/fiber/v2/middleware/compress"` → `"github.com/gofiber/fiber/v3/middleware/compress"`

- [ ] **Step 4: 修改 `internal/middleware/fgprof.go`**

import 变更：
- `"github.com/gofiber/contrib/fgprof"` → `"github.com/gofiber/contrib/v3/fgprof"`
- `"github.com/gofiber/fiber/v2"` → `"github.com/gofiber/fiber/v3"`

- [ ] **Step 5: 修改 `internal/middleware/trace.go`**

import 变更：`"github.com/gofiber/fiber/v2"` → `"github.com/gofiber/fiber/v3"`

签名变更 `*fiber.Ctx` → `fiber.Ctx`：
```go
return func(c fiber.Ctx) error {
```

- [ ] **Step 6: 修改 `internal/middleware/guard.go`**

import 变更：`"github.com/gofiber/fiber/v2"` → `"github.com/gofiber/fiber/v3"`

签名变更：
- `func(c *fiber.Ctx) error` → `func(c fiber.Ctx) error`
- `c.Context()` (获取 fasthttp 上下文) → `c.RequestCtx()` (v3 重命名)

在 GuardMiddleware 函数体中，`ctx := c.Context()` 改为 `ctx := c.RequestCtx()`。

- [ ] **Step 7: 修改 `internal/middleware/log.go`**

import 变更：`"github.com/gofiber/fiber/v2"` → `"github.com/gofiber/fiber/v3"`

签名变更 `*fiber.Ctx` → `fiber.Ctx`：

```go
return func(c fiber.Ctx) error {
```

此外，v3 中 `c.Request().Header.All()` 可能返回类型不同，需确认。在 v3 中 `c.Request()` 仍返回 `*fasthttp.Request`，`Header.All()` 签名不变。

- [ ] **Step 8: 修改 `internal/middleware/apikey.go`**

import 变更：`"github.com/gofiber/fiber/v2"` → `"github.com/gofiber/fiber/v3"`

`fiber.StatusUnauthorized`、`fiber.StatusInternalServerError` 等常量在 v3 中不变。

---

### Task 6: 迁移 SSE 流式写入（核心变更 - SendStreamWriter）

**Files:**
- Modify: `internal/api/util/http.go`
- Modify: `internal/handler/ping.go`
- Modify: `internal/application/llmproxy/util/sse.go`

- [ ] **Step 1: 修改 `internal/api/util/http.go`**

import 变更：
- `"github.com/gofiber/fiber/v2"` → `"github.com/gofiber/fiber/v3"`
- 移除 `"github.com/valyala/fasthttp"`

WrapStreamResponse 中的 `SetBodyStreamWriter` 替换为 `SendStreamWriter`：

```go
func WrapStreamResponse(ctx context.Context, handler func(w *bufio.Writer)) *huma.StreamResponse {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			fiberCtx := humafiber.Unwrap(humaCtx)
			if headers := util.GetPassthroughResponseHeaders(ctx); headers != nil {
				for k, hv := range headers {
					fiberCtx.Set(k, hv)
				}
			}
			fiberCtx.Set(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeEventStream)
			fiberCtx.Set(constant.HTTPTitleHeaderCacheControl, constant.HTTPCacheControlNoCache)
			fiberCtx.Set(constant.HTTPLowerHeaderConnection, constant.HTTPConnectionKeepAlive)
			fiberCtx.Set(constant.HTTPLowerHeaderTransferEncoding, constant.HTTPTransferEncodingChunked)
			fiberCtx.Set(constant.HTTPTitleHeaderXAccelBuffering, constant.HTTPHeaderDisabled)
			fiberCtx.Status(fiber.StatusOK)
			fiberCtx.SendStreamWriter(handler)
		},
	}
}
```

关键变更：
- `fiberCtx.Status(fiber.StatusOK).Response().SetBodyStreamWriter(fasthttp.StreamWriter(handler))` → `fiberCtx.Status(fiber.StatusOK)` 然后 `fiberCtx.SendStreamWriter(handler)`
- `handler` 参数类型从 `func(w *bufio.Writer)` 变为 `func(w *bufio.Writer)`（签名不变，SendStreamWriter 接受相同的回调类型）

WriteUpstreamError 中的 `fiber.StatusBadGateway` 不变。

- [ ] **Step 2: 修改 `internal/handler/ping.go`**

import 变更：
- 移除 `"github.com/valyala/fasthttp"`
- 添加 `"github.com/gofiber/fiber/v3"`

HandleSSEPing 中的 `SetBodyStreamWriter` 替换为 `SendStreamWriter`：

```go
func (h *pingHandler) HandleSSEPing(_ context.Context, _ *dto.EmptyReq) (rsp *huma.StreamResponse, err error) {
	return &huma.StreamResponse{
		Body: func(ctx huma.Context) {
			fCtx := humafiber.Unwrap(ctx)
			fCtx.Set(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeEventStream)
			fCtx.Set(constant.HTTPTitleHeaderCacheControl, constant.HTTPCacheControlNoCache)
			fCtx.Set(constant.HTTPLowerHeaderConnection, constant.HTTPConnectionKeepAlive)
			fCtx.Set(constant.HTTPLowerHeaderTransferEncoding, constant.HTTPTransferEncodingChunked)
			fCtx.Set(constant.HTTPTitleHeaderXAccelBuffering, constant.HTTPHeaderDisabled)
			fCtx.SendStreamWriter(func(w *bufio.Writer) {
				for i := range constant.SSEHeartbeatCount {
					data := &dto.SSEResponse{
						DataType: enum.SSEDataTypeHeartBeat,
						Data:     strconv.Itoa(i),
					}
					_, _ = fmt.Fprintf(w, constant.SSEDataFrameTemplate, lo.Must1(sonic.Marshal(data)))
					err := w.Flush()
					if err != nil {
						return
					}
					time.Sleep(constant.HeartbeatInterval)
				}
			})
		},
	}, nil
}
```

关键变更：
- `fCtx.Response().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {` → `fCtx.SendStreamWriter(func(w *bufio.Writer) {`
- 去掉 `fasthttp.StreamWriter` 包装

- [ ] **Step 3: 修改 `internal/application/llmproxy/util/sse.go`**

import 变更：
- 移除 `"github.com/valyala/fasthttp"`
- 添加 `"github.com/gofiber/fiber/v3"`

WrapErrorSSE 中替换：

```go
func WrapErrorSSE(ctx context.Context, err *model.Error) (rsp *huma.StreamResponse) {
	return &huma.StreamResponse{
		Body: func(hCtx huma.Context) {
			fCtx := humafiber.Unwrap(hCtx)
			fCtx.Set(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeEventStream)
			fCtx.Set(constant.HTTPTitleHeaderCacheControl, constant.HTTPCacheControlNoCache)
			fCtx.Set(constant.HTTPLowerHeaderConnection, constant.HTTPConnectionKeepAlive)
			fCtx.Set(constant.HTTPLowerHeaderTransferEncoding, constant.HTTPTransferEncodingChunked)
			fCtx.Set(constant.HTTPTitleHeaderXAccelBuffering, constant.HTTPHeaderDisabled)
			fCtx.SendStreamWriter(func(w *bufio.Writer) {
				writeSSEErrorResponse(ctx, w, err)
			})
		},
	}
}
```

关键变更：`fCtx.Response().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {` → `fCtx.SendStreamWriter(func(w *bufio.Writer) {`

---

### Task 7: 迁移 usecase 层 fiber 状态码引用

**Files:**
- Modify: `internal/application/llmproxy/usecase/openai_chat_unary.go`
- Modify: `internal/application/llmproxy/usecase/openai_response_unary.go`
- Modify: `internal/application/llmproxy/usecase/anthropic_message_unary.go`

- [ ] **Step 1: 修改三个文件的 import**

将 `"github.com/gofiber/fiber/v2"` 改为 `"github.com/gofiber/fiber/v3"`。

这三个文件只使用了 `fiber.StatusOK` 等常量，常量名在 v3 中不变。

---

### Task 8: 迁移路由文件

**Files:**
- Modify: `internal/router/router.go`

- [ ] **Step 1: 修改 import 和 RegisterDocsRouter 签名**

import 变更：`"github.com/gofiber/fiber/v2"` → `"github.com/gofiber/fiber/v3"`

签名变更 `*fiber.Ctx` → `fiber.Ctx`：

```go
func RegisterDocsRouter(app *fiber.App) {
	app.Get("/docs", func(c fiber.Ctx) error {
```

---

### Task 9: 迁移测试文件

**Files:**
- Modify: `test/unit/header_passthrough/header_passthrough_test.go`

- [ ] **Step 1: 修改 import 和 Fiber App 创建**

import 变更：
- `"github.com/gofiber/fiber/v2"` → `"github.com/gofiber/fiber/v3"`

`app.Test(req, -1)` → `app.Test(req, fiber.TestConfig{Timeout: 0})`：

```go
resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
```

---

### Task 10: 全局 import 批量替换与编译验证

**Files:**
- 所有包含 `fiber/v2` 的文件

- [ ] **Step 1: 全局搜索确认无遗漏的 v2 import**

```bash
grep -rn "gofiber/fiber/v2" --include="*.go" .
grep -rn "gofiber/contrib/fgprof\"" --include="*.go" .
grep -rn "valyala/fasthttp" --include="*.go" .
```

Expected: 零结果。如有遗漏，修改对应文件。

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

Expected: 编译成功，零错误。

- [ ] **Step 3: 运行 lint**

```bash
make lint
```

Expected: lint 通过。

- [ ] **Step 4: 运行单元测试**

```bash
go test -count=1 ./test/unit/...
```

Expected: 所有测试通过。

- [ ] **Step 5: 运行全量测试**

```bash
make test
```

Expected: 所有测试通过。

- [ ] **Step 6: 提交**

```bash
git add -A
git commit -m "feat: upgrade gofiber from v2 to v3 with humafiber adapter migration

- Upgrade gofiber/fiber from v2 to v3
- Upgrade huma to v2.38.0 (humafiber v3 support)
- Migrate fgprof to gofiber/contrib/v3/fgprof
- Replace fasthttp.StreamWriter with fiber.SendStreamWriter for SSE
- Update *fiber.Ctx to fiber.Ctx (interface)
- Migrate fiber.Config: Prefork->EnablePrefork, TrustedProxy->TrustProxy
- Migrate app.ShutdownWithTimeout to app.Shutdown(ctx)
- Migrate app.Test(req, -1) to app.Test(req, fiber.TestConfig{Timeout: 0})"
```
