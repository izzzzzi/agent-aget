package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/izzzzzi/agent-aget/internal/cdp"
	"github.com/izzzzzi/agent-aget/internal/page"
	sessionstore "github.com/izzzzzi/agent-aget/internal/session"
	"github.com/izzzzzi/agent-aget/internal/snapshot"
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
	configureAgentHelp(cmd)
	cmd.AddCommand(
		newPageReadCommand(),
		newPageSnapshotCommand(),
		newPageClickCommand(),
		newPageTypeCommand(),
		newPageFillCommand(),
		newPageSelectCommand(),
		newPagePressCommand(),
		newPageWaitCommand(),
		newPageScrollCommand(),
		newPageGetCommand(),
		newPageScreenshotCommand(),
		newPageIsCommand(),
		newPageJSCommand(),
		newPageCheckCommand(),
		newPageUncheckCommand(),
		newPageHoverCommand(),
		newPageFocusCommand(),
		newPageUploadCommand(),
		newPageDialogAcceptCommand(),
		newPageDialogDismissCommand(),
	)
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
	configureAgentHelp(cmd)
	return cmd
}

func newPageClickCommand() *cobra.Command {
	var sid string
	var selector string
	var ref string
	var force bool
	cmd := &cobra.Command{
		Use:   "click",
		Short: "Click an element on the current page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSelectorOrRef(cmd, selector, ref); err != nil {
				return err
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, true)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			target := page.ActionTarget{SID: sid, Selector: selector, Ref: ref}
			var clickErr error
			if force {
				clickErr = svc.ClickForceTarget(ctx, target)
			} else {
				clickErr = svc.ClickTarget(ctx, target)
			}
			if clickErr != nil {
				return writePageActionError(cmd, sid, selector, ref, clickErr)
			}

			payload := map[string]any{"ok": true, "sid": sid}
			if selector != "" {
				payload["selector"] = selector
			}
			if ref != "" {
				payload["ref"] = ref
			}
			if force {
				payload["force"] = true
			}
			return writeJSON(cmd, payload)
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "css selector")
	cmd.Flags().StringVar(&ref, "ref", "", "snapshot ref")
	cmd.Flags().BoolVar(&force, "force", false, "force click via CDP mouse events at element coordinates")
	configureAgentHelp(cmd)
	return cmd
}

func newPageTypeCommand() *cobra.Command {
	var sid string
	var selector string
	var ref string
	var text string
	cmd := &cobra.Command{
		Use:   "type",
		Short: "Type text into an element on the current page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSelectorOrRef(cmd, selector, ref); err != nil {
				return err
			}
			if !cmd.Flags().Changed("text") {
				return writeInvalidArgs(cmd, "text required")
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, ref != "")
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			target := page.ActionTarget{SID: sid, Selector: selector, Ref: ref}
			if err := svc.TypeTarget(ctx, target, text); err != nil {
				return writePageActionError(cmd, sid, selector, ref, err)
			}

			payload := map[string]any{"ok": true, "sid": sid, "text_len": len(text)}
			if selector != "" {
				payload["selector"] = selector
			}
			if ref != "" {
				payload["ref"] = ref
			}
			return writeJSON(cmd, payload)
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "css selector")
	cmd.Flags().StringVar(&ref, "ref", "", "snapshot ref")
	cmd.Flags().StringVar(&text, "text", "", "text to type")
	configureAgentHelp(cmd)
	return cmd
}

func newPageSnapshotCommand() *cobra.Command {
	var sid string
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Capture an agent-friendly page snapshot",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, true)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			result, err := svc.Snapshot(ctx, page.SnapshotOptions{SID: sid})
			if err != nil {
				return writeError(cmd, "page_snapshot_failed", err.Error(), map[string]any{"sid": sid})
			}
			return writeJSON(cmd, result)
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	configureAgentHelp(cmd)
	return cmd
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
	configureAgentHelp(cmd)
	return cmd
}

