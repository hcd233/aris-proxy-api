package main

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/tool/lintconv"
	"github.com/spf13/cobra"
)

// newConvCommand 构造 conv 子命令，扫描项目自定义编码规范。
func newConvCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "conv [paths...]",
		Short: "Scan custom coding conventions",
		Long:  `Run the built-in AST-based convention checker to detect errors, logging issues, architecture violations, style problems, magic values, and test anti-patterns.`,
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{constant.GoAllPackagesPattern}
			}
			result := lintconv.Run(args)
			result.Log()
			if result.ErrorCount() > 0 {
				return ierr.New(ierr.ErrInternal, lintFailedMessage)
			}
			return nil
		},
	}
}
