package bootstrap

import (
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/router"
	"github.com/redis/go-redis/v9"
	"go.uber.org/dig"
	"gorm.io/gorm"
)

type routeParams struct {
	dig.In

	DB               *gorm.DB
	RedisClient      *redis.Client
	PingHandler      handler.PingHandler
	TokenHandler     handler.TokenHandler
	Oauth2Handler    handler.Oauth2Handler
	UserHandler      handler.UserHandler
	APIKeyHandler    handler.APIKeyHandler
	SessionHandler   handler.SessionHandler
	AuditHandler     handler.AuditHandler
	OpenAIHandler    handler.OpenAIHandler
	AnthropicHandler handler.AnthropicHandler
}

// RegisterRoutes 注册文档和 API 路由。
//
//	@param server *Server
//	@return error
//	@author centonhuang
//	@update 2026-04-28 10:00:00
func RegisterRoutes(server *Server) error {
	return server.container.Invoke(func(params routeParams) {
		if config.Env != enum.EnvProduction {
			router.RegisterDocsRouter(server.App)
		}
		router.RegisterAPIRouter(server.HumaAPI, router.APIRouterDependencies{
			DB:               params.DB,
			RedisClient:      params.RedisClient,
			PingHandler:      params.PingHandler,
			TokenHandler:     params.TokenHandler,
			Oauth2Handler:    params.Oauth2Handler,
			UserHandler:      params.UserHandler,
			APIKeyHandler:    params.APIKeyHandler,
			SessionHandler:   params.SessionHandler,
			AuditHandler:     params.AuditHandler,
			OpenAIHandler:    params.OpenAIHandler,
			AnthropicHandler: params.AnthropicHandler,
		})
	})
}
