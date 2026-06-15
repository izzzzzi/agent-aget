package cli

import (
	"github.com/izzzzzi/agent-aget/internal/inspect"
	"github.com/spf13/cobra"
)

func newInspectCommand() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Start a real-time session inspector dashboard (HTTP + SSE)",
		Long: `Starts an HTTP server with a web dashboard that shows active sessions
and their snapshot state in real time via Server-Sent Events.

Useful for debugging agent workflows, inspecting snapshots, and
monitoring browser sessions without the CDP debugging port.

The dashboard is served at http://localhost:<port> by default.

Example:
  aget inspect
  aget inspect --port 9090
`,
		Args: noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return inspect.New(port).ListenAndServe()
		},
	}
	cmd.Flags().IntVar(&port, "port", 9223, "HTTP server port")
	configureAgentHelp(cmd)
	return cmd
}
