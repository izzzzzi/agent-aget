# Managed Browser Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a managed Chrome for Testing install path so `agent-aget` normally works after npm install without requiring a manually installed Chrome.

**Architecture:** A shared `browser-manifest.json` pins one Chrome for Testing version and platform archive metadata. Go owns runtime status, path resolution, CLI commands, and strict `aget browser install`; the npm `postinstall` reuses the same manifest for best-effort browser installation into the user cache.

**Tech Stack:** Go 1.22, Cobra, Node 18+ postinstall scripts, Chrome for Testing official archive URLs, existing JSON response contract.

---

## File Structure

Create:

```text
browser-manifest.json
internal/managedbrowser/browser-manifest.json
internal/managedbrowser/cache.go
internal/managedbrowser/cache_test.go
internal/managedbrowser/install.go
internal/managedbrowser/install_test.go
internal/managedbrowser/manifest.go
internal/managedbrowser/manifest_test.go
internal/cli/browser.go
internal/cli/browser_test.go
scripts/browser-install.js
scripts/browser-install.test.js
```

Modify:

```text
internal/browser/resolver.go
internal/browser/resolver_test.go
internal/cli/root.go
scripts/install.js
scripts/release-contract-test.js
scripts/smoke-test.js
package.json
README.md
README.en.md
```

Responsibilities:

- `browser-manifest.json`: root pinned browser contract consumed by npm packaging.
- `internal/managedbrowser/browser-manifest.json`: embedded copy of the root manifest consumed by the Go binary; release contract keeps both files byte-identical.
- `internal/managedbrowser/manifest.go`: embed/parse manifest and map Go platforms to manifest entries.
- `internal/managedbrowser/cache.go`: compute cache root, install directory, executable path, and installed status.
- `internal/managedbrowser/install.go`: strict download, checksum, extract, atomic install, and executable validation for Go CLI.
- `internal/cli/browser.go`: JSON command surface for `aget browser status|path|install`.
- `internal/browser/resolver.go`: browser priority order for `open`.
- `scripts/browser-install.js`: npm-side best-effort installer using the same manifest and cache layout.
- `scripts/install.js`: call browser installer after native binary install.
- `scripts/release-contract-test.js`: package/release contract checks for manifest and command surface.

## Task 1: Shared Manifest and Cache Model

**Files:**

- Create: `browser-manifest.json`
- Create: `internal/managedbrowser/browser-manifest.json`
- Create: `internal/managedbrowser/manifest.go`
- Create: `internal/managedbrowser/manifest_test.go`
- Create: `internal/managedbrowser/cache.go`
- Create: `internal/managedbrowser/cache_test.go`
- Modify: `package.json`

- [ ] **Step 1: Write failing manifest tests**

Create `internal/managedbrowser/manifest_test.go`:

```go
package managedbrowser

import "testing"

func TestBundledManifestHasPinnedVersionAndPlatforms(t *testing.T) {
	manifest, err := BundledManifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version == "" {
		t.Fatal("Version is empty")
	}
	for _, platform := range []string{"darwin-arm64", "darwin-x64", "linux-x64", "win32-x64"} {
		entry, ok := manifest.Platforms[platform]
		if !ok {
			t.Fatalf("missing platform %s", platform)
		}
		if entry.URL == "" || entry.Archive == "" || entry.SHA256 == "" || entry.ExecutablePath == "" {
			t.Fatalf("incomplete manifest entry for %s: %#v", platform, entry)
		}
	}
}

func TestCurrentPlatformEntryRejectsUnknownPlatform(t *testing.T) {
	manifest := Manifest{Version: "1", Platforms: map[string]Platform{}}
	_, err := manifest.PlatformEntry("linux-arm64")
	if err == nil {
		t.Fatal("expected unsupported platform error")
	}
}
```

- [ ] **Step 2: Run manifest tests and verify failure**

Run:

```bash
go test -count=1 ./internal/managedbrowser -run 'TestBundledManifest|TestCurrentPlatformEntry'
```

Expected: FAIL because `internal/managedbrowser` does not exist.

