package cli

import "github.com/spf13/cobra"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version metadata",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeJSON(cmd, map[string]any{
				"ok":      true,
				"version": version,
				"commit":  commit,
				"date":    date,
			})
		},
	}
	configureAgentHelp(cmd)
	return cmd
}
