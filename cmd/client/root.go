package main

import (
	"context"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/hcd233/aris-proxy-api/internal/client/update"
	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "aris",
		Short:         "Aris client",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// 与 version 子命令输出保持一致：裸版本字符串。
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(newInitCommand())
	root.AddCommand(newStatusCommand())
	root.AddCommand(newTraceCommand())
	root.AddCommand(newModelCommand())
	root.AddCommand(newVersionCommand())
	root.AddCommand(newUpdateCommand())
	return root
}

// execute 执行整棵命令树；ctx 为入口创建的根 context，args 为命令行参数（不含程序名）
func execute(ctx context.Context, args []string) error {
	root := newRootCommand()
	// cobra 的 Context() 在 Execute() 之前为 nil，根 context 由入口显式注入，避免下游拿 nil 建派生 context
	root.SetContext(ctx)
	finishCheck := startUpdateCheck(ctx, root, args)
	err := root.Execute()
	finishCheck()
	return err
}

// startUpdateCheck 对符合条件的交互式命令启动一次后台更新检查；返回等待并打印提示的函数
func startUpdateCheck(ctx context.Context, root *cobra.Command, args []string) func() {
	if !update.ShouldCheck(version, isStderrTerminal()) || !isUpdateCheckCommand(root, args) {
		return func() {}
	}
	return update.StartCheck(ctx, update.CheckOptions{Current: version, Out: os.Stderr})
}

// isUpdateCheckCommand 判断本次调用的命令是否参与更新检查（裸 aris 与 --version 不参与）
func isUpdateCheckCommand(root *cobra.Command, args []string) bool {
	target, _, err := root.Find(args)
	if err != nil || target == root {
		return false
	}
	return update.ShouldCheckCommand(strings.Fields(target.CommandPath())[1:])
}

// isStderrTerminal 判断 stderr 是否为交互终端（非交互场景不发起网络检查）
func isStderrTerminal() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}
