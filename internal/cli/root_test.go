package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

var errWriteFailed = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWriteFailed
}

func executeForTest(args ...string) (string, string, error) {
	cmd := NewRootCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestRootRequiresCommand(t *testing.T) {
	stdout, stderr, err := executeForTest()
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	var got map[string]any
	if json.Unmarshal([]byte(stderr), &got) != nil {
		t.Fatalf("stderr is not json: %q", stderr)
	}
	if got["code"] != "invalid_args" {
		t.Fatalf("code = %v", got["code"])
	}
}

func TestVersionCommand(t *testing.T) {
	stdout, stderr, err := executeForTest("version")
	if err != nil {
		t.Fatalf("version failed: %v stderr=%s", err, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %v", got["ok"])
	}
	if got["version"] == "" {
		t.Fatalf("version missing in %v", got)
	}
}

func TestVersionHelpReturnsJSONError(t *testing.T) {
	stdout, stderr, err := executeForTest("version", "--help")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestUnknownCommandWithHelpReturnsJSONError(t *testing.T) {
	stdout, stderr, err := executeForTest("unknown", "--help")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestCompletionCommandDisabled(t *testing.T) {
	stdout, stderr, err := executeForTest("completion", "bash")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestWriteJSONPropagatesWriteError(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetOut(failingWriter{})
	err := writeJSON(cmd, map[string]any{"ok": true})
	if !errors.Is(err, errWriteFailed) {
		t.Fatalf("err = %v, want %v", err, errWriteFailed)
	}
}

func TestWriteErrorPropagatesWriteError(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetErr(failingWriter{})
	err := writeError(cmd, "invalid_args", "bad args", nil)
	if !errors.Is(err, errWriteFailed) {
		t.Fatalf("err = %v, want %v", err, errWriteFailed)
	}
}

func assertInvalidArgsJSON(t *testing.T, body string) {
	t.Helper()
	assertErrorCodeJSON(t, body, "invalid_args")
}

func assertErrorCodeJSON(t *testing.T, body string, want string) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		if errors.Is(err, io.EOF) {
			t.Fatalf("stderr is empty")
		}
		t.Fatalf("stderr is not json: %q", body)
	}
	if got["code"] != want {
		t.Fatalf("code = %v", got["code"])
	}
}
