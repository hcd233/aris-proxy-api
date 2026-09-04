package main

import (
	"sync"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/tool/lintconv"
	"github.com/hcd233/aris-proxy-api/internal/tool/lintstatic"
	"github.com/spf13/cobra"
)

// lintFailedMessage lint 检查失败提示
const lintFailedMessage = "lint checks failed"

// newRootCommand 构造 lint 根命令，默认执行全部检查（conv + static 并发）。
func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "lint [paths...]",
		Short:         "Aris linter",
		SilenceUsage:  true,
		SilenceErrors: true,
		Long:          `Run all lint checks (conv + static in parallel) against the given paths, default ./...`,
		Args:          cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{constant.GoAllPackagesPattern}
			}
			return runAll(args)
		},
	}
	root.AddCommand(newConvCommand())
	root.AddCommand(newStaticCommand())
	return root
}

// runAll 并发执行 conv 与 static 两阶段，按 conv → static 顺序输出结果。
func runAll(paths []string) error {
	var (
		wg           sync.WaitGroup
		convResult   lintconv.Result
		staticResult lintstatic.Result
	)
	wg.Go(func() { convResult = lintconv.Run(paths) })
	wg.Go(func() { staticResult = lintstatic.Run(paths) })
	wg.Wait()

	convResult.Log()
	staticResult.Log()
	if convResult.ErrorCount() > 0 || staticResult.Err != nil {
		return ierr.New(ierr.ErrInternal, lintFailedMessage)
	}
	return nil
}

func execute() error {
	return newRootCommand().Execute()
}
