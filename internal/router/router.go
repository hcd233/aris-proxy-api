// Package router 路由
package router

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/api"
)

// RegisterDocsRouter 注册文档路由
//
//	@author centonhuang
//	@update 2025-11-10 17:26:08
func RegisterDocsRouter() {
	app := api.GetFiberApp()
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
func RegisterAPIRouter() {
	api := api.GetHumaAPI()
	apiGroup := huma.NewGroup(api, "/api")
	v1Group := huma.NewGroup(apiGroup, "/v1")

	initHealthRouter(api)

	tokenGroup := huma.NewGroup(v1Group, "/token")
	initTokenRouter(tokenGroup)

	oauth2Group := huma.NewGroup(v1Group, "/oauth2")
	initOauth2Router(oauth2Group)

	userGroup := huma.NewGroup(v1Group, "/user")
	initUserRouter(userGroup)

	apikeyGroup := huma.NewGroup(v1Group, "/apikey")
	initAPIKeyRouter(apikeyGroup)

	sessionGroup := huma.NewGroup(v1Group, "/session")
	initSessionRouter(sessionGroup)

	openaiGroup := huma.NewGroup(apiGroup, "/openai/v1")
	initOpenAIRouter(openaiGroup)

	anthropicGroup := huma.NewGroup(apiGroup, "/anthropic/v1")
	initAnthropicRouter(anthropicGroup)
}
