package cli

import (
	"encoding/json"
	"testing"
)

func TestDoctorReturnsJSON(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())

	stdout, stderr, err := executeForTest("doctor")

	if err != nil && stdout == "" {
		t.Fatalf("doctor returned no JSON: err=%v stderr=%s", err, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not json: %s", stdout)
	}
	if got["ok"] == nil || got["checks"] == nil {
		t.Fatalf("doctor response = %#v", got)
	}
	checks, ok := got["checks"].([]any)
	if !ok || len(checks) == 0 {
		t.Fatalf("checks missing or empty: %#v", got["checks"])
	}
	names := map[string]bool{}
	for _, raw := range checks {
		check, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("check has wrong shape: %#v", raw)
		}
		name, _ := check["name"].(string)
		names[name] = true
		if check["ok"] == nil || check["message"] == nil {
			t.Fatalf("check missing fields: %#v", check)
		}
	}
	if !names["state_dir"] || !names["sessions_dir"] || !names["artifacts_dir"] || !names["snapshots_dir"] || !names["browser"] {
		t.Fatalf("required checks missing: %#v", names)
	}
}

func TestDoctorRejectsPositionalsWithJSONError(t *testing.T) {
	stdout, stderr, err := executeForTest("doctor", "extra")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestDoctorHelpReturnsAgentHelp(t *testing.T) {
	stdout, stderr, err := executeForTest("doctor", "--help")
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
	if got["ok"] != true || got["kind"] != "agent_help" {
		t.Fatalf("unexpected help payload: %#v", got)
	}
}
