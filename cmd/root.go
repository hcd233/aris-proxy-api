// Package cmd 命令行工具
//
//	@update 2024-08-11 01:57:31
package cmd

import (
	"fmt"
	"os"

	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "",
	Short: "Aris Memory API",
	Long:  `Aris Memory API`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		msg := fmt.Sprintf("[Command] failed to execute command: %v", err)
		logger.Logger().Error(msg)
		os.Exit(1)
	}
}
