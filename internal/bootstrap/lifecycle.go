package bootstrap

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v3"
	blockedapp "github.com/hcd233/aris-proxy-api/internal/application/blocked"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type lifecycleParams struct {
	fx.In

	Lifecycle       fx.Lifecycle
	App             *fiber.App
	DB              *gorm.DB
	Cache           *redis.Client
	PoolManager     *pool.PoolManager
	InflightTracker *inflight.Tracker
	MetricsFlusher  *metrics.Flusher
	CronEntries     []cron.Cron
	CronManager     *cron.CronManager
	BlockedService  *blockedapp.BlockedService
	ListenHost      string `name:"listenHost"`
	ListenPort      string `name:"listenPort"`
}

func registerLifecycleHooks(params lifecycleParams) {
	// OnStop hooks run in REVERSE order of registration.
	// Desired stop order: Cron → Inflight → Pool → HTTP → Logger → DB → Redis
	// (Inflight 先于 Pool：drain 期间协程池必须存活，被截断流的计量/存储才能落库)
	// So register in reverse: Redis → DB → Logger → HTTP → Pool → Inflight → Cron

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error { return nil },
		OnStop: func(ctx context.Context) error {
			return cache.CloseCache(params.Cache)
		},
	})

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error { return nil },
		OnStop: func(ctx context.Context) error {
			return database.CloseDatabase(params.DB)
		},
	})

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			_ = params.BlockedService.Rebuild(ctx) //nolint:errcheck // 启动失败由 syncLoop 兜底
			params.BlockedService.StartSync(ctx)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			params.BlockedService.StopSync()
			return nil
		},
	})

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error { return nil },
		OnStop: func(ctx context.Context) error {
			return logger.Logger().Sync()
		},
	})

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			listenAddr := fmt.Sprintf(constant.HostPortTemplate, params.ListenHost, params.ListenPort)
			go func() {
				if listenErr := params.App.Listen(listenAddr); listenErr != nil {
					logger.Logger().Error("[Server] HTTP server listen error", zap.Error(listenErr))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), constant.FiberShutdownTimeout)
			defer cancel()
			if err := params.App.ShutdownWithContext(shutdownCtx); err != nil {
				logger.Logger().Error("[Server] HTTP server shutdown error", zap.Error(err))
			}
			return nil
		},
	})

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			params.MetricsFlusher.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			params.MetricsFlusher.Stop()
			return nil
		},
	})

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error { return nil },
		OnStop: func(ctx context.Context) error {
			poolCtx, cancel := context.WithTimeout(context.Background(), constant.PoolStopTimeout)
			defer cancel()
			if err := params.PoolManager.StopWithContext(poolCtx); err != nil {
				logger.Logger().Warn("[Server] Pool stop error", zap.Error(err))
			}
			return nil
		},
	})

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error { return nil },
		OnStop: func(ctx context.Context) error {
			params.InflightTracker.Drain(constant.InflightDrainSoftTimeout, constant.InflightDrainHardTimeout)
			return nil
		},
	})

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return nil
		},
		OnStop: func(ctx context.Context) error {
			cronCtx, cancel := context.WithTimeout(context.Background(), constant.CronStopTimeout)
			defer cancel()
			if err := params.CronManager.StopAll(cronCtx); err != nil {
				logger.Logger().Warn("[Server] Cron stop error", zap.Error(err))
			}
			return nil
		},
	})
}
