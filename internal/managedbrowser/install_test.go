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

func TestInstallCreatesMissingCacheRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing-cache-root")
	t.Setenv(CacheEnv, root)
	archive := makeZip(t, "chrome-linux64/chrome", "#!/bin/sh\n")
	sum := sha256.Sum256(archive)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	_, err := Install(context.Background(), Manifest{
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
	if _, err := os.Stat(root); err != nil {
		t.Fatal(err)
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

func TestInstallRejectsUnsafeArchiveName(t *testing.T) {
	root := t.TempDir()
	t.Setenv(CacheEnv, root)
	archiveBody := makeZip(t, "chrome-linux64/chrome", "#!/bin/sh\n")
	sum := sha256.Sum256(archiveBody)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archiveBody)
	}))
	defer server.Close()
	tests := []string{
		"../chrome.zip",
	}
	if runtime.GOOS != "windows" {
		tests = append(tests, "/tmp/chrome.zip")
		defer os.Remove("/tmp/chrome.zip")
	}
	for _, archive := range tests {
		t.Run(archive, func(t *testing.T) {
			_, err := Install(context.Background(), Manifest{
				Version: "148.0.7778.98",
				Platforms: map[string]Platform{"linux-x64": {
					Archive:        archive,
					URL:            server.URL,
					SHA256:         hex.EncodeToString(sum[:]),
					ExecutablePath: "chrome-linux64/chrome",
				}},
			}, "linux-x64")
			if err == nil {
				t.Fatal("expected unsafe archive error")
			}
		})
	}
}

func TestInstallKeepsExistingInstallWhenNewArchiveInvalid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode executable checks differ on windows")
	}
	root := t.TempDir()
	t.Setenv(CacheEnv, root)
	entry := Platform{
		Archive:        "chrome-linux64.zip",
		ExecutablePath: "chrome-linux64/chrome",
	}
	paths, err := Paths("148.0.7778.98", "linux-x64", entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.Executable), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Executable, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	archive := makeZip(t, "chrome-linux64/not-chrome", "new")
	sum := sha256.Sum256(archive)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()
	entry.URL = server.URL
	entry.SHA256 = hex.EncodeToString(sum[:])

	_, err = Install(context.Background(), Manifest{
		Version:   "148.0.7778.98",
		Platforms: map[string]Platform{"linux-x64": entry},
	}, "linux-x64")
	if err == nil {
		t.Fatal("expected staged executable validation error")
	}
	body, err := os.ReadFile(paths.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "old" {
		t.Fatalf("existing executable body = %q, want old", string(body))
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

func TestExtractZipPreservesSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on many windows systems")
	}
	archivePath := filepath.Join(t.TempDir(), "archive.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	header := &zip.FileHeader{Name: "chrome-mac-arm64/Google Chrome for Testing.app/Contents/Frameworks/Chrome Framework.framework/Resources"}
	header.SetMode(os.ModeSymlink | 0o777)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("Versions/Current/Resources")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	destination := t.TempDir()
	if err := extractZip(archivePath, destination); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(destination, filepath.FromSlash(header.Name))
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("mode = %v, want symlink", info.Mode())
	}
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if target != "Versions/Current/Resources" {
		t.Fatalf("target = %q, want %q", target, "Versions/Current/Resources")
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
