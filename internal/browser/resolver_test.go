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
	managedBrowserPath = func() (ResolvedBinary, bool) {
		return ResolvedBinary{Path: managed, Browser: "chrome-for-testing"}, true
	}
	defer func() { managedBrowserPath = old }()

	got, err := ResolveBinary("")
	if err != nil {
		t.Fatal(err)
	}
	if got != managed {
		t.Fatalf("ResolveBinary() = %q, want managed browser %q", got, managed)
	}
}

func TestResolveBinaryUsesCloakManagedBrowserBeforeChromeFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode executable checks differ on windows")
	}
	t.Setenv("AGET_BROWSER_PATH", "")

	cloak := filepath.Join(t.TempDir(), "managed-cloak")
	if err := os.WriteFile(cloak, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	chrome := filepath.Join(t.TempDir(), "managed-chrome")
	if err := os.WriteFile(chrome, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldPrimary := primaryManagedBrowserPath
	oldFallback := fallbackManagedBrowserPath
	primaryManagedBrowserPath = func() (ResolvedBinary, bool) {
		return ResolvedBinary{Path: cloak, Browser: "cloakbrowser"}, true
	}
	fallbackManagedBrowserPath = func() (ResolvedBinary, bool) {
		return ResolvedBinary{Path: chrome, Browser: "chrome-for-testing"}, true
	}
	defer func() {
		primaryManagedBrowserPath = oldPrimary
		fallbackManagedBrowserPath = oldFallback
	}()

	got, err := ResolveBinary("")
	if err != nil {
		t.Fatal(err)
	}
	if got != cloak {
		t.Fatalf("ResolveBinary() = %q, want CloakBrowser %q", got, cloak)
	}
}

func TestResolveReturnsCloakBrowserMetadataForManagedPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode executable checks differ on windows")
	}
	t.Setenv("AGET_BROWSER_PATH", "")

	cloak := filepath.Join(t.TempDir(), "managed-cloak")
	if err := os.WriteFile(cloak, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := managedBrowserPath
	managedBrowserPath = func() (ResolvedBinary, bool) {
		return ResolvedBinary{Path: cloak, Browser: "cloakbrowser"}, true
	}
	defer func() { managedBrowserPath = old }()

	got, err := Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != cloak || got.Browser != "cloakbrowser" {
		t.Fatalf("Resolve() = %#v, want CloakBrowser path %q", got, cloak)
	}
}

func TestResolveReturnsCloakBrowserMetadataForExplicitCloakPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable bits are not meaningful on windows")
	}
	exe := filepath.Join(t.TempDir(), "cloakbrowser")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve(exe)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != exe || got.Browser != "cloakbrowser" {
		t.Fatalf("Resolve() = %#v, want explicit CloakBrowser path %q", got, exe)
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
