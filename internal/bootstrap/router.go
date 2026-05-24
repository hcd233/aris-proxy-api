package bootstrap

import (
	"fmt"
	"net/url"

	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/application/oauth2/command"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
	appenum "github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/router"
	"github.com/hcd233/aris-proxy-api/internal/web"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"go.uber.org/dig"
	"gorm.io/gorm"
)

type routeParams struct {
	dig.In

	DB                *gorm.DB
	Cache             *redis.Client
	PingHandler       handler.PingHandler
	TokenHandler      handler.TokenHandler
	Oauth2Handler     handler.Oauth2Handler
	Oauth2Callback    command.HandleCallbackHandler
	UserHandler       handler.UserHandler
	APIKeyHandler     handler.APIKeyHandler
	SessionHandler    handler.SessionHandler
	EndpointHandler   handler.EndpointHandler
	ModelHandler      handler.ModelHandler
	AuditHandler      handler.AuditHandler
	OpenAIHandler     handler.OpenAIHandler
	AnthropicHandler  handler.AnthropicHandler
}

// RegisterRoutes 注册文档和 API 路由。
//
//	@param server *Server
//	@return error
//	@author centonhuang
//	@update 2026-04-28 10:00:00
func RegisterRoutes(server *Server) error {
	return server.container.Invoke(func(params routeParams) {
		if config.Env != appenum.EnvProduction {
			router.RegisterDocsRouter(server.App)
		}
		router.RegisterAPIRouter(server.HumaAPI, router.APIRouterDependencies{
			DB:               params.DB,
			Cache:            params.Cache,
			PingHandler:      params.PingHandler,
			TokenHandler:     params.TokenHandler,
			Oauth2Handler:    params.Oauth2Handler,
			UserHandler:      params.UserHandler,
			APIKeyHandler:    params.APIKeyHandler,
			SessionHandler:   params.SessionHandler,
			EndpointHandler:  params.EndpointHandler,
			ModelHandler:     params.ModelHandler,
			AuditHandler:     params.AuditHandler,
			OpenAIHandler:    params.OpenAIHandler,
			AnthropicHandler: params.AnthropicHandler,
		})

		server.App.Get("/api/v1/oauth2/callback", func(c fiber.Ctx) error {
			code := c.Query("code")
			state := c.Query("state")
			platform := c.Query("platform", string(enum.Oauth2PlatformGithub))

			result, err := params.Oauth2Callback.Handle(c.Context(), command.HandleCallbackCommand{
				Platform: enum.Oauth2Platform(platform),
				Code:     code,
				State:    state,
			})
			if err != nil {
				logger.WithCtx(c.Context()).Error("[OAuth2BrowserCallback] Callback failed", zap.Error(err))
				return c.Redirect().Status(fiber.StatusFound).To("/web/login?error=auth_failed")
			}

			redirectURL := fmt.Sprintf("/web/auth/callback?access_token=%s&refresh_token=%s",
				url.QueryEscape(result.TokenPair.AccessToken()),
				url.QueryEscape(result.TokenPair.RefreshToken()),
			)
			return c.Redirect().Status(fiber.StatusFound).To(redirectURL)
		})

		router.RegisterWebRouter(server.App, web.DistFS)
	})
}
