package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	sessionstore "github.com/izzzzzi/agent-aget/internal/session"
	"github.com/izzzzzi/agent-aget/internal/state"
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

	stdout, stderr, err := executeForTest("session", "close", "-s", "deadbeef")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertErrorCodeJSON(t, stderr, "session_not_found")
}

func TestSessionCloseDeletesRecordAndSessionUserDataDir(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	sid := "deadbeef"
	now := time.Now().UTC()
	record := sessionstore.Record{SID: sid, URL: "https://example.com", BrowserPID: 0, CreatedAt: now, UpdatedAt: now}
	if err := sessionstore.NewRegistry(state.SessionsDir()).Save(record); err != nil {
		t.Fatal(err)
	}
	userDataDir := filepath.Join(state.ProfilesDir(), sid)
	if err := os.MkdirAll(userDataDir, 0o700); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := executeForTest("session", "close", "-s", sid)
	if err != nil {
		t.Fatalf("session close failed: %v stderr=%s", err, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("unexpected response: %#v", got)
	}
	if _, err := sessionstore.NewRegistry(state.SessionsDir()).Get(sid); err != sessionstore.ErrNotFound {
		t.Fatalf("session Get err = %v, want ErrNotFound", err)
	}
	if _, err := os.Stat(userDataDir); !os.IsNotExist(err) {
		t.Fatalf("user data dir still exists or unexpected stat err: %v", err)
	}
}

func TestSessionGCRemovesStaleRecordsAndUserData(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	sid := "cafebabe"
	now := time.Now().UTC()
	record := sessionstore.Record{SID: sid, URL: "https://example.com", BrowserPID: 0, CreatedAt: now, UpdatedAt: now}
	if err := sessionstore.NewRegistry(state.SessionsDir()).Save(record); err != nil {
		t.Fatal(err)
	}
	userDataDir := filepath.Join(state.ProfilesDir(), sid)
	if err := os.MkdirAll(userDataDir, 0o700); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := executeForTest("session", "gc")
	if err != nil {
		t.Fatalf("session gc failed: %v stderr=%s", err, stderr)
	}
	var got struct {
		OK      bool     `json:"ok"`
		Removed []string `json:"removed"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if !got.OK || len(got.Removed) != 1 || got.Removed[0] != sid {
		t.Fatalf("unexpected response: %#v", got)
	}
	if _, err := os.Stat(userDataDir); !os.IsNotExist(err) {
		t.Fatalf("user data dir still exists or unexpected stat err: %v", err)
	}
}