- [ ] **Step 3: Generate manifest checksums**

Choose one Chrome for Testing version from the official Chrome for Testing availability dashboard and compute sha256 values before creating the manifest. Chrome for Testing does not publish a separate Linux ARM64 Chrome archive, so managed browser install supports `darwin-arm64`, `darwin-x64`, `linux-x64`, and `win32-x64` in the MVP; Linux ARM64 keeps system-browser fallback.

Run these commands from the repo root, replacing `148.0.7778.98` only if the official dashboard shows a newer stable version at implementation time:

```bash
VERSION=148.0.7778.98
TMP=$(mktemp -d)
curl -fsSL "https://storage.googleapis.com/chrome-for-testing-public/${VERSION}/mac-arm64/chrome-mac-arm64.zip" -o "$TMP/chrome-mac-arm64.zip"
curl -fsSL "https://storage.googleapis.com/chrome-for-testing-public/${VERSION}/mac-x64/chrome-mac-x64.zip" -o "$TMP/chrome-mac-x64.zip"
curl -fsSL "https://storage.googleapis.com/chrome-for-testing-public/${VERSION}/linux64/chrome-linux64.zip" -o "$TMP/chrome-linux64.zip"
curl -fsSL "https://storage.googleapis.com/chrome-for-testing-public/${VERSION}/win64/chrome-win64.zip" -o "$TMP/chrome-win64.zip"
shasum -a 256 "$TMP"/*.zip
```

Expected: four sha256 lines, one for each downloaded archive. Keep `TMP` available in the same shell for the next step.

- [ ] **Step 4: Add manifest files and parser**

Create `browser-manifest.json` from the downloaded archives:

```bash
VERSION="$VERSION" TMP="$TMP" node <<'NODE'
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');

const version = process.env.VERSION;
const tmp = process.env.TMP;
function sha(file) {
  return crypto.createHash('sha256').update(fs.readFileSync(path.join(tmp, file))).digest('hex');
}
const manifest = {
  version,
  platforms: {
    'darwin-arm64': {
      archive: 'chrome-mac-arm64.zip',
      url: `https://storage.googleapis.com/chrome-for-testing-public/${version}/mac-arm64/chrome-mac-arm64.zip`,
      sha256: sha('chrome-mac-arm64.zip'),
      executable_path: 'chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing',
    },
    'darwin-x64': {
      archive: 'chrome-mac-x64.zip',
      url: `https://storage.googleapis.com/chrome-for-testing-public/${version}/mac-x64/chrome-mac-x64.zip`,
      sha256: sha('chrome-mac-x64.zip'),
      executable_path: 'chrome-mac-x64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing',
    },
    'linux-x64': {
      archive: 'chrome-linux64.zip',
      url: `https://storage.googleapis.com/chrome-for-testing-public/${version}/linux64/chrome-linux64.zip`,
      sha256: sha('chrome-linux64.zip'),
      executable_path: 'chrome-linux64/chrome',
    },
    'win32-x64': {
      archive: 'chrome-win64.zip',
      url: `https://storage.googleapis.com/chrome-for-testing-public/${version}/win64/chrome-win64.zip`,
      sha256: sha('chrome-win64.zip'),
      executable_path: 'chrome-win64/chrome.exe',
    },
  },
};
fs.writeFileSync('browser-manifest.json', `${JSON.stringify(manifest, null, 2)}\n`);
NODE
```

Then copy it into the Go package:

```bash
cp browser-manifest.json internal/managedbrowser/browser-manifest.json
```

Create `internal/managedbrowser/manifest.go`:

```go
package managedbrowser

import (
	"embed"
	"encoding/json"
	"fmt"
	"runtime"
)

//go:embed browser-manifest.json
var manifestFS embed.FS

type Manifest struct {
	Version   string              `json:"version"`
	Platforms map[string]Platform `json:"platforms"`
}

type Platform struct {
	Archive        string `json:"archive"`
	URL            string `json:"url"`
	SHA256         string `json:"sha256"`
	ExecutablePath string `json:"executable_path"`
}

