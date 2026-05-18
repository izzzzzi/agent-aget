package browser

import (
	"os"
	"path/filepath"
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

	if _, err := ResolveBinary(missing); err == nil {
		t.Fatal("expected error")
	}
}
