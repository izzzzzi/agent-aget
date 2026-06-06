package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/izzzzzi/agent-aget/internal/browser"
	"github.com/izzzzzi/agent-aget/internal/cdp"
	"github.com/izzzzzi/agent-aget/internal/cookies"
	"github.com/izzzzzi/agent-aget/internal/ids"
	profilestore "github.com/izzzzzi/agent-aget/internal/profile"
	sessionstore "github.com/izzzzzi/agent-aget/internal/session"
	"github.com/izzzzzi/agent-aget/internal/state"
	"github.com/spf13/cobra"
)

var waitForOpenPage = cdp.WaitForPageURL

var openPageReadyTimeout = 30 * time.Second
var cdpReadyTimeout = 10 * time.Second

type devicePreset struct {
	Width     int
	Height    int
	Scale     float64
	UserAgent string
	Touch     bool
}

var devicePresets = map[string]devicePreset{
	"mobile": {
		Width:     375,
		Height:    812,
		Scale:     3,
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		Touch:     true,
	},
	"tablet": {
		Width:     768,
		Height:    1024,
		Scale:     2,
		UserAgent: "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		Touch:     true,
	},
	"desktop": {
		Width:     1280,
		Height:    720,
		Scale:     1,
		UserAgent: "",
		Touch:     false,
	},
}

