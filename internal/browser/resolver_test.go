package browser

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestResolveBinaryRejectsNonExecutableExplicitPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable bits are not meaningful on windows")
	}

	path := filepath.Join(t.TempDir(), "cloak-browser")
	if err := os.WriteFile(path, []byte("not executable\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveBinary(path)
	if err == nil {
		t.Fatal("expected error")
	}
	assertExecutableError(t, err)
}

func TestResolveBinaryRejectsNonExecutableEnvPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable bits are not meaningful on windows")
	}

	path := filepath.Join(t.TempDir(), "cloak-browser")
	if err := os.WriteFile(path, []byte("not executable\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGET_BROWSER_PATH", path)

	_, err := ResolveBinary("")
	if err == nil {
		t.Fatal("expected error")
	}
	assertExecutableError(t, err)
}

func TestResolveBinaryUsesManagedBrowserBeforeSystemCandidates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode executable checks differ on windows")
	}
	t.Setenv("AGET_BROWSER_PATH", "")
	systemDir := t.TempDir()
	system := filepath.Join(systemDir, candidateNames()[0])
	if err := os.WriteFile(system, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", systemDir)

	managed := filepath.Join(t.TempDir(), "managed-chrome")
	if err := os.WriteFile(managed, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := managedBrowserPath
	managedBrowserPath = func() (string, bool) { return managed, true }
	defer func() { managedBrowserPath = old }()

	got, err := ResolveBinary("")
	if err != nil {
		t.Fatal(err)
	}
	if got != managed {
		t.Fatalf("ResolveBinary() = %q, want managed browser %q", got, managed)
	}
}

func assertActionableBrowserPathError(t *testing.T, err error) {
	t.Helper()
	message := err.Error()
	if !strings.Contains(message, "--browser-path") && !strings.Contains(message, "AGET_BROWSER_PATH") {
		t.Fatalf("error = %q, want mention of --browser-path or AGET_BROWSER_PATH", message)
	}
}

func assertExecutableError(t *testing.T, err error) {
	t.Helper()
	if !strings.Contains(strings.ToLower(err.Error()), "executable") {
		t.Fatalf("error = %q, want mention of executable", err.Error())
	}
}
