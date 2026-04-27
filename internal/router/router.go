// Package router 路由
package router

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/handler"
)

// APIRouterDependencies API 路由依赖
//
//	@author centonhuang
//	@update 2026-04-28 10:00:00
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

// RegisterDocsRouter 注册文档路由
//
//	@author centonhuang
//	@update 2025-11-10 17:26:08
func RegisterDocsRouter(app *fiber.App) {
	app.Get("/docs", func(c *fiber.Ctx) error {
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
