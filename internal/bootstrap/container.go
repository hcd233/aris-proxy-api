package bootstrap

import (
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/api"
	"github.com/hcd233/aris-proxy-api/internal/bootstrap/modules"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
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
			registerMiddlewares,
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

type middlewareParams struct {
	fx.In

	App                  *fiber.App
	InflightTracker      *inflight.Tracker
	Cache                *redis.Client
	PrometheusMiddleware fiber.Handler
}

// registerMiddlewares 注册中间件链，顺序：Recover → Inflight → Guard → Fgprof → Prometheus → CORS → Compress → Trace → Log
func registerMiddlewares(params middlewareParams) {
	params.App.Use(constant.RoutePathMetrics, params.PrometheusMiddleware)

	params.App.Use(
		middleware.RecoverMiddleware(),
		middleware.InflightMiddleware(params.InflightTracker),
		middleware.GuardMiddleware(params.Cache, middleware.GuardConfig{
			StrikeThreshold: constant.GuardStrikeThreshold,
			StrikeWindow:    constant.GuardStrikeWindow,
			BanDuration:     constant.GuardBanDuration,
			IgnoredPaths: []string{
				constant.RoutePathRoot,
				constant.RoutePathHealth,
				constant.RoutePathReady,
				constant.RoutePathSSEHealth,
				constant.RoutePathFavicon,
				constant.RoutePathRobots,
				constant.RoutePathAppleTouchIcon,
				constant.RoutePathAppleTouchIconPrecomposed,
				constant.RoutePathWellKnownSecurity,
			},
		}),
		middleware.FgprofMiddleware(),
		middleware.CORSMiddleware(),
		middleware.CompressMiddleware(),
		middleware.TraceMiddleware(),
		middleware.LocaleMiddleware(),
		middleware.LogMiddleware(middleware.LogMiddlewareConfig{
			SamplingRules: []middleware.LogSamplingRule{
				{Path: constant.RoutePathHealth, Interval: constant.LogMiddlewareSamplingInterval},
				{Path: constant.RoutePathReady, Interval: constant.LogMiddlewareSamplingInterval},
				{Path: constant.RoutePathSSEHealth, Interval: constant.LogMiddlewareSamplingInterval},
			},
		}),
	)
}
