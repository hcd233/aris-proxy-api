package main

import (
	"github.com/hcd233/aris-proxy-api/internal/client/status"
	"github.com/spf13/cobra"
)

func newStatusCommand() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show aris trace client status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return status.RunStatus(cmd.Context(), status.StatusOptions{
				In:   cmd.InOrStdin(),
				Out:  cmd.OutOrStdout(),
				JSON: jsonOutput,
			})
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output machine-readable JSON")
	return cmd
}
