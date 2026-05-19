package cli

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/izzzzzi/agent-aget/internal/managedbrowser"
)

func TestBrowserStatusReportsMissingManagedBrowser(t *testing.T) {
	t.Setenv(managedbrowser.CacheEnv, t.TempDir())

	stdout, stderr, err := executeForTest("browser", "status")
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %v", got["ok"])
	}
	if got["installed"] != false {
		t.Fatalf("installed = %v, want false", got["installed"])
	}
}

func TestBrowserPathErrorsWhenManagedBrowserMissing(t *testing.T) {
	t.Setenv(managedbrowser.CacheEnv, t.TempDir())

	_, stderr, err := executeForTest("browser", "path")
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrorCodeJSON(t, stderr, "browser_not_installed")
}

func TestBrowserCommandsMapUnsupportedPlatformErrorsConsistently(t *testing.T) {
	err := errors.New("unsupported managed browser platform: plan9-amd64")

	for _, fallback := range []string{"browser_status_failed", "browser_path_failed", "browser_install_failed"} {
		if got := browserErrorCode(fallback, err); got != "browser_unsupported_platform" {
			t.Fatalf("browserErrorCode(%q, err) = %q, want browser_unsupported_platform", fallback, got)
		}
	}
}
