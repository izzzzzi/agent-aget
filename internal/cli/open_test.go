package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestOpenRequiresURL(t *testing.T) {
	stdout, stderr, err := executeForTest("open")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestOpenRejectsExtraURL(t *testing.T) {
	stdout, stderr, err := executeForTest("open", "https://example.com", "https://example.org")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestOpenMissingBrowserPathReturnsBrowserNotFound(t *testing.T) {
	stdout, stderr, err := executeForTest("open", "https://example.com", "--browser-path", "/missing/browser")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertErrorCodeJSON(t, stderr, "browser_not_found")
}

func TestOpenReturnsCommandMapAndSessionName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake executable is not portable to windows")
	}
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	browserPath := writeFakeBrowser(t)
	restore := replaceOpenPageWaiterForTest(t, func(ctx context.Context, debugURL, url string) error {
		return nil
	})
	defer restore()

	stdout, stderr, err := executeForTest("open", "https://example.com", "--browser-path", browserPath, "-n", "research")
	if err != nil {
		t.Fatalf("open failed: %v stderr=%s", err, stderr)
	}

	var got struct {
		OK      bool   `json:"ok"`
		SID     string `json:"sid"`
		Session string `json:"session"`
		Browser struct {
			Path     string `json:"path"`
			PID      int    `json:"pid"`
			DebugURL string `json:"debug_url"`
			Headless bool   `json:"headless"`
		} `json:"browser"`
		Page struct {
			URL string `json:"url"`
		} `json:"page"`
		NextCommands map[string]string `json:"next_commands"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if got.Browser.PID > 0 {
			if process, err := os.FindProcess(got.Browser.PID); err == nil {
				_ = process.Kill()
				_, _ = process.Wait()
			}
		}
	})

	if !got.OK {
		t.Fatalf("ok = false")
	}
	if got.SID == "" {
		t.Fatalf("sid missing in %#v", got)
	}
	if got.Session != "research" {
		t.Fatalf("session = %q, want research", got.Session)
	}
	if got.Browser.Path != browserPath || got.Browser.PID <= 0 || got.Browser.DebugURL == "" || !got.Browser.Headless {
		t.Fatalf("browser = %#v", got.Browser)
	}
	if got.Page.URL != "https://example.com" {
		t.Fatalf("page.url = %q", got.Page.URL)
	}

	want := map[string]string{
		"snapshot":   "aget page snapshot -s " + got.SID,
		"read":       "aget page read -s " + got.SID,
		"click_ref":  "aget page click -s " + got.SID + " --ref REF",
		"fill_ref":   "aget page fill -s " + got.SID + " --ref REF --text TEXT",
		"wait":       "aget page wait -s " + got.SID + " --text TEXT",
		"get":        "aget page get -s " + got.SID + " text --ref REF",
		"batch":      "aget batch -s " + got.SID + " --stdin",
		"screenshot": "aget page screenshot -s " + got.SID,
		"close":      "aget session close -s " + got.SID,
	}
	for key, value := range want {
		if got.NextCommands[key] != value {
			t.Fatalf("next_commands[%q] = %q, want %q", key, got.NextCommands[key], value)
		}
	}
}

func TestOpenWaitsForPageBeforeReturning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake executable is not portable to windows")
	}
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	browserPath := writeFakeBrowser(t)
	var gotDebugURL string
	var gotURL string
	restore := replaceOpenPageWaiterForTest(t, func(ctx context.Context, debugURL, url string) error {
		gotDebugURL = debugURL
		gotURL = url
		return nil
	})
	defer restore()

	stdout, stderr, err := executeForTest("open", "https://example.com/docs", "--browser-path", browserPath)
	if err != nil {
		t.Fatalf("open failed: %v stderr=%s", err, stderr)
	}
	var got struct {
		Browser struct {
			PID int `json:"pid"`
		} `json:"browser"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if got.Browser.PID > 0 {
			if process, err := os.FindProcess(got.Browser.PID); err == nil {
				_ = process.Kill()
				_, _ = process.Wait()
			}
		}
	})

	if gotDebugURL == "" {
		t.Fatal("open did not wait for browser page readiness")
	}
	if gotURL != "https://example.com/docs" {
		t.Fatalf("wait URL = %q, want https://example.com/docs", gotURL)
	}
}

func TestOpenGivesManagedBrowserEnoughTimeToBecomeReady(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake executable is not portable to windows")
	}
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	browserPath := writeFakeBrowser(t)
	var gotTimeout bool
	restore := replaceOpenPageWaiterForTest(t, func(ctx context.Context, debugURL, url string) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("open readiness context has no deadline")
		}
		gotTimeout = time.Until(deadline) >= 25*time.Second
		return nil
	})
	defer restore()

	stdout, stderr, err := executeForTest("open", "https://example.com/docs", "--browser-path", browserPath)
	if err != nil {
		t.Fatalf("open failed: %v stderr=%s", err, stderr)
	}
	var got struct {
		Browser struct {
			PID int `json:"pid"`
		} `json:"browser"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if got.Browser.PID > 0 {
			if process, err := os.FindProcess(got.Browser.PID); err == nil {
				_ = process.Kill()
				_, _ = process.Wait()
			}
		}
	})

	if !gotTimeout {
		t.Fatal("open readiness timeout is too short for managed browser cold starts")
	}
}

func writeFakeBrowser(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	exe := filepath.Join(dir, "fake-browser")
	script := "#!/bin/sh\nsleep 60\n"
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

func replaceOpenPageWaiterForTest(t *testing.T, waiter func(context.Context, string, string) error) func() {
	t.Helper()
	old := waitForOpenPage
	waitForOpenPage = waiter
	return func() {
		waitForOpenPage = old
	}
}
