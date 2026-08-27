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

// RegisterOpsRouter 注册运维分区路由：健康检查常驻，文档与 pprof(fgprof) 仅非生产开放。
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
