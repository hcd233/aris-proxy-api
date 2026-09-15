package main

import (
	"github.com/hcd233/aris-proxy-api/internal/client/update"
	"github.com/spf13/cobra"
)

func newUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update aris to the latest release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return update.Run(cmd.Context(), update.Options{
				Current: version,
				In:      cmd.InOrStdin(),
				Out:     cmd.OutOrStdout(),
			})
		},
	}
}
