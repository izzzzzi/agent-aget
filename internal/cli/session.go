package cli

import (
	"errors"

	sessionstore "github.com/izzzzzi/agent-aget/internal/session"
	"github.com/izzzzzi/agent-aget/internal/state"
	"github.com/spf13/cobra"
)

func newSessionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage browser sessions",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeInvalidArgs(cmd, "session subcommand required")
		},
	}
	configureAgentHelp(cmd)
	cmd.AddCommand(newSessionListCommand(), newSessionCloseCommand(), newSessionGCCommand())
	return cmd
}

func newSessionListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List browser sessions",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			records, err := sessionstore.NewRegistry(state.SessionsDir()).List()
			if err != nil {
				return writeError(cmd, "session_list_failed", err.Error(), nil)
			}
			if records == nil {
				records = []sessionstore.Record{}
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sessions": records})
		},
	}
	configureAgentHelp(cmd)
	return cmd
}

func newSessionCloseCommand() *cobra.Command {
	var sid string
	cmd := &cobra.Command{
		Use:   "close",
		Short: "Close a browser session",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sid == "" {
				return writeInvalidArgs(cmd, "session id required")
			}
			if err := sessionstore.NewRegistry(state.SessionsDir()).Delete(sid); err != nil {
				if errors.Is(err, sessionstore.ErrNotFound) {
					return writeError(cmd, "session_not_found", "session not found", map[string]any{"sid": sid})
				}
				return writeError(cmd, "session_close_failed", err.Error(), map[string]any{"sid": sid})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	configureAgentHelp(cmd)
	return cmd
}

func newSessionGCCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Garbage collect stale sessions",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeJSON(cmd, map[string]any{"ok": true, "removed": []string{}})
		},
	}
	configureAgentHelp(cmd)
	return cmd
}
