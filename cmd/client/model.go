package main

import (
	"github.com/hcd233/aris-proxy-api/internal/client/model"
	"github.com/spf13/cobra"
)

func newModelCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "model", Short: "Manage local agent model configuration"}
	cmd.AddCommand(newModelExportCommand())
	return cmd
}

func newModelExportCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Export aris models to a local agent harness (opencode/pi/codex/claude-code)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return model.RunExport(cmd.Context(), model.ExportOptions{
				In:  cmd.InOrStdin(),
				Out: cmd.OutOrStdout(),
			})
		},
	}
}
