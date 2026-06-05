package bootstrap

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/application/oauth2/port"
	appenum "github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/router"
	"github.com/hcd233/aris-proxy-api/internal/web"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

type routeParams struct {
	fx.In

	DB               *gorm.DB
	Cache            *redis.Client
	App              *fiber.App
	HumaAPI          huma.API
	AccessSigner     identityservice.TokenSigner `name:"accessSigner"`
	PingHandler      handler.PingHandler
	TokenHandler     handler.TokenHandler
	Oauth2Handler    handler.Oauth2Handler
	Oauth2Callback   port.HandleCallbackHandler
	UserHandler      handler.UserHandler
	APIKeyHandler    handler.APIKeyHandler
	SessionHandler   handler.SessionHandler
	EndpointHandler  handler.EndpointHandler
	ModelHandler     handler.ModelHandler
	AuditHandler     handler.AuditHandler
	OpenAIHandler    handler.OpenAIHandler
	AnthropicHandler handler.AnthropicHandler
}

func registerRoutes(params routeParams) {
	if config.Env != appenum.EnvProduction {
		router.RegisterDocsRouter(params.App)
	}
	router.RegisterAPIRouter(params.HumaAPI, router.APIRouterDependencies{
		DB:               params.DB,
		Cache:            params.Cache,
		AccessSigner:     params.AccessSigner,
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

	router.RegisterWebRouter(params.App, web.DistFS)
}
