package cli

import (
	"encoding/json"
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
