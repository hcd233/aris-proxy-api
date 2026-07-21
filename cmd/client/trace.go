package main

import (
	"github.com/hcd233/aris-proxy-api/internal/tracecli"
	"github.com/spf13/cobra"
)

func newTraceCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "trace", Short: "Ingest agent traces"}
	cmd.AddCommand(newTraceIngestCommand())
	return cmd
}

func newTraceIngestCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ingest",
		Short: "Ingest one agent hook event",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return tracecli.RunIngestCommand(cmd.Context(), tracecli.IngestCommandOptions{
				In:  cmd.InOrStdin(),
				Out: cmd.OutOrStdout(),
			})
		},
	}
}
