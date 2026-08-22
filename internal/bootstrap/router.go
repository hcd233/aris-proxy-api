package bootstrap

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"gorm.io/gorm"

	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	appenum "github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/config"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/router"
	"github.com/hcd233/aris-proxy-api/internal/web"
)

type routeParams struct {
	fx.In

	DB                 *gorm.DB
	Cache              *redis.Client
	App                *fiber.App
	HumaAPI            huma.API
	AccessSigner       identityservice.TokenSigner `name:"accessSigner"`
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
}

func registerRoutes(params routeParams) {
	if config.Env != appenum.EnvProduction {
		router.RegisterDocsRouter(params.App)
	}
	router.RegisterAPIRouter(params.HumaAPI, router.APIRouterDependencies{
		DB:                 params.DB,
		Cache:              params.Cache,
		AccessSigner:       params.AccessSigner,
		DemoModuleAccessor: params.DemoModuleAccessor,
		DemoAuditSubmitter: params.DemoAuditSubmitter,
		PingHandler:        params.PingHandler,
		TokenHandler:       params.TokenHandler,
		Oauth2Handler:      params.Oauth2Handler,
		UserHandler:        params.UserHandler,
		DemoHandler:        params.DemoHandler,
		APIKeyHandler:      params.APIKeyHandler,
		SessionHandler:     params.SessionHandler,
		EndpointHandler:    params.EndpointHandler,
		ModelHandler:       params.ModelHandler,
		AuditHandler:       params.AuditHandler,
		CronHandler:        params.CronHandler,
		OpenAIHandler:      params.OpenAIHandler,
		AnthropicHandler:   params.AnthropicHandler,
		TriggerHandler:     params.TriggerHandler,
		MetricsHandler:     params.MetricsHandler,
		DatasetHandler:     params.DatasetHandler,
		TraceHandler:       params.TraceHandler,
	})

	router.RegisterWebRouter(params.App, web.DistFS)
}