func newPageFillCommand() *cobra.Command {
	var sid string
	var selector string
	var ref string
	var text string
	cmd := &cobra.Command{
		Use:   "fill",
		Short: "Clear and fill an element on the current page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSelectorOrRef(cmd, selector, ref); err != nil {
				return err
			}
			if !cmd.Flags().Changed("text") {
				return writeInvalidArgs(cmd, "text required")
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, true)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			target := page.ActionTarget{SID: sid, Selector: selector, Ref: ref}
			if err := svc.Fill(ctx, page.FillOptions{Target: target, Text: text}); err != nil {
				return writePageActionError(cmd, sid, selector, ref, err)
			}
			payload := map[string]any{"ok": true, "sid": sid, "text_len": len(text)}
			if selector != "" {
				payload["selector"] = selector
			}
			if ref != "" {
				payload["ref"] = ref
			}
			return writeJSON(cmd, payload)
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "css selector")
	cmd.Flags().StringVar(&ref, "ref", "", "snapshot ref")
	cmd.Flags().StringVar(&text, "text", "", "text to fill")
	configureAgentHelp(cmd)
	return cmd
}

func newPagePressCommand() *cobra.Command {
	var sid string
	var key string
	cmd := &cobra.Command{
		Use:   "press",
		Short: "Press a key on the current page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if key == "" {
				return writeInvalidArgs(cmd, "key required")
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, false)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			if err := svc.Press(ctx, page.PressOptions{Key: key}); err != nil {
				return writeError(cmd, "page_action_failed", err.Error(), map[string]any{"sid": sid, "key": key})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid, "key": key})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&key, "key", "", "key to press")
	configureAgentHelp(cmd)
	return cmd
}

func newPageWaitCommand() *cobra.Command {
	var sid string
	var selector string
	var ref string
	var text string
	var url string
	var load string
	var appear string
	cmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait for a page condition",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if countNonEmpty(selector, ref, text, url, load, appear) != 1 {
				return writeInvalidArgs(cmd, "exactly one wait condition required")
			}
			if selector != "" && ref != "" {
				return writeInvalidArgs(cmd, "selector and ref are mutually exclusive")
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, ref != "" || appear != "")
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}

			if appear != "" {
				err = svc.WaitAppearTarget(ctx, page.ActionTarget{SID: sid, Selector: appear})
			} else {
				err = svc.Wait(ctx, page.WaitOptions{Target: page.ActionTarget{SID: sid, Selector: selector, Ref: ref}, Text: text, URL: url, Load: load})
			}
			if err != nil {
				return writeError(cmd, "page_wait_timeout", err.Error(), map[string]any{"sid": sid})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "css selector")
	cmd.Flags().StringVar(&ref, "ref", "", "snapshot ref")
	cmd.Flags().StringVar(&text, "text", "", "text to wait for")
	cmd.Flags().StringVar(&url, "url", "", "url substring or pattern")
	cmd.Flags().StringVar(&load, "load", "", "load state")
	cmd.Flags().StringVar(&appear, "appear", "", "css selector to wait for element to appear in DOM")
	configureAgentHelp(cmd)
	return cmd
}

func newPageScrollCommand() *cobra.Command {
	var sid string
	var direction string
	var pixels int
	cmd := &cobra.Command{
		Use:   "scroll",
		Short: "Scroll the current page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if direction == "" {
				return writeInvalidArgs(cmd, "direction required")
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, false)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			if err := svc.Scroll(ctx, page.ScrollOptions{Direction: direction, Pixels: pixels}); err != nil {
				return writeError(cmd, "page_action_failed", err.Error(), map[string]any{"sid": sid, "direction": direction})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid, "direction": direction, "pixels": pixels})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&direction, "direction", "", "scroll direction")
	cmd.Flags().IntVar(&pixels, "px", 800, "pixels to scroll")
	configureAgentHelp(cmd)
	return cmd
}

