package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveBinaryUsesExplicitPath(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "cloak-browser")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveBinary(exe)
	if err != nil {
		t.Fatal(err)
	}
	if got != exe {
		t.Fatalf("ResolveBinary() = %q, want %q", got, exe)
	}
}

func TestResolveBinaryRejectsMissingExplicitPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")

	_, err := ResolveBinary(missing)
	if err == nil {
		t.Fatal("expected error")
	}
	assertActionableBrowserPathError(t, err)
}

func TestResolveBinaryRejectsMissingEnvPath(t *testing.T) {
	t.Setenv("AGET_BROWSER_PATH", filepath.Join(t.TempDir(), "missing"))

	_, err := ResolveBinary("")
	if err == nil {
		t.Fatal("expected error")
	}
	assertActionableBrowserPathError(t, err)
}

func assertActionableBrowserPathError(t *testing.T, err error) {
	t.Helper()
	message := err.Error()
	if !strings.Contains(message, "--browser-path") && !strings.Contains(message, "AGET_BROWSER_PATH") {
		t.Fatalf("error = %q, want mention of --browser-path or AGET_BROWSER_PATH", message)
	}
}
