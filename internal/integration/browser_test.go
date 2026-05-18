package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestBrowserSessionSmoke(t *testing.T) {
	browserPath := os.Getenv("AGET_BROWSER_PATH")
	if browserPath == "" {
		t.Skip("AGET_BROWSER_PATH not set")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
<head><title>aget smoke</title></head>
<body><main><h1>aget smoke</h1><p>ready</p></main></body>
</html>`))
	}))
	defer server.Close()

	stateDir := t.TempDir()
	openOut := runAget(t, stateDir, "open", server.URL, "--browser-path", browserPath)

	var opened struct {
		SID     string `json:"sid"`
		Browser struct {
			PID int `json:"pid"`
		} `json:"browser"`
	}
	if err := json.Unmarshal(openOut, &opened); err != nil {
		t.Fatalf("open output is not valid JSON: %v\n%s", err, openOut)
	}
	if opened.SID == "" {
		t.Fatalf("open output missing sid: %s", openOut)
	}
	if opened.Browser.PID <= 0 {
		t.Fatalf("open output missing browser pid: %s", openOut)
	}
	defer cleanupBrowserSession(t, stateDir, opened.SID, opened.Browser.PID)

	readOut := readPageWithRetry(t, stateDir, opened.SID)
	var readPayload map[string]any
	if err := json.Unmarshal(readOut, &readPayload); err != nil {
		t.Fatalf("read output is not valid JSON: %v\n%s", err, readOut)
	}
}

func readPageWithRetry(t *testing.T, stateDir, sid string) []byte {
	t.Helper()

	var lastErr error
	var lastOut []byte
	for attempt := 0; attempt < 10; attempt++ {
		out, err := runAgetResult(stateDir, "page", "read", "-s", sid, "--limit", "10")
		if err == nil {
			return out
		}
		lastErr = err
		lastOut = out
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("page read failed after retries: %v\n%s", lastErr, lastOut)
	return nil
}

func cleanupBrowserSession(t *testing.T, stateDir, sid string, pid int) {
	t.Helper()

	if _, err := runAgetResult(stateDir, "session", "close", "-s", sid); err != nil {
		t.Logf("session close failed during cleanup: %v", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		t.Logf("find browser process %d during cleanup: %v", pid, err)
		return
	}
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Logf("kill browser process %d during cleanup: %v", pid, err)
	}
	_, _ = process.Wait()
}

func runAget(t *testing.T, stateDir string, args ...string) []byte {
	t.Helper()

	out, err := runAgetResult(stateDir, args...)
	if err != nil {
		t.Fatalf("go run ./cmd/aget %v failed: %v\n%s", args, err, out)
	}
	return out
}

func runAgetResult(stateDir string, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"run", "./cmd/aget"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = repoRoot()
	cmd.Env = append(os.Environ(), "AGET_STATE_DIR="+stateDir)

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	err := cmd.Run()
	return combined.Bytes(), err
}

func repoRoot() string {
	return filepath.Clean("../..")
}
