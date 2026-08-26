// Package router 路由
package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
)

// APIRouterDependencies API 路由依赖
//
//	@author centonhuang
//	@update 2026-08-16 10:00:00
type APIRouterDependencies struct {
	DB                 *gorm.DB
	Cache              *redis.Client
	AccessSigner       jwt.TokenSigner
	DemoModuleAccessor demoport.DemoModuleAccessor
	DemoAuditSubmitter demoport.DemoSubmitter
	PingHandler        handler.PingHandler
	TokenHandler       handler.TokenHandler
	Oauth2Handler      handler.Oauth2Handler
	UserHandler        handler.UserHandler
	DemoHandler        handler.DemoHandler
	APIKeyHandler      handler.APIKeyHandler
	SessionHandler     handler.SessionHandler
	EndpointHandler    handler.EndpointHandler
	ModelHandler       handler.ModelHandler
	AuditHandler       handler.AuditHandler
	CronHandler        handler.CronHandler
	OpenAIHandler      handler.OpenAIHandler
	AnthropicHandler   handler.AnthropicHandler
	TriggerHandler     handler.TriggerHandler
	MetricsHandler     handler.MetricsHandler
	DatasetHandler     handler.DatasetHandler
	TraceHandler       handler.TraceHandler
	ClientHandler      handler.ClientHandler
	UpstreamHandler    handler.UpstreamHandler
}

// RegisterDocsRouter 注册文档路由
//
// 仅非生产环境开放：生产环境不注册 /docs，避免暴露 API 文档入口。
// OpenAPI JSON 的暴露也由 internal/api/huma.go 的 OpenAPIPath 按环境控制。
//
//	@author centonhuang
//	@update 2025-11-10 17:26:08
func RegisterDocsRouter(app *fiber.App) {
	if config.Env == enum.EnvProduction {
		return
	}
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

// RegisterAPIRouter 注册API路由
//
//	@author centonhuang
//	@update 2025-11-10 17:26:08
func RegisterAPIRouter(humaAPI huma.API, deps APIRouterDependencies) {
	apiGroup := huma.NewGroup(humaAPI, "/api")
	v1Group := huma.NewGroup(apiGroup, "/v1")

	initHealthRouter(humaAPI, deps.PingHandler)

	huma.Register(humaAPI, huma.Operation{
		OperationID: "installTraceScript", Method: http.MethodGet, Path: "/install.sh",
		Summary:     "InstallTraceScript",
		Description: "Return the self-contained Aris trace client install script",
		Tags:        []string{constant.TagTrace},
	}, deps.TraceHandler.HandleInstallScript)

	tokenGroup := huma.NewGroup(v1Group, "/token")
	clientAPIGroup := huma.NewGroup(v1Group, "/client")
	initClientRouter(clientAPIGroup, deps.ClientHandler, deps.DB)
	initTokenRouter(tokenGroup, deps.TokenHandler, deps.Cache)

	oauth2Group := huma.NewGroup(v1Group, "/oauth2")
	initOauth2Router(oauth2Group, deps.Oauth2Handler, deps.Cache)

	userGroup := huma.NewGroup(v1Group, "/user")
	initUserRouter(userGroup, deps.UserHandler, deps.DB, deps.Cache, deps.AccessSigner)

	demoGroup := huma.NewGroup(v1Group, "/demo")
	initDemoRouter(demoGroup, deps.DemoHandler, deps.DB, deps.Cache, deps.AccessSigner)

	apikeyGroup := huma.NewGroup(v1Group, "/apikey")
	initAPIKeyRouter(apikeyGroup, deps.APIKeyHandler, deps.DB, deps.Cache, deps.AccessSigner)

	sessionJWTGroup := huma.NewGroup(v1Group, "/session")
	initSessionJWTRouter(sessionJWTGroup, deps.SessionHandler, deps.DB, deps.Cache, deps.AccessSigner, deps.DemoModuleAccessor, deps.DemoAuditSubmitter)

	sessionPublicGroup := huma.NewGroup(v1Group, "/session")
	initSessionPublicRouter(sessionPublicGroup, deps.SessionHandler, deps.Cache)

	endpointGroup := huma.NewGroup(v1Group, "/endpoint")
	initEndpointRouter(endpointGroup, deps.EndpointHandler, deps.DB, deps.Cache, deps.AccessSigner)

	modelGroup := huma.NewGroup(v1Group, "/model")
	initModelRouter(modelGroup, deps.ModelHandler, deps.DB, deps.Cache, deps.AccessSigner)

	upstreamGroup := huma.NewGroup(v1Group, "/upstream")
	initUpstreamRouter(upstreamGroup, deps.UpstreamHandler, deps.DB, deps.Cache, deps.AccessSigner, deps.DemoModuleAccessor, deps.DemoAuditSubmitter)

	auditGroup := huma.NewGroup(v1Group, "/audit")
	initAuditRouter(auditGroup, deps.AuditHandler, deps.CronHandler, deps.DB, deps.Cache, deps.AccessSigner, deps.DemoModuleAccessor, deps.DemoAuditSubmitter)

	cronGroup := huma.NewGroup(v1Group, "/cron")
	initCronRouter(cronGroup, deps.CronHandler, deps.DB, deps.Cache, deps.AccessSigner, deps.DemoModuleAccessor, deps.DemoAuditSubmitter)

	triggerGroup := huma.NewGroup(v1Group, "/trigger")
	initTriggerRouter(triggerGroup, deps.TriggerHandler, deps.DB, deps.Cache, deps.AccessSigner, deps.DemoModuleAccessor, deps.DemoAuditSubmitter)

	openaiGroup := huma.NewGroup(apiGroup, "/openai/v1")
	initOpenAIRouter(openaiGroup, deps.OpenAIHandler, deps.DB, deps.Cache)

	anthropicGroup := huma.NewGroup(apiGroup, "/anthropic/v1")
	initAnthropicRouter(anthropicGroup, deps.AnthropicHandler, deps.DB, deps.Cache)

	metricsGroup := huma.NewGroup(v1Group, "/metrics")
	initMetricsRouter(metricsGroup, deps.MetricsHandler, deps.DB, deps.Cache, deps.AccessSigner, deps.DemoModuleAccessor, deps.DemoAuditSubmitter)

	datasetGroup := huma.NewGroup(v1Group, "/dataset")
	initDatasetRouter(datasetGroup, deps.DatasetHandler, deps.DB, deps.Cache, deps.AccessSigner)

	traceGroup := huma.NewGroup(v1Group, "/trace")
	initTraceRouter(traceGroup, TraceRouterDependencies{
		TraceHandler: deps.TraceHandler,
	}, deps.DB, deps.Cache, deps.AccessSigner)
}
