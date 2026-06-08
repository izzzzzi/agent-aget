package cli

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/izzzzzi/agent-aget/internal/browser"
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
			closed, err := closeSession(sid)
			if err != nil {
				if errors.Is(err, sessionstore.ErrNotFound) {
					return writeError(cmd, "session_not_found", "session not found", map[string]any{"sid": sid})
				}
				return writeError(cmd, "session_close_failed", err.Error(), map[string]any{"sid": sid})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid, "closed": closed})
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
			removed, err := gcSessions()
			if err != nil {
				return writeError(cmd, "session_gc_failed", err.Error(), nil)
			}
			return writeJSON(cmd, map[string]any{"ok": true, "removed": removed})
		},
	}
	configureAgentHelp(cmd)
	return cmd
}

func closeSession(sid string) (map[string]any, error) {
	registry := sessionstore.NewRegistry(state.SessionsDir())
	record, err := registry.Get(sid)
	if err != nil {
		return nil, err
	}

	processErr := browser.StopPID(record.BrowserPID)
	if processErr != nil && !errors.Is(processErr, os.ErrProcessDone) {
		return nil, processErr
	}

	if err := registry.Delete(sid); err != nil {
		return nil, err
	}

	removedUserDataDir := false
	if record.Profile == "" {
		if err := os.RemoveAll(filepath.Join(state.ProfilesDir(), sid)); err != nil {
			return nil, err
		}
		removedUserDataDir = true
	}

	return map[string]any{
		"browser_pid":           record.BrowserPID,
		"process_already_gone":  errors.Is(processErr, os.ErrProcessDone),
		"removed_user_data_dir": removedUserDataDir,
	}, nil
}

func gcSessions() ([]string, error) {
	registry := sessionstore.NewRegistry(state.SessionsDir())
	records, err := registry.List()
	if err != nil {
		return nil, err
	}

	removed := make([]string, 0)
	for _, record := range records {
		if record.BrowserPID > 0 && processExists(record.BrowserPID) {
			continue
		}
		if err := registry.Delete(record.SID); err != nil && !errors.Is(err, sessionstore.ErrNotFound) {
			return nil, err
		}
		if record.Profile == "" {
			if err := os.RemoveAll(filepath.Join(state.ProfilesDir(), record.SID)); err != nil {
				return nil, err
			}
		}
		removed = append(removed, record.SID)
	}
	return removed, nil
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
