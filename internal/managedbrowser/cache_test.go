package managedbrowser

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCacheDirUsesOverride(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGET_BROWSER_CACHE_DIR", root)

	got, err := CacheRoot()
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("CacheRoot() = %q, want %q", got, root)
	}
}

func TestInstallPathsUseVersionAndPlatform(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGET_BROWSER_CACHE_DIR", root)
	entry := Platform{ExecutablePath: filepath.Join("chrome-linux64", "chrome")}

	paths, err := Paths("148.0.7778.98", "linux-x64", entry)
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(root, "agent-aget", "chrome-for-testing", "148.0.7778.98", "linux-x64")
	if paths.InstallDir != wantDir {
		t.Fatalf("InstallDir = %q, want %q", paths.InstallDir, wantDir)
	}
	if !strings.HasSuffix(paths.Executable, filepath.Join("linux-x64", "chrome-linux64", "chrome")) {
		t.Fatalf("Executable = %q", paths.Executable)
	}
}

func TestStatusDetectsInstalledExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode executable checks differ on windows")
	}
	root := t.TempDir()
	t.Setenv("AGET_BROWSER_CACHE_DIR", root)
	entry := Platform{ExecutablePath: filepath.Join("chrome-linux64", "chrome")}

	paths, err := Paths("148.0.7778.98", "linux-x64", entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.Executable), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	status := Status(paths)
	if !status.Installed || !status.Executable {
		t.Fatalf("status = %#v, want installed executable", status)
	}
}
