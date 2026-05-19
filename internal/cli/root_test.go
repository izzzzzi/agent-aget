package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
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

func TestVersionHelpReturnsAgentHelp(t *testing.T) {
	stdout, stderr, err := executeForTest("version", "--help")
	if err != nil {
		t.Fatalf("expected help success, got err=%v stderr=%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["kind"] != "agent_help" || got["command_group"] != "version" {
		t.Fatalf("unexpected help payload: %#v", got)
	}
}

func TestRootHelpReturnsAgentHelp(t *testing.T) {
	stdout, stderr, err := executeForTest("--help")
	if err != nil {
		t.Fatalf("expected help success, got err=%v stderr=%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["tool"] != "aget" || got["audience"] != "llm_agent" || got["agent_prompt_command"] != "aget prompt" {
		t.Fatalf("unexpected root help payload: %#v", got)
	}
	commands, ok := got["commands"].(map[string]any)
	if !ok {
		t.Fatalf("commands missing or wrong type: %#v", got["commands"])
	}
	if commands["open"] == "" || commands["page_read"] == "" {
		t.Fatalf("core commands missing: %#v", commands)
	}
}

func TestBrowserHelpReturnsAgentHelp(t *testing.T) {
	stdout, stderr, err := executeForTest("browser", "--help")
	if err != nil {
		t.Fatalf("expected help success, got err=%v stderr=%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["kind"] != "agent_help" || got["command_group"] != "browser" {
		t.Fatalf("unexpected browser help payload: %#v", got)
	}
	commands, ok := got["commands"].(map[string]any)
	if !ok {
		t.Fatalf("commands missing or wrong type: %#v", got["commands"])
	}
	if commands["status"] == "" || commands["install"] == "" || commands["path"] == "" {
		t.Fatalf("browser commands missing: %#v", commands)
	}
}

func TestOpenHelpReturnsAgentHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "help without url", args: []string{"open", "--help"}},
		{name: "help before url", args: []string{"open", "--help", "https://example.com"}},
		{name: "help after url", args: []string{"open", "https://example.com", "--help"}},
		{name: "short help before url", args: []string{"open", "-h", "https://example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AGET_BROWSER_PATH", "/missing/browser")

			stdout, stderr, err := executeForTest(tt.args...)
			if err != nil {
				t.Fatalf("expected help success, got err=%v stderr=%s", err, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatal(err)
			}
			if got["ok"] != true || got["kind"] != "agent_help" || got["command_group"] != "open" {
				t.Fatalf("unexpected open help payload: %#v", got)
			}
		})
	}
}

func TestInvalidArgsHintPointsToPrompt(t *testing.T) {
	_, stderr, err := executeForTest()
	if err == nil {
		t.Fatal("expected error")
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stderr), &got); err != nil {
		t.Fatal(err)
	}
	details, ok := got["details"].(map[string]any)
	if !ok {
		t.Fatalf("details missing: %#v", got)
	}
	hint, _ := details["hint"].(string)
	if hint == "" || !strings.Contains(hint, "aget prompt") || !strings.Contains(hint, "aget --help") {
		t.Fatalf("hint = %q", hint)
	}
}

func TestHelpWithInvalidPositionalsReturnsJSONError(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown before root help", args: []string{"unknown", "--help"}},
		{name: "unknown after root help", args: []string{"--help", "unknown"}},
		{name: "unknown page subcommand before help", args: []string{"page", "bogus", "--help"}},
		{name: "unknown page subcommand after help", args: []string{"page", "--help", "bogus"}},
		{name: "extra page read arg after help", args: []string{"page", "read", "--help", "bogus"}},
		{name: "extra open arg after help before url", args: []string{"open", "--help", "https://example.com", "extra"}},
		{name: "extra open arg after help after url", args: []string{"open", "https://example.com", "--help", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := executeForTest(tt.args...)
			if err == nil {
				t.Fatal("expected error")
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			assertInvalidArgsJSON(t, stderr)
		})
	}
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
