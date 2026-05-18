package cli

import "testing"

func TestPageReadRequiresSID(t *testing.T) {
	stdout, stderr, err := executeForTest("page", "read")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestPageClickRequiresSelector(t *testing.T) {
	stdout, stderr, err := executeForTest("page", "click", "-s", "abc12345")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestPageReadMissingSessionReturnsSessionNotFound(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())

	stdout, stderr, err := executeForTest("page", "read", "-s", "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertErrorCodeJSON(t, stderr, "session_not_found")
}

func TestPageNestedHelpReturnsJSONError(t *testing.T) {
	stdout, stderr, err := executeForTest("page", "read", "--help")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}
