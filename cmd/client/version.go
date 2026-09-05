package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version 由 release 构建通过 -ldflags "-X main.version=<tag>" 注入，本地构建默认 dev。
var version = "dev"

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the aris client version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version)
			return err
		},
	}
}
