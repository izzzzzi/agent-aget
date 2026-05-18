package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

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