func newPageGetCommand() *cobra.Command {
	var sid string
	var selector string
	var ref string
	cmd := &cobra.Command{
		Use:   "get KIND",
		Short: "Get focused page data",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return writeInvalidArgs(cmd, "exactly one get kind is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			kind := args[0]
			if !validGetKind(kind) {
				return writeInvalidArgs(cmd, "unsupported get kind "+kind)
			}
			needsTarget := kind == "text" || kind == "html" || kind == "value"
			if needsTarget {
				if err := validateSelectorOrRef(cmd, selector, ref); err != nil {
					return err
				}
			} else if selector != "" || ref != "" {
				return writeInvalidArgs(cmd, "selector/ref only valid for text, html, or value")
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, ref != "")
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			value, err := svc.Get(ctx, page.GetOptions{Kind: kind, Target: page.ActionTarget{SID: sid, Selector: selector, Ref: ref}})
			if err != nil {
				return writePageActionError(cmd, sid, selector, ref, err)
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid, "kind": kind, "value": value})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "css selector")
	cmd.Flags().StringVar(&ref, "ref", "", "snapshot ref")
	configureAgentHelp(cmd)
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
			} else {
				resolved, err := filepath.Abs(path)
				if err != nil {
					return writeError(cmd, "page_screenshot_failed", "invalid path", map[string]any{"sid": sid})
				}
				if !isPathUnderSafeRoots(resolved, []string{os.TempDir(), "/tmp", state.ArtifactsDir(), state.SnapshotsDir()}) {
					return writeError(cmd, "page_screenshot_failed", "--path must be under /tmp or aget state directory", map[string]any{"sid": sid})
				}
				path = resolved
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
	configureAgentHelp(cmd)
	return cmd
}

func newPageSelectCommand() *cobra.Command {
	var sid string
	var selector string
	var ref string
	var value string
	cmd := &cobra.Command{
		Use:   "select",
		Short: "Select an option in a dropdown on the current page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSelectorOrRef(cmd, selector, ref); err != nil {
				return err
			}
			if value == "" {
				return writeInvalidArgs(cmd, "value required")
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, true)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			target := page.ActionTarget{SID: sid, Selector: selector, Ref: ref}
			if err := svc.Select(ctx, target, value); err != nil {
				return writePageActionError(cmd, sid, selector, ref, err)
			}

			payload := map[string]any{"ok": true, "sid": sid}
			if selector != "" {
				payload["selector"] = selector
			}
			if ref != "" {
				payload["ref"] = ref
			}
			return writeJSON(cmd, payload)
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "css selector")
	cmd.Flags().StringVar(&ref, "ref", "", "snapshot ref")
	cmd.Flags().StringVar(&value, "value", "", "option value or visible text")
	configureAgentHelp(cmd)
	return cmd
}

func newPageIsCommand() *cobra.Command {
	var sid string
	var selector string
	var ref string
	cmd := &cobra.Command{
		Use:   "is PROP",
		Short: "Check element state: visible, hidden, enabled, disabled, checked, editable, focused",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return writeInvalidArgs(cmd, "exactly one property is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			prop := args[0]
			if err := validateSelectorOrRef(cmd, selector, ref); err != nil {
				return err
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, true)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			target := page.ActionTarget{SID: sid, Selector: selector, Ref: ref}
			result, err := svc.Is(ctx, target, prop)
			if err != nil {
				return writePageActionError(cmd, sid, selector, ref, err)
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid, "prop": prop, "value": result})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "css selector")
	cmd.Flags().StringVar(&ref, "ref", "", "snapshot ref")
	configureAgentHelp(cmd)
	return cmd
}

func newPageJSCommand() *cobra.Command {
	var sid string
	var expr string
	var file string
	cmd := &cobra.Command{
		Use:   "js",
		Short: "Execute JavaScript on the current page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if expr == "" && file == "" {
				return writeInvalidArgs(cmd, "expr or file required")
			}
			if expr != "" && file != "" {
				return writeInvalidArgs(cmd, "expr and file are mutually exclusive")
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, false)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			expression := expr
			fromFile := file != ""
			if fromFile {
				resolved, err := filepath.Abs(file)
				if err != nil {
					return writeError(cmd, "page_action_failed", "invalid file path", map[string]any{"sid": sid})
				}
				safeRoots := []string{os.TempDir(), state.ArtifactsDir()}
				allowed := false
				for _, root := range safeRoots {
					rel, err := filepath.Rel(root, resolved)
					if err == nil && !strings.HasPrefix(rel, "..") {
						allowed = true
						break
					}
				}
				if !allowed {
					return writeError(cmd, "page_action_failed", "--file path must be under /tmp or aget artifacts directory", map[string]any{"sid": sid})
				}
				data, err := os.ReadFile(resolved)
				if err != nil {
					return writeError(cmd, "page_action_failed", "cannot read file", map[string]any{"sid": sid})
				}
				expression = string(data)
			}
			result, err := svc.Eval(ctx, expression)
			if err != nil {
				msg := err.Error()
				if fromFile {
					msg = "JavaScript evaluation failed"
				}
				return writeError(cmd, "page_action_failed", msg, map[string]any{"sid": sid})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid, "result": result})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&expr, "expr", "", "JavaScript expression")
	cmd.Flags().StringVar(&file, "file", "", "JavaScript file to execute")
	configureAgentHelp(cmd)
	return cmd
}

