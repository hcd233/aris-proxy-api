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
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/httpclient"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"go.uber.org/zap"

	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
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

		database.InitDatabase()
		cache.InitCache()
		httpclient.InitHTTPClient()
		pool.InitPoolManager()
		cron.InitCronJobs()

		server, err := bootstrap.BuildServer()
		if err != nil {
			logger.Logger().Error("[Server] Build server failed", zap.Error(err))
			os.Exit(1)
		}
		app := server.App

		app.Use(
			middleware.RecoverMiddleware(),
			middleware.GuardMiddleware(middleware.GuardConfig{
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
			gracefulShutdown(app)
		}
	},
}

// gracefulShutdown 按序执行优雅关闭流程
//
// 关闭顺序：HTTP 服务 → 日志同步 → 协程池 → 定时任务 → 数据库 → Redis
//
//	@param app *fiber.App
//	@author centonhuang
//	@update 2026-04-04 10:00:00
func gracefulShutdown(app *fiber.App) {
	ctx, cancel := context.WithTimeout(context.Background(), constant.ShutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)

		// Step 1: 停止接受新 HTTP 请求，等待现有请求完成
		logger.Logger().Info("[Server] Step 1/6: Shutting down HTTP server...")
		if err := app.ShutdownWithTimeout(constant.FiberShutdownTimeout); err != nil {
			logger.Logger().Error("[Server] HTTP server shutdown error", zap.Error(err))
		}

		// Step 2: 同步日志（flush CLS 等外部日志缓冲）
		logger.Logger().Info("[Server] Step 2/6: Syncing logger...")
		if err := logger.Logger().Sync(); err != nil {
			logger.Logger().Error("[Server] Logger sync error", zap.Error(err))
		}

		// Step 3: 停止协程池（等待所有排队的消息存储任务完成）
		logger.Logger().Info("[Server] Step 3/6: Stopping pool manager...")
		pool.StopPoolManager()

		// Step 4: 停止定时任务
		logger.Logger().Info("[Server] Step 4/6: Stopping cron jobs...")
		cron.StopCronJobs()

		// Step 5: 关闭数据库连接池
		logger.Logger().Info("[Server] Step 5/6: Closing database connection...")
		if err := database.CloseDatabase(); err != nil {
			logger.Logger().Error("[Server] Database close error", zap.Error(err))
		}

		// Step 6: 关闭 Redis 连接
		logger.Logger().Info("[Server] Step 6/6: Closing Redis connection...")
		if err := cache.CloseCache(); err != nil {
			logger.Logger().Error("[Server] Redis close error", zap.Error(err))
		}

		logger.Logger().Info("[Server] Graceful shutdown completed")
	}()

	select {
	case <-done:
		// 正常关闭完成
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