func BundledManifest() (Manifest, error) {
	body, err := manifestFS.ReadFile("browser-manifest.json")
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.Version == "" {
		return Manifest{}, fmt.Errorf("browser manifest version is empty")
	}
	if len(manifest.Platforms) == 0 {
		return Manifest{}, fmt.Errorf("browser manifest has no platforms")
	}
	return manifest, nil
}

func CurrentPlatformKey() string {
	return PlatformKey(runtime.GOOS, runtime.GOARCH)
}

func PlatformKey(goos, goarch string) string {
	switch goos + "/" + goarch {
	case "darwin/arm64":
		return "darwin-arm64"
	case "darwin/amd64":
		return "darwin-x64"
	case "linux/amd64":
		return "linux-x64"
	case "windows/amd64":
		return "win32-x64"
	default:
		return goos + "-" + goarch
	}
}

func (m Manifest) PlatformEntry(platform string) (Platform, error) {
	entry, ok := m.Platforms[platform]
	if !ok {
		return Platform{}, fmt.Errorf("unsupported managed browser platform: %s", platform)
	}
	return entry, nil
}
```

- [ ] **Step 5: Add browser manifest to npm package files**

Modify `package.json`:

```json
"files": [
  "bin/aget.js",
  "scripts",
  "browser-manifest.json",
  "README.md",
  "README.en.md",
  "LICENSE"
]
```

- [ ] **Step 6: Verify manifest tests pass**

Run:

```bash
go test -count=1 ./internal/managedbrowser -run 'TestBundledManifest|TestCurrentPlatformEntry'
```

Expected: PASS.

- [ ] **Step 7: Write failing cache tests**

Create `internal/managedbrowser/cache_test.go`:

```go
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
```

- [ ] **Step 8: Run cache tests and verify failure**

Run:

```bash
go test -count=1 ./internal/managedbrowser -run 'TestCache|TestInstallPaths|TestStatus'
```

Expected: FAIL because cache functions are not implemented.

- [ ] **Step 9: Implement cache model**

Create `internal/managedbrowser/cache.go`:

```go
package managedbrowser

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const CacheEnv = "AGET_BROWSER_CACHE_DIR"

type InstallPaths struct {
	CacheRoot  string `json:"cache_dir"`
	InstallDir string `json:"install_dir"`
	Executable string `json:"path"`
}

type InstallStatus struct {
	Installed  bool `json:"installed"`
	Executable bool `json:"executable"`
}

func CacheRoot() (string, error) {
	if override := os.Getenv(CacheEnv); override != "" {
		return override, nil
	}
	root, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return root, nil
}

func Paths(version, platform string, entry Platform) (InstallPaths, error) {
	root, err := CacheRoot()
	if err != nil {
		return InstallPaths{}, err
	}
	if version == "" || platform == "" || entry.ExecutablePath == "" {
		return InstallPaths{}, fmt.Errorf("browser install path requires version, platform, and executable path")
	}
	installDir := filepath.Join(root, "agent-aget", "chrome-for-testing", version, platform)
	return InstallPaths{
		CacheRoot:  root,
		InstallDir: installDir,
		Executable: filepath.Join(installDir, filepath.FromSlash(entry.ExecutablePath)),
	}, nil
}

