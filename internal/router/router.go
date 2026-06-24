// Package router 路由
package router

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// APIRouterDependencies API 路由依赖
//
//	@author centonhuang
//	@update 2026-04-28 10:00:00
type APIRouterDependencies struct {
	DB               *gorm.DB
	Cache            *redis.Client
	AccessSigner     jwt.TokenSigner
	PingHandler      handler.PingHandler
	TokenHandler     handler.TokenHandler
	Oauth2Handler    handler.Oauth2Handler
	UserHandler      handler.UserHandler
	APIKeyHandler    handler.APIKeyHandler
	SessionHandler   handler.SessionHandler
	EndpointHandler  handler.EndpointHandler
	ModelHandler     handler.ModelHandler
	AuditHandler     handler.AuditHandler
	CronHandler      handler.CronHandler
	OpenAIHandler    handler.OpenAIHandler
	AnthropicHandler handler.AnthropicHandler
	BlockedHandler   handler.BlockedHandler
	MetricsHandler   handler.MetricsHandler
}

// RegisterDocsRouter 注册文档路由
//
//	@author centonhuang
//	@update 2025-11-10 17:26:08
func RegisterDocsRouter(app *fiber.App) {
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

	tokenGroup := huma.NewGroup(v1Group, "/token")
	initTokenRouter(tokenGroup, deps.TokenHandler, deps.Cache)

	oauth2Group := huma.NewGroup(v1Group, "/oauth2")
	initOauth2Router(oauth2Group, deps.Oauth2Handler, deps.Cache)

	userGroup := huma.NewGroup(v1Group, "/user")
	initUserRouter(userGroup, deps.UserHandler, deps.DB, deps.Cache, deps.AccessSigner)

	apikeyGroup := huma.NewGroup(v1Group, "/apikey")
	initAPIKeyRouter(apikeyGroup, deps.APIKeyHandler, deps.DB, deps.Cache, deps.AccessSigner)

	sessionJWTGroup := huma.NewGroup(v1Group, "/session")
	initSessionJWTRouter(sessionJWTGroup, deps.SessionHandler, deps.DB, deps.Cache, deps.AccessSigner)

	sessionPublicGroup := huma.NewGroup(v1Group, "/session")
	initSessionPublicRouter(sessionPublicGroup, deps.SessionHandler, deps.Cache)

	endpointGroup := huma.NewGroup(v1Group, "/endpoint")
	initEndpointRouter(endpointGroup, deps.EndpointHandler, deps.DB, deps.Cache, deps.AccessSigner)

	modelGroup := huma.NewGroup(v1Group, "/model")
	initModelRouter(modelGroup, deps.ModelHandler, deps.DB, deps.Cache, deps.AccessSigner)

	auditGroup := huma.NewGroup(v1Group, "/audit")
	initAuditRouter(auditGroup, deps.AuditHandler, deps.CronHandler, deps.DB, deps.Cache, deps.AccessSigner)

	cronGroup := huma.NewGroup(v1Group, "/cron")
	initCronRouter(cronGroup, deps.CronHandler, deps.DB, deps.Cache, deps.AccessSigner)

	blockedGroup := huma.NewGroup(v1Group, "/block")
	initBlockedRouter(blockedGroup, deps.BlockedHandler, deps.DB, deps.Cache, deps.AccessSigner)

	openaiGroup := huma.NewGroup(apiGroup, "/openai/v1")
	initOpenAIRouter(openaiGroup, deps.OpenAIHandler, deps.DB, deps.Cache)

	anthropicGroup := huma.NewGroup(apiGroup, "/anthropic/v1")
	initAnthropicRouter(anthropicGroup, deps.AnthropicHandler, deps.DB, deps.Cache)

	metricsGroup := huma.NewGroup(v1Group, "/metrics")
	initMetricsRouter(metricsGroup, deps.MetricsHandler, deps.DB, deps.Cache, deps.AccessSigner)
}
