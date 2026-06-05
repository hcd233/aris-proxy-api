package bootstrap

import (
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/api"
	"github.com/hcd233/aris-proxy-api/internal/bootstrap/modules"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"go.uber.org/fx"
)

func BuildFxAppOptions(host, port string, customizers ...fx.Option) []fx.Option {
	opts := []fx.Option{
		fx.Supply(
			fx.Annotate(host, fx.ResultTags(`name:"listenHost"`)),
			fx.Annotate(port, fx.ResultTags(`name:"listenPort"`)),
		),
		modules.InfraModule,
		modules.CronModule,
		modules.RepositoryModule,
		modules.ApplicationModule,
		modules.HandlerModule,
		fx.Provide(
			api.NewFiberApp,
			api.NewHumaAPI,
		),
		fx.Invoke(
			registerRoutes,
			registerLifecycleHooks,
		),
		fx.StopTimeout(constant.ShutdownTimeout),
	}
	opts = append(opts, customizers...)
	return opts
}

func BuildFxApp(host, port string, customizers ...fx.Option) *fx.App {
	return fx.New(BuildFxAppOptions(host, port, customizers...)...)
}

type Server struct {
	App     *fiber.App
	HumaAPI interface{}
}