func Status(paths InstallPaths) InstallStatus {
	info, err := os.Stat(paths.Executable)
	if err != nil || info.IsDir() {
		return InstallStatus{}
	}
	executable := true
	if runtime.GOOS != "windows" {
		executable = info.Mode().Perm()&0o111 != 0
	}
	return InstallStatus{Installed: true, Executable: executable}
}
```

- [ ] **Step 10: Verify managedbrowser tests pass**

Run:

```bash
go test -count=1 ./internal/managedbrowser
```

Expected: PASS.

- [ ] **Step 11: Commit manifest and cache model**

Run:

```bash
git add browser-manifest.json package.json internal/managedbrowser
git commit -m "feat: add managed browser manifest"
```

## Task 2: Strict Go Browser Installer

**Files:**

- Create: `internal/managedbrowser/install.go`
- Create: `internal/managedbrowser/install_test.go`

- [ ] **Step 1: Write failing install tests**

Create `internal/managedbrowser/install_test.go`:

```go
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
		Version: "148.0.7778.98",
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
			Archive: "chrome-linux64.zip",
			URL: server.URL,
			SHA256: hex.EncodeToString(sum[:]),
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
```

- [ ] **Step 2: Run install tests and verify failure**

Run:

```bash
go test -count=1 ./internal/managedbrowser -run TestInstall
```

Expected: FAIL because `Install` is undefined.

- [ ] **Step 3: Implement strict installer**

Create `internal/managedbrowser/install.go` with these exported contracts:

```go
package managedbrowser

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type InstallResult struct {
	OK               bool   `json:"ok"`
	Version          string `json:"version"`
	Platform         string `json:"platform"`
	Path             string `json:"path"`
	CacheDir         string `json:"cache_dir"`
	AlreadyInstalled bool   `json:"already_installed"`
}

func Install(ctx context.Context, manifest Manifest, platform string) (InstallResult, error) {
	entry, err := manifest.PlatformEntry(platform)
	if err != nil {
		return InstallResult{}, err
	}
	paths, err := Paths(manifest.Version, platform, entry)
	if err != nil {
		return InstallResult{}, err
	}
	if status := Status(paths); status.Installed && status.Executable {
		return InstallResult{OK: true, Version: manifest.Version, Platform: platform, Path: paths.Executable, CacheDir: paths.CacheRoot, AlreadyInstalled: true}, nil
	}
	tmp, err := os.MkdirTemp(paths.CacheRoot, "agent-aget-browser-")
	if err != nil {
		return InstallResult{}, err
	}
	defer os.RemoveAll(tmp)

	archivePath := filepath.Join(tmp, entry.Archive)
	if err := download(ctx, entry.URL, archivePath); err != nil {
		return InstallResult{}, fmt.Errorf("download browser archive: %w", err)
	}
	if err := verifySHA256(archivePath, entry.SHA256); err != nil {
		return InstallResult{}, err
	}
	extractDir := filepath.Join(tmp, "extract")
	if err := extractZip(archivePath, extractDir); err != nil {
		return InstallResult{}, fmt.Errorf("extract browser archive: %w", err)
	}
	staged := filepath.Join(tmp, "staged")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(staged, filepath.FromSlash(entry.ExecutablePath))), 0o700); err != nil {
		return InstallResult{}, err
	}
	if err := os.Rename(extractDir, staged); err != nil {
		return InstallResult{}, err
	}
	if err := os.RemoveAll(paths.InstallDir); err != nil {
		return InstallResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(paths.InstallDir), 0o700); err != nil {
		return InstallResult{}, err
	}
	if err := os.Rename(staged, paths.InstallDir); err != nil {
		return InstallResult{}, err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(paths.Executable, 0o755)
	}
	if status := Status(paths); !status.Installed || !status.Executable {
		return InstallResult{}, fmt.Errorf("browser executable validation failed: %s", paths.Executable)
	}
	return InstallResult{OK: true, Version: manifest.Version, Platform: platform, Path: paths.Executable, CacheDir: paths.CacheRoot}, nil
}
```

Add these helper functions in the same file:

```go
func download(ctx context.Context, url, destination string) error
func verifySHA256(path, expected string) error
func extractZip(archivePath, destination string) error
func safeZipPath(destination, name string) (string, error)
```

`download` must reject non-200 responses and follow Go's default redirect behavior. `verifySHA256` must compare lowercase hex. `safeZipPath` must reject entries that escape the destination directory by checking that the cleaned output path starts with the cleaned destination plus a path separator.

- [ ] **Step 4: Run install tests and fix compile/runtime failures**

Run:

```bash
go test -count=1 ./internal/managedbrowser -run TestInstall
```

Expected: PASS.

- [ ] **Step 5: Run all managedbrowser tests**

Run:

```bash
go test -count=1 ./internal/managedbrowser
```

Expected: PASS.

- [ ] **Step 6: Commit strict installer**

Run:

```bash
git add internal/managedbrowser
git commit -m "feat: add managed browser installer"
```

## Task 3: CLI Browser Commands and Resolver Priority

**Files:**

- Create: `internal/cli/browser.go`
- Create: `internal/cli/browser_test.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/browser/resolver.go`
- Modify: `internal/browser/resolver_test.go`

- [ ] **Step 1: Write failing resolver priority test**

Append to `internal/browser/resolver_test.go`:

```go
func TestResolveBinaryUsesManagedBrowserBeforeSystemCandidates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode executable checks differ on windows")
	}
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
```

- [ ] **Step 2: Run resolver test and verify failure**

Run:

```bash
go test -count=1 ./internal/browser -run TestResolveBinaryUsesManagedBrowserBeforeSystemCandidates
```

Expected: FAIL because `managedBrowserPath` is undefined.

- [ ] **Step 3: Implement managed browser resolver hook**

Modify `internal/browser/resolver.go`:

```go
import "github.com/izzzzzi/agent-aget/internal/managedbrowser"

