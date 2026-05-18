package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/izzzzzi/agent-aget/internal/cdp"
	"github.com/izzzzzi/agent-aget/internal/page"
	sessionstore "github.com/izzzzzi/agent-aget/internal/session"
	"github.com/izzzzzi/agent-aget/internal/state"
	"github.com/spf13/cobra"
)

var newChromeDPDriver = func(parent context.Context, debugURL string) (cdp.Driver, error) {
	return cdp.NewChromeDPDriver(parent, debugURL)
}

var pageCommandTimeout = 30 * time.Second

func newPageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "Inspect and control the active page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeInvalidArgs(cmd, "page subcommand required")
		},
	}
	disableHelpFlag(cmd)
	cmd.AddCommand(newPageReadCommand(), newPageClickCommand(), newPageTypeCommand(), newPageScreenshotCommand())
	return cmd
}

func newPageReadCommand() *cobra.Command {
	var sid string
	var limit int
	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read the current page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			driver, err := newChromeDPDriver(ctx, record.DebugURL)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}

			result, err := page.NewService(driver).Read(ctx, page.ReadOptions{Limit: limit})
			if err != nil {
				return writeError(cmd, "page_read_failed", err.Error(), map[string]any{"sid": sid})
			}
			result.OK = true
			return writeJSON(cmd, result)
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().IntVar(&limit, "limit", 80, "maximum number of lines")
	disableHelpFlag(cmd)
	return cmd
}

func newPageClickCommand() *cobra.Command {
	return newPageActionCommand("click", false)
}

func newPageTypeCommand() *cobra.Command {
	return newPageActionCommand("type", true)
}

func newPageActionCommand(name string, needsText bool) *cobra.Command {
	var sid string
	var selector string
	var text string
	cmd := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("%s elements on the current page", name),
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if selector == "" {
				return writeInvalidArgs(cmd, "selector required")
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			driver, err := newChromeDPDriver(ctx, record.DebugURL)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}

			svc := page.NewService(driver)
			if needsText {
				if err := svc.Type(ctx, selector, text); err != nil {
					return writeError(cmd, "page_action_failed", err.Error(), map[string]any{"sid": sid, "selector": selector})
				}
			} else if err := svc.Click(ctx, selector); err != nil {
				return writeError(cmd, "page_action_failed", err.Error(), map[string]any{"sid": sid, "selector": selector})
			}

			payload := map[string]any{"ok": true, "sid": sid, "selector": selector}
			if needsText {
				payload["text_len"] = len(text)
			}
			return writeJSON(cmd, payload)
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "css selector")
	if needsText {
		cmd.Flags().StringVar(&text, "text", "", "text to type")
	}
	disableHelpFlag(cmd)
	return cmd
}

func newPageScreenshotCommand() *cobra.Command {
	var sid string
	var path string
	cmd := &cobra.Command{
		Use:   "screenshot",
		Short: "Capture a page screenshot",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			driver, err := newChromeDPDriver(ctx, record.DebugURL)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}

			if path == "" {
				path = filepath.Join(state.ArtifactsDir(), sid+".png")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return writeError(cmd, "page_screenshot_failed", err.Error(), map[string]any{"sid": sid})
			}
			if err := page.NewService(driver).Screenshot(ctx, path); err != nil {
				return writeError(cmd, "page_screenshot_failed", err.Error(), map[string]any{"sid": sid, "path": path})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid, "path": path})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&path, "path", "", "artifact path")
	disableHelpFlag(cmd)
	return cmd
}

func pageOperationContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), pageCommandTimeout)
}

func lookupSession(cmd *cobra.Command, sid string) (sessionstore.Record, error) {
	if sid == "" {
		return sessionstore.Record{}, writeInvalidArgs(cmd, "session id required")
	}
	record, err := sessionstore.NewRegistry(state.SessionsDir()).Get(sid)
	if err != nil {
		if errors.Is(err, sessionstore.ErrNotFound) {
			return sessionstore.Record{}, writeError(cmd, "session_not_found", "session not found", map[string]any{"sid": sid})
		}
		return sessionstore.Record{}, writeError(cmd, "session_lookup_failed", err.Error(), map[string]any{"sid": sid})
	}
	return record, nil
}