func deviceEmulationAction(preset devicePreset) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		// Apply viewport with device scale factor
		if err := chromedp.EmulateViewport(int64(preset.Width), int64(preset.Height), chromedp.EmulateScale(preset.Scale)).Do(ctx); err != nil {
			return err
		}
		// Override user agent
		if preset.UserAgent != "" {
			if err := emulation.SetUserAgentOverride(preset.UserAgent).Do(ctx); err != nil {
				return err
			}
		}
		// Enable touch emulation
		if preset.Touch {
			touch := true
			maxPoints := int64(5)
			if err := emulation.SetTouchEmulationEnabled(touch).WithMaxTouchPoints(maxPoints).Do(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

func newOpenCommand() *cobra.Command {
	var name string
	var headful bool
	var browserPath string
	var cookieInput string
	var profileName string
	var deviceName string
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

			// Validate device preset
			var device *devicePreset
			if deviceName != "" {
				d, ok := devicePresets[deviceName]
				if !ok {
					return writeInvalidArgs(cmd, "unsupported device: "+deviceName+" (use mobile, tablet, or desktop)")
				}
				device = &d
			}

			sid, err := ids.NewSessionID()
			if err != nil {
				return writeError(cmd, "session_create_failed", err.Error(), nil)
			}
			port, err := browser.FindFreePort()
			if err != nil {
				return writeError(cmd, "browser_launch_failed", err.Error(), nil)
			}

			// Determine user data directory: profile or session-scoped
			userDataDir := filepath.Join(state.ProfilesDir(), sid)
			if profileName != "" {
				store := profilestore.NewStore(state.ProfileMetaPath())
				if _, err := store.Get(profileName); err != nil {
					if errors.Is(err, profilestore.ErrNotFound) {
						return writeError(cmd, "profile_not_found", "profile not found: "+profileName, map[string]any{"profile": profileName})
					}
					return writeError(cmd, "profile_lookup_failed", err.Error(), map[string]any{"profile": profileName})
				}

				// Check profile not already in use
				sessions, _ := sessionstore.NewRegistry(state.SessionsDir()).List()
				if sessions != nil {
					for _, s := range sessions {
						if s.Profile == profileName {
							return writeError(cmd, "profile_in_use", "profile is already in use by session "+s.SID, map[string]any{"profile": profileName, "sid": s.SID})
						}
					}
				}

				userDataDir = state.ProfileUserDataDir(profileName)
			}

			// Parse cookies early so we fail fast before launching the browser.
			// When --profile is set, cookies are already seeded in the profile;
			// --cookies is ignored in that case.
			var parsedCookies []*network.CookieParam
			if cookieInput != "" && profileName == "" {
				parsedCookies, err = cookies.ParseCookies(cookieInput)
				if err != nil {
					return writeError(cmd, "cookie_parse_failed", err.Error(), nil)
				}
				if len(parsedCookies) == 0 {
					return writeError(cmd, "cookie_parse_failed", "0 valid cookies parsed from input", nil)
				}
				cookies.ApplyDomain(parsedCookies, url)
			}

			var windowWidth, windowHeight int
			if device != nil {
				windowWidth = device.Width
				windowHeight = device.Height
			}

			process, err := browser.Launch(browser.LaunchOptions{
				BinaryPath:   resolved.Path,
				BrowserName:  resolved.Browser,
				URL:          url,
				UserDataDir:  userDataDir,
				Port:         port,
				Headless:     !headful,
				WindowWidth:  windowWidth,
				WindowHeight: windowHeight,
			})
			if err != nil {
				return writeError(cmd, "browser_launch_failed", err.Error(), nil)
			}

			// Wait for the page to be ready first (browser auto-navigates to URL)
			readyCtx, readyCancel := context.WithTimeout(context.Background(), openPageReadyTimeout)
			if err := waitForOpenPage(readyCtx, process.DebugURL, url); err != nil {
				readyCancel()
				_ = process.Stop()
				return writeError(cmd, "browser_launch_failed", err.Error(), map[string]any{"debug_url": process.DebugURL})
			}
			readyCancel()

			// Apply device emulation (viewport scale, user-agent, touch) and reload
			if device != nil {
				if err := emulateDeviceAndReload(process.DebugURL, *device); err != nil {
					_ = process.Stop()
					return writeError(cmd, "device_emulation_failed", err.Error(), nil)
				}
			}

			// If cookies provided, inject after page load and reload so the page
			// picks up the cookies on subsequent requests.
			if cookieInput != "" {
				if err := injectCookiesAndReload(process.DebugURL, url, parsedCookies); err != nil {
					_ = process.Stop()
					return writeError(cmd, "cookie_injection_failed", err.Error(), map[string]any{"debug_url": process.DebugURL})
				}
			}

			now := time.Now().UTC()
			record := sessionstore.Record{
				SID:        sid,
				Name:       name,
				URL:        url,
				Profile:    profileName,
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
				"device":  deviceName,
				"browser": map[string]any{"name": resolved.Browser, "path": resolved.Path, "pid": process.PID, "debug_url": process.DebugURL, "headless": !headful},
				"page":    map[string]any{"url": url},
				"record":  record,
				"next_commands": map[string]string{
					"snapshot":   "aget page snapshot -s " + sid,
					"read":       "aget page read -s " + sid,
					"click_ref":  "aget page click -s " + sid + " --ref REF",
					"fill_ref":   "aget page fill -s " + sid + " --ref REF --text TEXT",
					"wait":       "aget page wait -s " + sid + " --text TEXT",
					"get":        "aget page get -s " + sid + " text --ref REF",
					"batch":      "aget batch -s " + sid + " --stdin",
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
	cmd.Flags().StringVar(&cookieInput, "cookies", "", "cookies to inject (file path for Netscape format, or inline name=value; pairs)")
	cmd.Flags().StringVar(&profileName, "profile", "", "persistent browser profile to use (created with aget profile create)")
	cmd.Flags().StringVar(&deviceName, "device", "", "device preset: mobile, tablet, or desktop (default: desktop)")
	configureAgentHelp(cmd)
	return cmd
}

// injectCookiesAndReload connects to the browser via CDP, injects cookies into
// the existing page, and reloads so the page picks up the cookies. The CDP
// connection is temporary — it's dropped after reload, but the tab stays open.
func injectCookiesAndReload(debugURL, targetURL string, cookieParams []*network.CookieParam) error {
	// Wait for CDP debug port to be ready
	ctx, cancel := context.WithTimeout(context.Background(), cdpReadyTimeout)
	defer cancel()
	if err := waitForCDPPort(ctx, debugURL); err != nil {
		return fmt.Errorf("browser CDP port not ready: %w", err)
	}

	// Create a temporary chromedp connection. Find the existing page target
	// that was navigated by the browser on launch, and connect to it.
	// Discard tab-level cancel to prevent closeTarget from closing the tab.
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), debugURL)
	defer allocCancel()
	tabCtx, _ := chromedp.NewContext(allocCtx)

	// Inject cookies, then reload to make the page use them
	if err := chromedp.Run(tabCtx,
		cookies.InjectCookiesAction(cookieParams),
		chromedp.Navigate(targetURL),
	); err != nil {
		return fmt.Errorf("cdp inject/reload: %w", err)
	}

	return nil
}

// emulateDeviceAndReload connects to the browser via CDP, applies device
// emulation (viewport scale, user-agent, touch), and reloads the page.
func emulateDeviceAndReload(debugURL string, preset devicePreset) error {
	ctx, cancel := context.WithTimeout(context.Background(), cdpReadyTimeout)
	defer cancel()
	if err := waitForCDPPort(ctx, debugURL); err != nil {
		return fmt.Errorf("browser CDP port not ready: %w", err)
	}

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), debugURL)
	defer allocCancel()
	tabCtx, _ := chromedp.NewContext(allocCtx)

	if err := chromedp.Run(tabCtx, deviceEmulationAction(preset)); err != nil {
		return fmt.Errorf("cdp device emulation: %w", err)
	}

	return nil
}

// waitForCDPPort polls the browser's DevTools Protocol endpoint until
// it responds successfully.
func waitForCDPPort(ctx context.Context, debugURL string) error {
	base := strings.TrimRight(debugURL, "/")
	url := base + "/json/version"

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			return nil
		}

		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
