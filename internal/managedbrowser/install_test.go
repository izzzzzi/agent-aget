package managedbrowser

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallReportsAlreadyInstalled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode executable checks differ on windows")
	}
	root := t.TempDir()
	t.Setenv(CacheEnv, root)
	entry := Platform{ExecutablePath: "chrome-linux64/chrome"}
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

	result, err := Install(context.Background(), Manifest{
		Version:   "148.0.7778.98",
		Platforms: map[string]Platform{"linux-x64": entry},
	}, "linux-x64")
	if err != nil {
		t.Fatal(err)
	}
	if !result.AlreadyInstalled {
		t.Fatalf("AlreadyInstalled = false")
	}
	if result.Path != paths.Executable {
		t.Fatalf("Path = %q, want %q", result.Path, paths.Executable)
	}
}

func TestInstallRejectsChecksumMismatch(t *testing.T) {
	root := t.TempDir()
	t.Setenv(CacheEnv, root)
	archive := makeZip(t, "chrome-linux64/chrome", "#!/bin/sh\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	_, err := Install(context.Background(), Manifest{
		Version: "148.0.7778.98",
		Platforms: map[string]Platform{"linux-x64": {
			Archive: "chrome-linux64.zip", URL: server.URL, SHA256: "0000", ExecutablePath: "chrome-linux64/chrome",
		}},
	}, "linux-x64")
	if err == nil {
		t.Fatal("expected checksum error")
	}
}

func TestInstallDownloadsExtractsAndValidatesExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode executable checks differ on windows")
	}
	root := t.TempDir()
	t.Setenv(CacheEnv, root)
	archive := makeZip(t, "chrome-linux64/chrome", "#!/bin/sh\n")
	sum := sha256.Sum256(archive)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	result, err := Install(context.Background(), Manifest{
		Version: "148.0.7778.98",
		Platforms: map[string]Platform{"linux-x64": {
			Archive:        "chrome-linux64.zip",
			URL:            server.URL,
			SHA256:         hex.EncodeToString(sum[:]),
			ExecutablePath: "chrome-linux64/chrome",
		}},
	}, "linux-x64")
	if err != nil {
		t.Fatal(err)
	}
	if result.AlreadyInstalled {
		t.Fatal("AlreadyInstalled = true")
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatal(err)
	}
}

func TestSafeZipPathRejectsTraversal(t *testing.T) {
	destination := t.TempDir()
	tests := []string{
		"../chrome",
		"chrome-linux64/../../chrome",
	}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := safeZipPath(destination, name); err == nil {
				t.Fatal("safeZipPath() error = nil, want error")
			}
		})
	}
}

func makeZip(t *testing.T, name, body string) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
