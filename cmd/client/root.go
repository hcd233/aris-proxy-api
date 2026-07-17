package main

import "github.com/spf13/cobra"

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "aris",
		Short:         "Aris client",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newTraceCommand())
	return root
}

func execute() error {
	return newRootCommand().Execute()
}