func newPageCheckCommand() *cobra.Command {
	var sid string
	var selector string
	var ref string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check a checkbox or radio button on the current page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSelectorOrRef(cmd, selector, ref); err != nil {
				return err
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, true)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			target := page.ActionTarget{SID: sid, Selector: selector, Ref: ref}
			if err := svc.Check(ctx, target); err != nil {
				return writePageActionError(cmd, sid, selector, ref, err)
			}

			payload := map[string]any{"ok": true, "sid": sid}
			if selector != "" {
				payload["selector"] = selector
			}
			if ref != "" {
				payload["ref"] = ref
			}
			return writeJSON(cmd, payload)
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "css selector")
	cmd.Flags().StringVar(&ref, "ref", "", "snapshot ref")
	configureAgentHelp(cmd)
	return cmd
}

func newPageUncheckCommand() *cobra.Command {
	var sid string
	var selector string
	var ref string
	cmd := &cobra.Command{
		Use:   "uncheck",
		Short: "Uncheck a checkbox on the current page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSelectorOrRef(cmd, selector, ref); err != nil {
				return err
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, true)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			target := page.ActionTarget{SID: sid, Selector: selector, Ref: ref}
			if err := svc.Uncheck(ctx, target); err != nil {
				return writePageActionError(cmd, sid, selector, ref, err)
			}

			payload := map[string]any{"ok": true, "sid": sid}
			if selector != "" {
				payload["selector"] = selector
			}
			if ref != "" {
				payload["ref"] = ref
			}
			return writeJSON(cmd, payload)
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "css selector")
	cmd.Flags().StringVar(&ref, "ref", "", "snapshot ref")
	configureAgentHelp(cmd)
	return cmd
}

func newPageHoverCommand() *cobra.Command {
	var sid string
	var selector string
	var ref string
	cmd := &cobra.Command{
		Use:   "hover",
		Short: "Hover over an element on the current page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSelectorOrRef(cmd, selector, ref); err != nil {
				return err
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, true)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			target := page.ActionTarget{SID: sid, Selector: selector, Ref: ref}
			if err := svc.Hover(ctx, target); err != nil {
				return writePageActionError(cmd, sid, selector, ref, err)
			}

			payload := map[string]any{"ok": true, "sid": sid}
			if selector != "" {
				payload["selector"] = selector
			}
			if ref != "" {
				payload["ref"] = ref
			}
			return writeJSON(cmd, payload)
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "css selector")
	cmd.Flags().StringVar(&ref, "ref", "", "snapshot ref")
	configureAgentHelp(cmd)
	return cmd
}

func newPageFocusCommand() *cobra.Command {
	var sid string
	var selector string
	var ref string
	cmd := &cobra.Command{
		Use:   "focus",
		Short: "Focus an element on the current page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSelectorOrRef(cmd, selector, ref); err != nil {
				return err
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, true)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			target := page.ActionTarget{SID: sid, Selector: selector, Ref: ref}
			if err := svc.Focus(ctx, target); err != nil {
				return writePageActionError(cmd, sid, selector, ref, err)
			}

			payload := map[string]any{"ok": true, "sid": sid}
			if selector != "" {
				payload["selector"] = selector
			}
			if ref != "" {
				payload["ref"] = ref
			}
			return writeJSON(cmd, payload)
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "css selector")
	cmd.Flags().StringVar(&ref, "ref", "", "snapshot ref")
	configureAgentHelp(cmd)
	return cmd
}

