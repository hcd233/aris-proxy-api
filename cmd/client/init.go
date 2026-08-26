package main

import (
	"github.com/hcd233/aris-proxy-api/internal/client/setup"
	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	var host string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize aris client (host + API key)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return setup.RunInit(cmd.Context(), setup.InitOptions{
				Host: host,
				In:   cmd.InOrStdin(),
				Out:  cmd.OutOrStdout(),
			})
		},
	}
	cmd.Flags().StringVar(&host, "host", "", "Aris server origin, e.g. https://aris.example.com")
	return cmd
}
