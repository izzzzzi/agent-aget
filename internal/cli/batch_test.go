package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/izzzzzi/agent-aget/internal/cdp"
)

func executeForTestWithStdin(stdin string, args ...string) (string, string, error) {
	cmd := NewRootCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestBatchRequiresStdinFlag(t *testing.T) {
	stdout, stderr, err := executeForTestWithStdin(`[]`, "batch", "-s", "abc12345")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestBatchStopsOnFirstErrorAndKeepsFailureOnStdout(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &failingBatchDriver{failOnPress: true}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	input := `[
		{"cmd":"fill","selector":"#email","text":"secret@example.com"},
		{"cmd":"press","key":"Enter"},
		{"cmd":"click","selector":"#after"}
	]`
	stdout, stderr, err := executeForTestWithStdin(input, "batch", "-s", "abc12345", "--stdin")
	if err == nil {
		t.Fatal("expected error")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if strings.Contains(stdout, "secret@example.com") {
		t.Fatalf("stdout leaked fill text: %s", stdout)
	}
	if driver.clicked != "" {
		t.Fatalf("batch did not stop on first failure; clicked = %q", driver.clicked)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != false || got["sid"] != "abc12345" || got["failed_index"] != float64(1) {
		t.Fatalf("response = %#v", got)
	}
	results := got["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	first := results[0].(map[string]any)
	if first["cmd"] != "fill" || first["text_len"] != float64(18) {
		t.Fatalf("first result = %#v", first)
	}
	errorPayload := got["error"].(map[string]any)
	if errorPayload["code"] != "page_action_failed" || errorPayload["message"] == "" {
		t.Fatalf("error = %#v", errorPayload)
	}
}

func TestBatchSuccessIncludesResultsAndSavesSnapshotRefs(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{
		snapshot: cdp.SnapshotState{
			URL:   "https://example.com",
			Title: "Example",
			Elements: []cdp.Element{
				{Kind: "button", Selector: "button[type=submit]", Text: "Submit", Visible: true, Enabled: true},
			},
		},
	}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	input := `[
		{"cmd":"snapshot"},
		{"cmd":"click","ref":"@e1"},
		{"cmd":"get","kind":"url"}
	]`
	stdout, stderr, err := executeForTestWithStdin(input, "batch", "-s", "abc12345", "--stdin")
	if err != nil {
		t.Fatalf("batch failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if driver.clicked != "button[type=submit]" {
		t.Fatalf("clicked = %q", driver.clicked)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["sid"] != "abc12345" {
		t.Fatalf("response = %#v", got)
	}
	results := got["results"].([]any)
	if len(results) != 3 {
		t.Fatalf("results len = %d, want 3", len(results))
	}
	snapshot := results[0].(map[string]any)
	elements := snapshot["elements"].([]any)
	first := elements[0].(map[string]any)
	if first["ref"] != "@e1" {
		t.Fatalf("snapshot first ref = %v", first["ref"])
	}
}

func TestBatchInvalidJSONReturnsStructuredStdoutError(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")

	stdout, stderr, err := executeForTestWithStdin(`{"cmd":"fill"}`, "batch", "-s", "abc12345", "--stdin")
	if err == nil {
		t.Fatal("expected error")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != false || got["sid"] != "abc12345" || got["failed_index"] != nil {
		t.Fatalf("response = %#v", got)
	}
	errorPayload := got["error"].(map[string]any)
	if errorPayload["code"] != "invalid_json" {
		t.Fatalf("error = %#v", errorPayload)
	}
}

func TestBatchFillRequiresText(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	stdout, stderr, err := executeForTestWithStdin(`[{"cmd":"fill","selector":"#email"}]`, "batch", "-s", "abc12345", "--stdin")
	if err == nil {
		t.Fatal("expected error")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != false || got["failed_index"] != float64(0) {
		t.Fatalf("response = %#v", got)
	}
	errorPayload := got["error"].(map[string]any)
	if errorPayload["code"] != "invalid_args" {
		t.Fatalf("error = %#v", errorPayload)
	}
}

type failingBatchDriver struct {
	recordingDriver
	failOnPress bool
}

func (d *failingBatchDriver) Press(context.Context, string) error {
	if d.failOnPress {
		return errors.New("press failed")
	}
	return nil
}