var managedBrowserPath = func() (string, bool) {
	manifest, err := managedbrowser.BundledManifest()
	if err != nil {
		return "", false
	}
	entry, err := manifest.PlatformEntry(managedbrowser.CurrentPlatformKey())
	if err != nil {
		return "", false
	}
	paths, err := managedbrowser.Paths(manifest.Version, managedbrowser.CurrentPlatformKey(), entry)
	if err != nil {
		return "", false
	}
	status := managedbrowser.Status(paths)
	return paths.Executable, status.Installed && status.Executable
}
```

Call this hook after `AGET_BROWSER_PATH` and before `candidateNames()`:

```go
if path, ok := managedBrowserPath(); ok {
	return path, nil
}
```

Update `browserNotFoundMessage` to include the recovery command:

```go
const browserNotFoundMessage = "Chromium-compatible browser not found; run `aget browser install`, set --browser-path, or set AGET_BROWSER_PATH"
```

- [ ] **Step 4: Verify resolver tests pass**

Run:

```bash
go test -count=1 ./internal/browser
```

Expected: PASS.

- [ ] **Step 5: Write failing CLI browser tests**

Create `internal/cli/browser_test.go`:

```go
package cli

import (
	"encoding/json"
	"testing"

	"github.com/izzzzzi/agent-aget/internal/managedbrowser"
)

func TestBrowserStatusReportsMissingManagedBrowser(t *testing.T) {
	t.Setenv(managedbrowser.CacheEnv, t.TempDir())

	stdout, stderr, err := executeForTest("browser", "status")
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %v", got["ok"])
	}
	if got["installed"] != false {
		t.Fatalf("installed = %v, want false", got["installed"])
	}
}

func TestBrowserPathErrorsWhenManagedBrowserMissing(t *testing.T) {
	t.Setenv(managedbrowser.CacheEnv, t.TempDir())

	_, stderr, err := executeForTest("browser", "path")
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrorCodeJSON(t, stderr, "browser_not_installed")
}
```

- [ ] **Step 6: Run CLI browser tests and verify failure**

Run:

```bash
go test -count=1 ./internal/cli -run TestBrowser
```

Expected: FAIL because the `browser` command is not registered.

- [ ] **Step 7: Implement `aget browser` command group**

Create `internal/cli/browser.go` with commands:

```go
func newBrowserCommand() *cobra.Command
func newBrowserStatusCommand() *cobra.Command
func newBrowserPathCommand() *cobra.Command
func newBrowserInstallCommand() *cobra.Command
func currentManagedBrowser() (managedbrowser.Manifest, string, managedbrowser.Platform, managedbrowser.InstallPaths, error)
```

Register it in `internal/cli/root.go`:

```go
cmd.AddCommand(newVersionCommand(), newSessionCommand(), newOpenCommand(), newPageCommand(), newBrowserCommand())
```

Response details:

```go
// status
map[string]any{
  "ok": true,
  "version": manifest.Version,
  "platform": platform,
  "cache_dir": paths.CacheRoot,
  "path": paths.Executable,
  "installed": status.Installed,
  "executable": status.Executable,
}

