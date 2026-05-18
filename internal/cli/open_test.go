package cli

import "testing"

func TestOpenRequiresURL(t *testing.T) {
	stdout, stderr, err := executeForTest("open")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestOpenRejectsExtraURL(t *testing.T) {
	stdout, stderr, err := executeForTest("open", "https://example.com", "https://example.org")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestOpenMissingBrowserPathReturnsBrowserNotFound(t *testing.T) {
	stdout, stderr, err := executeForTest("open", "https://example.com", "--browser-path", "/missing/browser")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertErrorCodeJSON(t, stderr, "browser_not_found")
}
