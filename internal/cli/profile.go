package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/izzzzzi/agent-aget/internal/browser"
	"github.com/izzzzzi/agent-aget/internal/cookies"
	profilestore "github.com/izzzzzi/agent-aget/internal/profile"
	sessionstore "github.com/izzzzzi/agent-aget/internal/session"
	"github.com/izzzzzi/agent-aget/internal/state"
	"github.com/spf13/cobra"
)

func newProfileCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage persistent browser profiles",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeInvalidArgs(cmd, "profile subcommand required")
		},
	}
	configureAgentHelp(cmd)
	cmd.AddCommand(
		newProfileCreateCommand(),
		newProfileListCommand(),
		newProfileShowCommand(),
		newProfileDeleteCommand(),
	)
	return cmd
}

func newProfileCreateCommand() *cobra.Command {
	var cookieInput string
	var browserPath string
	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a persistent browser profile, optionally with cookies",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return writeInvalidArgs(cmd, "exactly one profile name is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			store := profilestore.NewStore(state.ProfileMetaPath())

			if err := store.Create(name, cookieInput != ""); err != nil {
				if errors.Is(err, profilestore.ErrExists) {
					return writeError(cmd, "profile_exists", "profile already exists: "+name, map[string]any{"name": name})
				}
				return writeError(cmd, "profile_create_failed", err.Error(), map[string]any{"name": name})
			}

			if cookieInput != "" {
				if err := seedProfileCookies(name, cookieInput, browserPath); err != nil {
					_ = store.Delete(name)
					return writeError(cmd, "profile_cookie_failed", err.Error(), map[string]any{"name": name})
				}
			}

			return writeJSON(cmd, map[string]any{
				"ok":               true,
				"name":             name,
				"cookies_imported": cookieInput != "",
				"path":             state.ProfileUserDataDir(name),
			})
		},
	}
	cmd.Flags().StringVar(&cookieInput, "cookies", "", "cookies to seed (file path for Netscape format, or inline name=value pairs)")
	cmd.Flags().StringVar(&browserPath, "browser-path", "", "browser binary path")
	configureAgentHelp(cmd)
	return cmd
}

func newProfileListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List persistent browser profiles",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store := profilestore.NewStore(state.ProfileMetaPath())
			records, err := store.List()
			if err != nil {
				return writeError(cmd, "profile_list_failed", err.Error(), nil)
			}

			// Enrich with whether a session is currently using each profile
			sessions, _ := sessionstore.NewRegistry(state.SessionsDir()).List()
			activeProfiles := map[string]bool{}
			if sessions != nil {
				for _, s := range sessions {
					if s.Profile != "" {
						activeProfiles[s.Profile] = true
					}
				}
			}

			type profileEntry struct {
				Name            string    `json:"name"`
				CreatedAt       time.Time `json:"created_at"`
				CookiesImported bool      `json:"cookies_imported"`
				Active          bool      `json:"active"`
			}
			entries := make([]profileEntry, 0, len(records))
			for _, r := range records {
				entries = append(entries, profileEntry{
					Name:            r.Name,
					CreatedAt:       r.CreatedAt,
					CookiesImported: r.CookiesImported,
					Active:          activeProfiles[r.Name],
				})
			}
			if entries == nil {
				entries = []profileEntry{}
			}
			return writeJSON(cmd, map[string]any{"ok": true, "profiles": entries})
		},
	}
	configureAgentHelp(cmd)
	return cmd
}

func newProfileShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show NAME",
		Short: "Show details for a persistent browser profile",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return writeInvalidArgs(cmd, "exactly one profile name is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			store := profilestore.NewStore(state.ProfileMetaPath())
			rec, err := store.Get(name)
			if err != nil {
				if errors.Is(err, profilestore.ErrNotFound) {
					return writeError(cmd, "profile_not_found", "profile not found: "+name, map[string]any{"name": name})
				}
				return writeError(cmd, "profile_show_failed", err.Error(), map[string]any{"name": name})
			}

			sessions, _ := sessionstore.NewRegistry(state.SessionsDir()).List()
			active := false
			if sessions != nil {
				for _, s := range sessions {
					if s.Profile == name {
						active = true
						break
					}
				}
			}

			return writeJSON(cmd, map[string]any{
				"ok":               true,
				"name":             rec.Name,
				"created_at":       rec.CreatedAt,
				"cookies_imported": rec.CookiesImported,
				"active":           active,
				"path":             state.ProfileUserDataDir(name),
			})
		},
	}
	configureAgentHelp(cmd)
	return cmd
}

func newProfileDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a persistent browser profile and its data",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return writeInvalidArgs(cmd, "exactly one profile name is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			store := profilestore.NewStore(state.ProfileMetaPath())

			// Check if profile is in use
			sessions, _ := sessionstore.NewRegistry(state.SessionsDir()).List()
			if sessions != nil {
				for _, s := range sessions {
					if s.Profile == name {
						return writeError(cmd, "profile_in_use", "profile is currently in use by session "+s.SID, map[string]any{"name": name, "sid": s.SID})
					}
				}
			}

			if _, err := store.Get(name); err != nil {
				if errors.Is(err, profilestore.ErrNotFound) {
					return writeError(cmd, "profile_not_found", "profile not found: "+name, map[string]any{"name": name})
				}
				return writeError(cmd, "profile_delete_failed", err.Error(), map[string]any{"name": name})
			}

			if err := store.Delete(name); err != nil {
				return writeError(cmd, "profile_delete_failed", err.Error(), map[string]any{"name": name})
			}

			// Remove the profile directory
			profileDir := state.ProfileUserDataDir(name)
			if err := os.RemoveAll(profileDir); err != nil {
				return writeError(cmd, "profile_delete_failed", fmt.Sprintf("could not remove profile directory: %v", err), map[string]any{"name": name, "path": profileDir})
			}

			return writeJSON(cmd, map[string]any{"ok": true, "name": name})
		},
	}
	configureAgentHelp(cmd)
	return cmd
}

// seedProfileCookies launches a headless browser with the profile's user data dir,
// injects cookies, and closes — persisting them on disk for future sessions.
func seedProfileCookies(name, cookieInput, browserPath string) error {
	resolved, err := browser.Resolve(browserPath)
	if err != nil {
		return fmt.Errorf("browser resolve: %w", err)
	}

	parsedCookies, err := cookies.ParseCookies(cookieInput)
	if err != nil {
		return fmt.Errorf("parse cookies: %w", err)
	}
	if len(parsedCookies) == 0 {
		return fmt.Errorf("0 valid cookies parsed from input")
	}

	// Set domain to empty so cookies work for any URL during seed
	for _, c := range parsedCookies {
		if c.Domain == "" {
			c.Domain = ""                     // leave empty; Chromium accepts cookies without domain for any origin in headless
			c.URL = "https://" + name + ".ru" // hint URL for the cookie
		}
	}

	port, err := browser.FindFreePort()
	if err != nil {
		return fmt.Errorf("find port: %w", err)
	}

	userDataDir := state.ProfileUserDataDir(name)
	process, err := browser.Launch(browser.LaunchOptions{
		BinaryPath:  resolved.Path,
		BrowserName: resolved.Browser,
		UserDataDir: userDataDir,
		Port:        port,
		Headless:    true,
	})
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	defer func() { _ = process.Stop() }()

	// Wait for CDP port
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := waitForCDPPort(ctx, process.DebugURL); err != nil {
		return fmt.Errorf("cdp port: %w", err)
	}

	// Inject cookies via a temporary CDP connection
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), process.DebugURL)
	defer allocCancel()
	tabCtx, _ := chromedp.NewContext(allocCtx)

	var networkCookies []*network.CookieParam
	for _, c := range parsedCookies {
		cp := &network.CookieParam{
			Name:   c.Name,
			Value:  c.Value,
			Domain: c.Domain,
			Path:   "/",
		}
		if cp.Domain == "" {
			continue
		}
		networkCookies = append(networkCookies, cp)
	}

	if err := chromedp.Run(tabCtx, cookies.InjectCookiesAction(networkCookies)); err != nil {
		return fmt.Errorf("inject cookies: %w", err)
	}

	return nil
}