func newPageUploadCommand() *cobra.Command {
	var sid string
	var selector string
	var ref string
	var filePath string
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload files to a file input on the current page",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSelectorOrRef(cmd, selector, ref); err != nil {
				return err
			}
			if filePath == "" {
				return writeInvalidArgs(cmd, "file required")
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, true)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			target := page.ActionTarget{SID: sid, Selector: selector, Ref: ref}
			if err := svc.Upload(ctx, target, []string{filePath}); err != nil {
				return writePageActionError(cmd, sid, selector, ref, err)
			}

			payload := map[string]any{"ok": true, "sid": sid}
			if selector != "" {
				payload["selector"] = selector
			}
			if ref != "" {
				payload["ref"] = ref
			}
			return writeJSON(cmd, payload)
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "css selector")
	cmd.Flags().StringVar(&ref, "ref", "", "snapshot ref")
	cmd.Flags().StringVar(&filePath, "file", "", "path to file")
	configureAgentHelp(cmd)
	return cmd
}

func newPageDialogAcceptCommand() *cobra.Command {
	var sid string
	var text string
	cmd := &cobra.Command{
		Use:   "dialog-accept",
		Short: "Auto-accept the next browser dialog (alert/confirm/prompt)",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, false)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			if err := svc.DialogAccept(ctx, text); err != nil {
				return writeError(cmd, "page_action_failed", err.Error(), map[string]any{"sid": sid})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&text, "text", "", "prompt response text")
	configureAgentHelp(cmd)
	return cmd
}

func newPageDialogDismissCommand() *cobra.Command {
	var sid string
	cmd := &cobra.Command{
		Use:   "dialog-dismiss",
		Short: "Auto-dismiss the next browser dialog (alert/confirm/prompt)",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			ctx, cancel := pageOperationContext()
			defer cancel()

			svc, err := pageServiceForRecord(ctx, record, false)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			if err := svc.DialogDismiss(ctx); err != nil {
				return writeError(cmd, "page_action_failed", err.Error(), map[string]any{"sid": sid})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	configureAgentHelp(cmd)
	return cmd
}

func pageOperationContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), pageCommandTimeout)
}

func pageServiceForRecord(ctx context.Context, record sessionstore.Record, withRefs bool) (*page.Service, error) {
	driver, err := newChromeDPDriver(ctx, record.DebugURL)
	if err != nil {
		return nil, err
	}
	if withRefs {
		return page.NewServiceWithRefs(driver, snapshot.NewStore(state.SnapshotsDir())), nil
	}
	return page.NewService(driver), nil
}

func validateSelectorOrRef(cmd *cobra.Command, selector, ref string) error {
	if selector == "" && ref == "" {
		return writeInvalidArgs(cmd, "selector or ref required")
	}
	if selector != "" && ref != "" {
		return writeInvalidArgs(cmd, "selector and ref are mutually exclusive")
	}
	return nil
}

func writePageActionError(cmd *cobra.Command, sid, selector, ref string, err error) error {
	details := map[string]any{"sid": sid}
	if selector != "" {
		details["selector"] = selector
	}
	if ref != "" {
		details["ref"] = ref
	}
	if errors.Is(err, snapshot.ErrRefNotFound) || errors.Is(err, snapshot.ErrNotFound) || errors.Is(err, page.ErrRefNotFound) {
		details["hint"] = "run `aget page snapshot -s " + sid + "` again"
		return writeError(cmd, "ref_not_found", err.Error(), details)
	}
	return writeError(cmd, "page_action_failed", err.Error(), details)
}

func countNonEmpty(values ...string) int {
	count := 0
	for _, value := range values {
		if value != "" {
			count++
		}
	}
	return count
}

func isPathUnderSafeRoots(path string, roots []string) bool {
	for _, root := range roots {
		resolvedRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			resolvedRoot = root
		}
		// Try resolving symlinks on the path's parent directory
		dir := filepath.Dir(path)
		resolvedDir, err := filepath.EvalSymlinks(dir)
		if err == nil {
			path = filepath.Join(resolvedDir, filepath.Base(path))
		}
		// Clean both for comparison
		resolvedRoot = filepath.Clean(resolvedRoot) + string(filepath.Separator)
		cleanPath := filepath.Clean(path) + string(filepath.Separator)
		if strings.HasPrefix(cleanPath, resolvedRoot) {
			return true
		}
	}
	return false
}

func validGetKind(kind string) bool {
	switch kind {
	case "url", "title", "text", "html", "value":
		return true
	default:
		return false
	}
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
