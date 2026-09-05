package main

import "github.com/spf13/cobra"

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
	return root
}

func execute() error {
	return newRootCommand().Execute()
}
