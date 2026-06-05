package cmd

import (
	"os"
	"runtime/debug"

	"github.com/hcd233/aris-proxy-api/internal/bootstrap"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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

		app := bootstrap.BuildFxApp(host, port)
		app.Run()
	},
}

func init() {
	serverCmd.AddCommand(startServerCmd)
	rootCmd.AddCommand(serverCmd)

	startServerCmd.Flags().StringP("host", "", "localhost", "监听的主机")
	startServerCmd.Flags().StringP("port", "p", "8080", "监听的端口")
}
