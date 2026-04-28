package cmd

import (
	"os"

	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/tool/lintconv"
	"github.com/hcd233/aris-proxy-api/internal/tool/lintstatic"
	"github.com/spf13/cobra"
)

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Lint Command Group",
	Long:  `Lint command group for code quality checks, including custom convention scanning.`,
}

var convLintCmd = &cobra.Command{
	Use:   "conv",
	Short: "Scan custom coding conventions",
	Long:  `Run the built-in AST-based convention checker to detect errors, logging issues, architecture violations, style problems, magic values, and test anti-patterns.`,
	Run: func(_ *cobra.Command, args []string) {
		result := lintconv.Run(args)
		result.Log()
		if result.ErrorCount() > 0 {
			os.Exit(1)
		}
	},
}

var staticLintCmd = &cobra.Command{
	Use:   "static",
	Short: "Run static analysis (go vet + staticcheck)",
	Long:  `Run go vet and staticcheck (if installed) for standard static analysis across the project.`,
	Run: func(_ *cobra.Command, args []string) {
		result := lintstatic.Run(args)
		result.Log(logger.Logger())
		if result.Err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	lintCmd.AddCommand(convLintCmd)
	lintCmd.AddCommand(staticLintCmd)
	rootCmd.AddCommand(lintCmd)
}
