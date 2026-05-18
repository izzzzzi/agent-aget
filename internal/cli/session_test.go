package cli

import (
	"encoding/json"
	"testing"
)

func TestSessionListEmpty(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())

	stdout, stderr, err := executeForTest("session", "list")
	if err != nil {
		t.Fatalf("session list failed: %v stderr=%s", err, stderr)
	}

	var got struct {
		OK       bool  `json:"ok"`
		Sessions []any `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if !got.OK {
		t.Fatalf("ok = false")
	}
	if got.Sessions == nil || len(got.Sessions) != 0 {
		t.Fatalf("sessions = %#v, want empty slice", got.Sessions)
	}
}

func TestSessionCloseRequiresSID(t *testing.T) {
	stdout, stderr, err := executeForTest("session", "close")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestSessionCloseMissingReturnsSessionNotFound(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())

	stdout, stderr, err := executeForTest("session", "close", "-s", "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertErrorCodeJSON(t, stderr, "session_not_found")
}