// path missing error
writeError(cmd, "browser_not_installed", "managed browser is not installed", map[string]any{
  "recovery": "aget browser install",
  "cache_dir": paths.CacheRoot,
})
```

`browser install` calls `managedbrowser.Install(context.Background(), manifest, platform)` and returns its JSON result. Map unsupported platform to `browser_unsupported_platform`; map other install errors to `browser_install_failed`.

- [ ] **Step 8: Verify CLI browser tests pass**

Run:

```bash
go test -count=1 ./internal/cli -run TestBrowser
```

Expected: PASS.

- [ ] **Step 9: Run Go tests for affected packages**

Run:

```bash
go test -count=1 ./internal/browser ./internal/cli ./internal/managedbrowser
```

Expected: PASS.

- [ ] **Step 10: Commit CLI and resolver integration**

Run:

```bash
git add internal/browser internal/cli internal/managedbrowser
git commit -m "feat: add managed browser commands"
```

## Task 4: npm Best-Effort Browser Installer

**Files:**

- Create: `scripts/browser-install.js`
- Create: `scripts/browser-install.test.js`
- Modify: `scripts/install.js`
- Modify: `scripts/smoke-test.js`
- Modify: `package.json`

- [ ] **Step 1: Write failing Node browser installer tests**

Create `scripts/browser-install.test.js`:

```js
'use strict';

const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const assert = require('node:assert/strict');
const installer = require('./browser-install');

function main() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'aget-browser-test-'));
  process.env.AGET_BROWSER_CACHE_DIR = root;

  const manifest = {
    version: '148.0.7778.98',
    platforms: {
      'linux-x64': {
        archive: 'chrome-linux64.zip',
        url: 'https://example.invalid/chrome.zip',
        sha256: 'abc',
        executable_path: 'chrome-linux64/chrome',
      },
    },
  };

  const info = installer.pathsFor(manifest, 'linux-x64');
  assert.equal(info.cacheDir, root);
  assert.equal(
    info.installDir,
    path.join(root, 'agent-aget', 'chrome-for-testing', '148.0.7778.98', 'linux-x64'),
  );
  assert.equal(info.executable, path.join(info.installDir, 'chrome-linux64', 'chrome'));

  assert.equal(installer.isExecutable(info.executable), false);
  fs.mkdirSync(path.dirname(info.executable), { recursive: true });
  fs.writeFileSync(info.executable, '#!/bin/sh\n', { mode: 0o755 });
  assert.equal(installer.isExecutable(info.executable), true);

  fs.rmSync(root, { recursive: true, force: true });
  delete process.env.AGET_BROWSER_CACHE_DIR;
}

if (require.main === module) {
  main();
}
```

- [ ] **Step 2: Run Node test and verify failure**

Run:

```bash
node scripts/browser-install.test.js
```

Expected: FAIL because `scripts/browser-install.js` does not exist.

- [ ] **Step 3: Implement npm browser install module**

Create `scripts/browser-install.js` with exported functions:

```js
'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const https = require('node:https');
const os = require('node:os');
const path = require('node:path');
const { execFileSync } = require('node:child_process');

function cacheRoot() {
  if (process.env.AGET_BROWSER_CACHE_DIR) return process.env.AGET_BROWSER_CACHE_DIR;
  return path.join(os.homedir(), process.platform === 'darwin' ? 'Library/Caches' : process.platform === 'win32' ? 'AppData/Local' : '.cache');
}

function platformKey(platform = process.platform, arch = process.arch) {
  if (platform === 'darwin' && arch === 'arm64') return 'darwin-arm64';
  if (platform === 'darwin' && arch === 'x64') return 'darwin-x64';
  if (platform === 'linux' && arch === 'x64') return 'linux-x64';
  if (platform === 'linux' && arch === 'arm64') return 'linux-arm64';
  if (platform === 'win32' && arch === 'x64') return 'win32-x64';
  return `${platform}-${arch}`;
}

