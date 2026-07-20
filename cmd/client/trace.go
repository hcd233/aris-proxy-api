package main

import (
	"os"

	"github.com/hcd233/aris-proxy-api/internal/tracecli"
	"github.com/spf13/cobra"
)

func newTraceCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "trace", Short: "Configure and ingest agent traces"}
	cmd.AddCommand(newTraceInitCommand(), newTraceIngestCommand())
	return cmd
}

func newTraceInitCommand() *cobra.Command {
	var host string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Configure trace collection",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := tracecli.DefaultPaths()
			if err != nil {
				return err
			}
			commandPath, err := os.Executable()
			if err != nil {
				return err
			}
			runner := tracecli.InitRunner{
				Terminal: tracecli.NewTerminal(),
				Config:   tracecli.NewConfigStore(paths),
				Codex:    tracecli.NewCodexHookInstaller(paths),
				HTTP:     tracecli.NewHTTPClient(nil),
				Paths:    paths,
			}
			return runner.Run(cmd.Context(), tracecli.InitOptions{
				Host:        host,
				CommandPath: commandPath,
			})
		},
	}
	cmd.Flags().StringVar(&host, "host", "", "trace server origin")
	if err := cmd.MarkFlagRequired("host"); err != nil {
		panic(err)
	}
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
