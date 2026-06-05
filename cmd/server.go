package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/hcd233/aris-proxy-api/internal/bootstrap"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"go.uber.org/zap"

	"github.com/gofiber/fiber/v3"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server Command Group",
	Long:  `Server command group for starting and managing the API server`,
}

var startServerCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the API server",
	Long:  `Start and run the API server, listening on the specified host and port`,
	Run: func(cmd *cobra.Command, _ []string) {
		defer func() {
			if r := recover(); r != nil {
				logger.Logger().Error("[Server] Start server panic", zap.Any("error", r), zap.ByteString("stack", debug.Stack()))
				os.Exit(1)
			}
		}()
		host, port := lo.Must1(cmd.Flags().GetString("host")), lo.Must1(cmd.Flags().GetString("port"))

		logger.Logger().Info("[Server] Environment",
			zap.String("env", config.Env),
			zap.Duration("readTimeout", config.ReadTimeout),
			zap.Duration("writeTimeout", config.WriteTimeout),
			zap.Int("maxHeaderBytes", config.MaxHeaderBytes),
			zap.Int("poolStoreWorkers", config.Pool.Store.Workers),
			zap.Int("poolStoreQueueSize", config.Pool.Store.QueueSize),
			zap.Int("poolAgentWorkers", config.Pool.Agent.Workers),
			zap.Int("poolAgentQueueSize", config.Pool.Agent.QueueSize),
			zap.Int("sqlBatchSize", config.SQLBatchSize),
		)

		infra := bootstrap.InitInfrastructure()
		server, err := bootstrap.BuildServer(infra)
		if err != nil {
			logger.Logger().Error("[Server] Build server failed", zap.Error(err))
			os.Exit(1)
		}
		app := server.App

		app.Use(
			middleware.RecoverMiddleware(),
			middleware.InflightMiddleware(server.Tracker),
			middleware.GuardMiddleware(infra.Cache, middleware.GuardConfig{
				StrikeThreshold: constant.GuardStrikeThreshold,
				StrikeWindow:    constant.GuardStrikeWindow,
				BanDuration:     constant.GuardBanDuration,
				IgnoredPaths: []string{
					constant.RoutePathRoot,
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
			middleware.LogMiddleware(middleware.LogMiddlewareConfig{
				SamplingRules: []middleware.LogSamplingRule{
					{Path: constant.RoutePathHealth, Interval: constant.LogMiddlewareSamplingInterval},
					{Path: constant.RoutePathReady, Interval: constant.LogMiddlewareSamplingInterval},
					{Path: constant.RoutePathSSEHealth, Interval: constant.LogMiddlewareSamplingInterval},
				},
			}),
		)

		if err := bootstrap.RegisterRoutes(server); err != nil {
			logger.Logger().Error("[Server] Register routes failed", zap.Error(err))
			os.Exit(1)
		}

		// 启动 HTTP 服务（在 goroutine 中运行，主 goroutine 用于信号监听）
		listenAddr := fmt.Sprintf("%s:%s", host, port)
		listenErr := make(chan error, 1)
		go func() {
			listenErr <- app.Listen(listenAddr)
		}()

		// 监听关闭信号（SIGINT/SIGTERM）
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		select {
		case err := <-listenErr:
			if err != nil {
				logger.Logger().Error("[Server] HTTP server exited unexpectedly", zap.Error(err))
				os.Exit(1)
			}
		case sig := <-quit:
			logger.Logger().Info("[Server] Received shutdown signal, starting graceful shutdown...", zap.String("signal", sig.String()))
			gracefulShutdown(app, infra, server.Tracker)
		}
	},
}

func gracefulShutdown(app *fiber.App, infra *bootstrap.Infrastructure, tracker *inflight.Tracker) {
	ctx, cancel := context.WithTimeout(context.Background(), constant.ShutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)

		// Step 1: 停止定时任务（等待当前 job 完成）
		logger.Logger().Info("[Server] Step 1/8: Stopping cron jobs...")
		cron.StopCronJobs()

		// Step 2: 停止协程池（等待排队任务完成）
		logger.Logger().Info("[Server] Step 2/8: Stopping pool manager...")
		pool.StopPoolManager()

		// Step 3: 进入 draining 状态，拒绝新请求
		logger.Logger().Info("[Server] Step 3/8: Entering draining state...")

		// Step 4: 等待进行中请求完成（含 SSE 流）
		logger.Logger().Info("[Server] Step 4/8: Waiting for inflight requests to complete...")
		tracker.Drain(constant.InflightDrainTimeout)

		// Step 5: 关闭 HTTP Server
		logger.Logger().Info("[Server] Step 5/8: Shutting down HTTP server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), constant.FiberShutdownTimeout)
		defer shutdownCancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			logger.Logger().Error("[Server] HTTP server shutdown error", zap.Error(err))
		}

		// Step 6: 同步日志（flush CLS 等外部日志缓冲）
		logger.Logger().Info("[Server] Step 6/8: Syncing logger...")
		if err := logger.Logger().Sync(); err != nil {
			logger.Logger().Error("[Server] Logger sync error", zap.Error(err))
		}

		// Step 7: 关闭数据库连接池
		logger.Logger().Info("[Server] Step 7/8: Closing database connection...")
		if err := database.CloseDatabase(infra.DB); err != nil {
			logger.Logger().Error("[Server] Database close error", zap.Error(err))
		}

		// Step 8: 关闭 Redis 连接
		logger.Logger().Info("[Server] Step 8/8: Closing Redis connection...")
		if err := cache.CloseCache(infra.Cache); err != nil {
			logger.Logger().Error("[Server] Redis close error", zap.Error(err))
		}

		logger.Logger().Info("[Server] Graceful shutdown completed")
	}()

	select {
	case <-done:
	case <-ctx.Done():
		logger.Logger().Error("[Server] Graceful shutdown timed out, forcing exit", zap.Duration("timeout", constant.ShutdownTimeout))
	}
}

func init() {
	serverCmd.AddCommand(startServerCmd)
	rootCmd.AddCommand(serverCmd)

	startServerCmd.Flags().StringP("host", "", "localhost", "监听的主机")
	startServerCmd.Flags().StringP("port", "p", "8080", "监听的端口")
}
