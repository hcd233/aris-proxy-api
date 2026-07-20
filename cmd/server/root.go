// Package main 服务端命令行入口。
package main

import (
	"os"

	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "",
	Short: "Aris Memory API",
	Long:  `Aris Memory API`,
}

func execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.Logger().Error("[Command] Failed to execute command", zap.Error(err))
		os.Exit(1)
	}
}
