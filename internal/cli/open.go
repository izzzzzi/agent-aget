package cli

import (
	"context"
	"path/filepath"
	"time"

	"github.com/izzzzzi/agent-aget/internal/browser"
	"github.com/izzzzzi/agent-aget/internal/cdp"
	"github.com/izzzzzi/agent-aget/internal/ids"
	sessionstore "github.com/izzzzzi/agent-aget/internal/session"
	"github.com/izzzzzi/agent-aget/internal/state"
	"github.com/spf13/cobra"
)

var waitForOpenPage = cdp.WaitForPageURL

var openPageReadyTimeout = 30 * time.Second

func newOpenCommand() *cobra.Command {
	var name string
	var headful bool
	var browserPath string
	cmd := &cobra.Command{
		Use:   "open URL",
		Short: "Open a URL in a managed browser session",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return writeInvalidArgs(cmd, "exactly one URL is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			resolved, err := browser.Resolve(browserPath)
			if err != nil {
				return writeError(cmd, "browser_not_found", err.Error(), nil)
			}

			sid, err := ids.NewSessionID()
			if err != nil {
				return writeError(cmd, "session_create_failed", err.Error(), nil)
			}
			port, err := browser.FindFreePort()
			if err != nil {
				return writeError(cmd, "browser_launch_failed", err.Error(), nil)
			}

			process, err := browser.Launch(browser.LaunchOptions{
				BinaryPath:  resolved.Path,
				BrowserName: resolved.Browser,
				URL:         url,
				UserDataDir: filepath.Join(state.ProfilesDir(), sid),
				Port:        port,
				Headless:    !headful,
			})
			if err != nil {
				return writeError(cmd, "browser_launch_failed", err.Error(), nil)
			}
			readyCtx, readyCancel := context.WithTimeout(context.Background(), openPageReadyTimeout)
			if err := waitForOpenPage(readyCtx, process.DebugURL, url); err != nil {
				readyCancel()
				_ = process.Stop()
				return writeError(cmd, "browser_launch_failed", err.Error(), map[string]any{"debug_url": process.DebugURL})
			}
			readyCancel()

			now := time.Now().UTC()
			record := sessionstore.Record{
				SID:        sid,
				Name:       name,
				URL:        url,
				BrowserPID: process.PID,
				DebugURL:   process.DebugURL,
				Headless:   !headful,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			if err := sessionstore.NewRegistry(state.SessionsDir()).Save(record); err != nil {
				_ = process.Stop()
				return writeError(cmd, "session_save_failed", err.Error(), map[string]any{"sid": sid})
			}

			return writeJSON(cmd, map[string]any{
				"ok":      true,
				"sid":     sid,
				"session": name,
				"browser": map[string]any{"name": resolved.Browser, "path": resolved.Path, "pid": process.PID, "debug_url": process.DebugURL, "headless": !headful},
				"page":    map[string]any{"url": url},
				"record":  record,
				"next_commands": map[string]string{
					"read":       "aget page read -s " + sid,
					"click":      "aget page click -s " + sid + " --selector CSS",
					"type":       "aget page type -s " + sid + " --selector CSS --text TEXT",
					"screenshot": "aget page screenshot -s " + sid,
					"close":      "aget session close -s " + sid,
				},
			})
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "session name")
	cmd.Flags().BoolVar(&headful, "headful", false, "run browser with a visible window")
	cmd.Flags().StringVar(&browserPath, "browser-path", "", "browser binary path")
	configureAgentHelp(cmd)
	return cmd
}