function pathsFor(manifest, key = platformKey()) {
  const entry = manifest.platforms[key];
  if (!entry) throw new Error(`unsupported managed browser platform: ${key}`);
  const root = cacheRoot();
  const installDir = path.join(root, 'agent-aget', 'chrome-for-testing', manifest.version, key);
  return {
    entry,
    cacheDir: root,
    installDir,
    executable: path.join(installDir, ...entry.executable_path.split('/')),
  };
}

function isExecutable(file) {
  try {
    const stat = fs.statSync(file);
    if (!stat.isFile()) return false;
    if (process.platform === 'win32') return true;
    return (stat.mode & 0o111) !== 0;
  } catch (_) {
    return false;
  }
}
```

Add `download`, `sha256`, `verifyChecksum`, `extractZip`, and `installFromManifest` to the same module. Reuse `powershellCommand()` from `scripts/install.js` for Windows zip extraction and `unzip -q` or PowerShell for other zip extraction. Export:

```js
module.exports = { cacheRoot, platformKey, pathsFor, isExecutable, installFromManifest };
```

When run directly, read `browser-manifest.json` from repo root and call `installFromManifest`.

- [ ] **Step 4: Verify Node browser installer tests pass**

Run:

```bash
node scripts/browser-install.test.js
```

Expected: PASS.

- [ ] **Step 5: Wire best-effort install into `scripts/install.js`**

Modify `scripts/install.js`:

```js
const browserInstall = require('./browser-install');
```

After native binary copy succeeds:

```js
if (process.env.AGET_SKIP_BROWSER_DOWNLOAD !== '1') {
  try {
    await browserInstall.installFromManifest();
  } catch (error) {
    console.warn(`agent-aget: managed browser install skipped: ${error.message}`);
    console.warn('agent-aget: run `aget browser install` to install it later');
  }
}
```

Keep `AGENT_AGET_SKIP_DOWNLOAD=1` returning before native and browser downloads.

- [ ] **Step 6: Add npm test script coverage**

Modify `package.json`:

```json
"scripts": {
  "test": "node scripts/smoke-test.js && node scripts/browser-install.test.js",
  "postinstall": "node scripts/install.js",
  "check": "test -z \"$(gofmt -l .)\" && go vet ./... && go test ./... && npm run smoke && npm run test:browser-install && npm pack --dry-run",
  "smoke": "node scripts/smoke-test.js",
  "test:browser-install": "node scripts/browser-install.test.js",
  "release:contract": "node scripts/release-contract-test.js",
  "pack:dry": "npm pack --dry-run"
}
```

- [ ] **Step 7: Verify npm tests pass without browser download**

Run:

```bash
AGET_SKIP_BROWSER_DOWNLOAD=1 npm run test:browser-install
AGENT_AGET_SKIP_DOWNLOAD=1 npm run smoke
```

Expected: both commands exit 0.

- [ ] **Step 8: Commit npm browser installer**

Run:

```bash
git add scripts package.json
git commit -m "feat: add npm managed browser install"
```

## Task 5: Docs, Release Contract, and Final Verification

**Files:**

- Modify: `scripts/release-contract-test.js`
- Modify: `README.md`
- Modify: `README.en.md`

- [ ] **Step 1: Write failing release contract assertions**

Modify `scripts/release-contract-test.js` so `verifyArtifactFiles` or a new `verifyPackageFiles` checks:

```js
function verifyPackageFiles() {
  const files = run('npm', ['pack', '--dry-run', '--json']);
  const parsed = JSON.parse(files);
  const names = parsed[0].files.map((file) => file.path);
  for (const required of ['browser-manifest.json', 'scripts/browser-install.js']) {
    if (!names.includes(required)) {
      throw new Error(`missing npm package file: ${required}`);
    }
  }
  const rootManifest = fs.readFileSync(path.join(root, 'browser-manifest.json'), 'utf8');
  const embeddedManifest = fs.readFileSync(path.join(root, 'internal/managedbrowser/browser-manifest.json'), 'utf8');
  if (rootManifest !== embeddedManifest) {
    throw new Error('root and embedded browser manifests differ');
  }
}
```

Call `verifyPackageFiles()` from `main()` before GoReleaser.

- [ ] **Step 2: Run release contract and verify failure or pass for the right reason**

Run:

```bash
npm run release:contract
```

Expected before packaging files are correct: FAIL with `missing npm package file`. Expected after Task 4 package changes: PASS through the new package-file check and continue to GoReleaser.

- [ ] **Step 3: Update README files**

In `README.md`, replace the manual-browser-only install section with:

````markdown
При `npm i -g agent-aget` пакет скачивает native `aget` и пытается установить pinned Chrome for Testing в пользовательский cache. Если сеть недоступна, установка пакета не падает; браузер можно поставить позже:

```bash
aget browser install
aget browser status
aget browser path
```

Порядок выбора браузера:

1. `--browser-path`
2. `AGET_BROWSER_PATH`
3. managed Chrome for Testing из cache
4. системный Chrome/Chromium

Чтобы пропустить установку managed browser:

```bash
AGET_SKIP_BROWSER_DOWNLOAD=1 npm i -g agent-aget
```
````

In `README.en.md`, add the English equivalent:

````markdown
During `npm i -g agent-aget`, the package downloads the native `aget` binary and tries to install pinned Chrome for Testing into the user cache. If the network is unavailable, package installation continues; install the browser later with:

```bash
aget browser install
aget browser status
aget browser path
```

Browser resolution order:

1. `--browser-path`
2. `AGET_BROWSER_PATH`
3. managed Chrome for Testing from cache
4. system Chrome/Chromium

To skip managed browser installation:

```bash
AGET_SKIP_BROWSER_DOWNLOAD=1 npm i -g agent-aget
```
````

- [ ] **Step 4: Verify docs and package contract**

Run:

```bash
npm pack --dry-run
npm run release:contract
```

Expected: `browser-manifest.json` and `scripts/browser-install.js` appear in package output; release contract exits 0.

- [ ] **Step 5: Run full verification**

Run:

```bash
test -z "$(gofmt -l cmd internal)" && go vet ./... && go test -count=1 ./... && go test -race -count=1 ./... && GOTOOLCHAIN=go1.22.12 go test -count=1 ./... && AGENT_AGET_SKIP_DOWNLOAD=1 npm run smoke && npm run test:browser-install && npm pack --dry-run && npm run release:contract
```

Expected: exit 0.

- [ ] **Step 6: Optional real browser install smoke**

Run only when network time and disk usage are acceptable:

```bash
AGET_BROWSER_CACHE_DIR=/tmp/aget-browser-install-smoke go run ./cmd/aget browser install
AGET_BROWSER_CACHE_DIR=/tmp/aget-browser-install-smoke go run ./cmd/aget browser status
AGET_BROWSER_CACHE_DIR=/tmp/aget-browser-install-smoke go run ./cmd/aget open https://example.com
```

Expected: install returns `ok:true`, status returns `installed:true`, and open resolves the managed browser without `--browser-path`.

- [ ] **Step 7: Commit docs and release contract**

Run:

```bash
git add README.md README.en.md scripts/release-contract-test.js
git commit -m "docs: document managed browser install"
```

## Final Completion Checklist

- [ ] `browser-manifest.json` uses exact sha256 values generated from downloaded archives.
- [ ] `internal/managedbrowser/browser-manifest.json` is byte-identical to `browser-manifest.json`.
- [ ] `npm pack --dry-run` includes `browser-manifest.json`.
- [ ] `aget browser status` never accesses the network.
- [ ] `aget browser path` never accesses the network.
- [ ] `aget open` does not download a browser implicitly.
- [ ] Explicit `--browser-path` and `AGET_BROWSER_PATH` still override managed browser.
- [ ] Full verification command exits 0.
