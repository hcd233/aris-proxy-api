package main

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/tool/lintstatic"
	"github.com/spf13/cobra"
)

// newStaticCommand 构造 static 子命令，执行 golangci-lint 静态检查。
func newStaticCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "static [paths...]",
		Short: "Run static analysis (golangci-lint, includes govet/staticcheck)",
		Long:  `Run golangci-lint for standard static analysis across the project. govet and staticcheck are built-in linters, consistent with CI coverage.`,
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{constant.GoAllPackagesPattern}
			}
			result := lintstatic.Run(args)
			result.Log()
			return result.Err
		},
	}
}
