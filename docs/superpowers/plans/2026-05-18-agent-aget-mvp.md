# agent_aget MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first working `aget` CLI: a headless-by-default browser session tool for LLM agents with JSON responses, local sessions, page read/click/type/screenshot commands, and an npm wrapper.

**Architecture:** The Go CLI owns command parsing, JSON contracts, local state, session registry, browser process lifecycle, and page operations. CloakBrowser-compatible Chromium is launched as an external binary with a remote debugging port; page operations go through a small internal CDP interface backed by `chromedp` and test fakes.

**Tech Stack:** Go 1.22, Cobra, chromedp, Node 18+ for npm installer scripts, GoReleaser, GitHub Actions.

---

## File Structure

Create this structure:

```text
cmd/aget/main.go
internal/browser/launcher.go
internal/browser/launcher_test.go
internal/browser/resolver.go
internal/browser/resolver_test.go
internal/cdp/client.go
internal/cdp/chromedp.go
internal/cdp/fake_test.go
internal/cli/open.go
internal/cli/open_test.go
internal/cli/page.go
internal/cli/page_test.go
internal/cli/root.go
internal/cli/root_test.go
internal/cli/session.go
internal/cli/session_test.go
internal/cli/version.go
internal/ids/ids.go
internal/ids/ids_test.go
internal/page/service.go
internal/page/service_test.go
internal/response/response.go
internal/response/response_test.go
internal/session/registry.go
internal/session/registry_test.go
internal/state/dirs.go
internal/state/dirs_test.go
bin/aget.js
scripts/install.js
scripts/platform.js
scripts/release-contract-test.js
scripts/smoke-test.js
.github/dependabot.yml
.github/workflows/ci.yml
.github/workflows/release.yml
.gitignore
.goreleaser.yaml
.markdownlint-cli2.yaml
AGENT_INSTRUCTIONS.md
LICENSE
README.md
README.en.md
package.json
```

Responsibilities:

- `internal/cli`: Cobra command tree and command-level JSON output.
- `internal/response`: stable JSON encoding for success and error responses.
- `internal/state`: OS-specific state, artifact, and browser profile directories, with `AGET_STATE_DIR` override.
- `internal/ids`: short session ids.
- `internal/session`: registry files under the state directory.
- `internal/browser`: resolves the browser binary and starts/stops a trusted browser process.
- `internal/cdp`: browser/page control interface and `chromedp` implementation.
- `internal/page`: agent-friendly page operations using the CDP interface.
- `scripts` and `bin`: npm wrapper matching the `agent_ssh` installation model.

## Task 1: Project Foundation and JSON Contract

**Files:**

- Create: `go.mod`
- Create: `cmd/aget/main.go`
- Create: `internal/response/response.go`
- Create: `internal/response/response_test.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/root_test.go`
- Create: `internal/cli/version.go`

- [ ] **Step 1: Write failing response tests**

Create `internal/response/response_test.go`:

```go
package response

import (
	"encoding/json"
	"testing"
)

func TestMarshalAppendsNewline(t *testing.T) {
	body, err := Marshal(map[string]any{"ok": true, "sid": "abc12345"})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"ok\":true,\"sid\":\"abc12345\"}\n"
	if string(body) != want {
		t.Fatalf("body = %q, want %q", string(body), want)
	}
}

func TestMarshalErrorShape(t *testing.T) {
	body, err := MarshalError("session_not_found", "session missing", map[string]any{"sid": "abc12345"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != false {
		t.Fatalf("ok = %v, want false", got["ok"])
	}
	if got["code"] != "session_not_found" {
		t.Fatalf("code = %v", got["code"])
	}
	if got["message"] != "session missing" {
		t.Fatalf("message = %v", got["message"])
	}
}
```

- [ ] **Step 2: Run response tests and verify failure**

Run:

```bash
go test ./internal/response
```

Expected: FAIL because `go.mod` and `internal/response` do not exist.

- [ ] **Step 3: Add module and response implementation**

Create `go.mod`:

```go
module github.com/izzzzzi/agent-aget

go 1.22

require github.com/spf13/cobra v1.8.1
```

Create `internal/response/response.go`:

```go
package response

import "encoding/json"

type OK map[string]any

type Error struct {
	OK      bool           `json:"ok"`
	Code    string         `json:"code"`
	Message string         `json:"message,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func Marshal(v any) ([]byte, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func MarshalError(code, message string, details map[string]any) ([]byte, error) {
	return Marshal(Error{OK: false, Code: code, Message: message, Details: details})
}
```

- [ ] **Step 4: Verify response tests pass**

Run:

```bash
go test ./internal/response
```

Expected: PASS.

- [ ] **Step 5: Write failing CLI root tests**

Create `internal/cli/root_test.go`:

```go
package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func executeForTest(args ...string) (string, string, error) {
	cmd := NewRootCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestRootRequiresCommand(t *testing.T) {
	stdout, stderr, err := executeForTest()
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	var got map[string]any
	if json.Unmarshal([]byte(stderr), &got) != nil {
		t.Fatalf("stderr is not json: %q", stderr)
	}
	if got["code"] != "invalid_args" {
		t.Fatalf("code = %v", got["code"])
	}
}

func TestVersionCommand(t *testing.T) {
	stdout, stderr, err := executeForTest("version")
	if err != nil {
		t.Fatalf("version failed: %v stderr=%s", err, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %v", got["ok"])
	}
	if got["version"] == "" {
		t.Fatalf("version missing in %v", got)
	}
}
```

- [ ] **Step 6: Run CLI root tests and verify failure**

Run:

```bash
go test ./internal/cli
```

Expected: FAIL because `NewRootCommand` and `version` command do not exist.

- [ ] **Step 7: Add CLI root, version command, and main**

Create `internal/cli/root.go`:

```go
package cli

import (
	"errors"

	"github.com/izzzzzi/agent-aget/internal/response"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "aget",
		Short:         "Browser workflow helper for LLM agents",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return writeInvalidArgs(cmd, "unknown command "+args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeInvalidArgs(cmd, "command required")
		},
	}
	cmd.PersistentFlags().Bool("json", true, "emit JSON output")
	_ = cmd.PersistentFlags().MarkHidden("json")
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return writeInvalidArgs(cmd, err.Error())
	})
	cmd.AddCommand(newVersionCommand())
	return cmd
}

func Execute() error {
	return NewRootCommand().Execute()
}

func noPositionalArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return writeInvalidArgs(cmd, "unexpected positional arguments")
	}
	return nil
}

func writeInvalidArgs(cmd *cobra.Command, message string) error {
	return writeError(cmd, "invalid_args", message, map[string]any{"hint": "run aget --help"})
}

func writeJSON(cmd *cobra.Command, v any) error {
	body, err := response.Marshal(v)
	if err != nil {
		return err
	}
	_, _ = cmd.OutOrStdout().Write(body)
	return nil
}

func writeError(cmd *cobra.Command, code, message string, details map[string]any) error {
	body, marshalErr := response.MarshalError(code, message, details)
	if marshalErr != nil {
		return marshalErr
	}
	_, _ = cmd.ErrOrStderr().Write(body)
	return errors.New(message)
}
```

Create `internal/cli/version.go`:

```go
package cli

import "github.com/spf13/cobra"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version metadata",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeJSON(cmd, map[string]any{
				"ok":      true,
				"version": version,
				"commit":  commit,
				"date":    date,
			})
		},
	}
}
```

Create `cmd/aget/main.go`:

```go
package main

import (
	"os"

	"github.com/izzzzzi/agent-aget/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 8: Run foundation verification**

Run:

```bash
go mod tidy
gofmt -w cmd internal
go test ./...
go run ./cmd/aget version
```

Expected: all tests PASS and `go run` prints JSON containing `"ok":true`.

- [ ] **Step 9: Commit foundation**

Run:

```bash
git add go.mod go.sum cmd internal
git commit -m "feat: add aget cli foundation"
```

## Task 2: State Directories, IDs, and Session Registry

**Files:**

- Create: `internal/state/dirs.go`
- Create: `internal/state/dirs_test.go`
- Create: `internal/ids/ids.go`
- Create: `internal/ids/ids_test.go`
- Create: `internal/session/registry.go`
- Create: `internal/session/registry_test.go`

- [ ] **Step 1: Write failing state and id tests**

Create `internal/state/dirs_test.go`:

```go
package state

import (
	"path/filepath"
	"testing"
)

func TestBaseDirUsesOverride(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", filepath.Join("tmp", "aget-state"))
	if BaseDir() != filepath.Join("tmp", "aget-state") {
		t.Fatalf("BaseDir() = %q", BaseDir())
	}
}

func TestDerivedDirs(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", "/tmp/aget-test")
	if SessionsDir() != filepath.Join("/tmp/aget-test", "sessions") {
		t.Fatalf("SessionsDir() = %q", SessionsDir())
	}
	if ArtifactsDir() != filepath.Join("/tmp/aget-test", "artifacts") {
		t.Fatalf("ArtifactsDir() = %q", ArtifactsDir())
	}
	if ProfilesDir() != filepath.Join("/tmp/aget-test", "profiles") {
		t.Fatalf("ProfilesDir() = %q", ProfilesDir())
	}
}
```

Create `internal/ids/ids_test.go`:

```go
package ids

import "testing"

func TestNewSessionID(t *testing.T) {
	id, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 8 {
		t.Fatalf("len(id) = %d, want 8", len(id))
	}
	for _, r := range id {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Fatalf("id contains non-hex rune %q in %q", r, id)
		}
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/state ./internal/ids
```

Expected: FAIL because packages are missing.

- [ ] **Step 3: Implement state and ids**

Create `internal/state/dirs.go`:

```go
package state

import (
	"os"
	"path/filepath"
	"runtime"
)

func BaseDir() string {
	if override := os.Getenv("AGET_STATE_DIR"); override != "" {
		return override
	}
	switch runtime.GOOS {
	case "windows":
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "aget")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, "Library", "Application Support", "aget")
		}
	default:
		if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
			return filepath.Join(stateHome, "aget")
		}
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, ".local", "state", "aget")
		}
	}
	return "aget"
}

func SessionsDir() string {
	return filepath.Join(BaseDir(), "sessions")
}

func ArtifactsDir() string {
	return filepath.Join(BaseDir(), "artifacts")
}

func ProfilesDir() string {
	return filepath.Join(BaseDir(), "profiles")
}
```

Create `internal/ids/ids.go`:

```go
package ids

import (
	"crypto/rand"
	"encoding/hex"
)

func NewSessionID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
```

- [ ] **Step 4: Verify state and id tests pass**

Run:

```bash
gofmt -w internal/state internal/ids
go test ./internal/state ./internal/ids
```

Expected: PASS.

- [ ] **Step 5: Write failing registry tests**

Create `internal/session/registry_test.go`:

```go
package session

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistrySaveGetListDelete(t *testing.T) {
	registry := NewRegistry(filepath.Join(t.TempDir(), "sessions"))
	record := Record{
		SID:        "abc12345",
		Name:       "work",
		URL:        "https://example.com",
		Title:      "Example",
		BrowserPID: 123,
		DebugURL:   "http://127.0.0.1:9222",
		Headless:   true,
		CreatedAt:  time.Unix(10, 0).UTC(),
		UpdatedAt:  time.Unix(20, 0).UTC(),
	}
	if err := registry.Save(record); err != nil {
		t.Fatal(err)
	}
	got, err := registry.Get("abc12345")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "work" || got.URL != "https://example.com" || !got.Headless {
		t.Fatalf("unexpected record: %+v", got)
	}
	list, err := registry.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].SID != "abc12345" {
		t.Fatalf("list = %+v", list)
	}
	if err := registry.Delete("abc12345"); err != nil {
		t.Fatal(err)
	}
	_, err = registry.Get("abc12345")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
```

- [ ] **Step 6: Run registry tests and verify failure**

Run:

```bash
go test ./internal/session
```

Expected: FAIL because registry is missing.

- [ ] **Step 7: Implement registry**

Create `internal/session/registry.go`:

```go
package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ErrNotFound = errors.New("session not found")

type Record struct {
	SID        string    `json:"sid"`
	Name       string    `json:"name,omitempty"`
	URL        string    `json:"url"`
	Title      string    `json:"title,omitempty"`
	BrowserPID int       `json:"browser_pid"`
	DebugURL   string    `json:"debug_url"`
	Headless   bool      `json:"headless"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Registry struct {
	dir string
}

func NewRegistry(dir string) *Registry {
	return &Registry{dir: dir}
}

func (r *Registry) Save(record Record) error {
	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path(record.SID), append(body, '\n'), 0o600)
}

func (r *Registry) Get(sid string) (Record, error) {
	body, err := os.ReadFile(r.path(sid))
	if errors.Is(err, os.ErrNotExist) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(body, &record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (r *Registry) List() ([]Record, error) {
	entries, err := os.ReadDir(r.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []Record
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(r.dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var record Record
		if err := json.Unmarshal(body, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})
	return records, nil
}

func (r *Registry) Delete(sid string) error {
	err := os.Remove(r.path(sid))
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}

func (r *Registry) path(sid string) string {
	return filepath.Join(r.dir, sid+".json")
}
```

- [ ] **Step 8: Verify registry and all tests pass**

Run:

```bash
gofmt -w internal/session
go test ./...
```

Expected: PASS.

- [ ] **Step 9: Commit state and registry**

Run:

```bash
git add internal/state internal/ids internal/session
git commit -m "feat: add local session registry"
```

## Task 3: Browser Binary Resolver and Process Launcher

**Files:**

- Create: `internal/browser/resolver.go`
- Create: `internal/browser/resolver_test.go`
- Create: `internal/browser/launcher.go`
- Create: `internal/browser/launcher_test.go`

- [ ] **Step 1: Write failing resolver tests**

Create `internal/browser/resolver_test.go`:

```go
package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveBinaryUsesExplicitPath(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "cloak-browser")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveBinary(exe)
	if err != nil {
		t.Fatal(err)
	}
	if got != exe {
		t.Fatalf("got %q, want %q", got, exe)
	}
}

func TestResolveBinaryRejectsMissingExplicitPath(t *testing.T) {
	_, err := ResolveBinary(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run resolver tests and verify failure**

Run:

```bash
go test ./internal/browser -run Resolve
```

Expected: FAIL because `internal/browser` is missing.

- [ ] **Step 3: Implement resolver**

Create `internal/browser/resolver.go`:

```go
package browser

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func ResolveBinary(explicit string) (string, error) {
	if explicit != "" {
		return requireExecutable(explicit)
	}
	if fromEnv := os.Getenv("AGET_BROWSER_PATH"); fromEnv != "" {
		return requireExecutable(fromEnv)
	}
	for _, candidate := range candidateNames() {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", errors.New("CloakBrowser-compatible binary not found; set --browser-path or AGET_BROWSER_PATH")
}

func candidateNames() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"cloakbrowser.exe", "chrome.exe", "chromium.exe"}
	case "darwin":
		return []string{"cloakbrowser", "chromium", "google-chrome"}
	default:
		return []string{"cloakbrowser", "chromium-browser", "chromium", "google-chrome"}
	}
}

func requireExecutable(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("browser path is a directory: %s", path)
	}
	return path, nil
}
```

- [ ] **Step 4: Verify resolver tests pass**

Run:

```bash
gofmt -w internal/browser
go test ./internal/browser -run Resolve
```

Expected: PASS.

- [ ] **Step 5: Write failing launcher tests**

Create `internal/browser/launcher_test.go`:

```go
package browser

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildArgsHeadless(t *testing.T) {
	args := buildArgs(LaunchOptions{
		URL:         "https://example.com",
		UserDataDir: "/tmp/profile",
		Port:        9333,
		Headless:    true,
	})
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--remote-debugging-address=127.0.0.1",
		"--remote-debugging-port=9333",
		"--user-data-dir=/tmp/profile",
		"--headless=new",
		"https://example.com",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args %q missing %q", joined, want)
		}
	}
}

func TestFindFreePort(t *testing.T) {
	port, err := FindFreePort()
	if err != nil {
		t.Fatal(err)
	}
	if port <= 0 {
		t.Fatalf("port = %d", port)
	}
}

func TestLaunchWithFakeExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fake is unix-only")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-browser")
	logFile := filepath.Join(dir, "args.txt")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + logFile + "\nsleep 5\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	process, err := Launch(LaunchOptions{
		BinaryPath:  fake,
		URL:         "https://example.com",
		UserDataDir: filepath.Join(dir, "profile"),
		Port:        9334,
		Headless:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer process.Stop()
	if process.PID <= 0 {
		t.Fatalf("pid = %d", process.PID)
	}
}
```

- [ ] **Step 6: Run launcher tests and verify failure**

Run:

```bash
go test ./internal/browser -run 'BuildArgs|FindFreePort|Launch'
```

Expected: FAIL because launcher types and functions do not exist.

- [ ] **Step 7: Implement launcher**

Create `internal/browser/launcher.go`:

```go
package browser

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
)

type LaunchOptions struct {
	BinaryPath  string
	URL         string
	UserDataDir string
	Port        int
	Headless    bool
}

type Process struct {
	PID      int
	DebugURL string
	cmd      *exec.Cmd
}

func FindFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener address %T", listener.Addr())
	}
	return addr.Port, nil
}

func Launch(options LaunchOptions) (*Process, error) {
	if err := os.MkdirAll(options.UserDataDir, 0o700); err != nil {
		return nil, err
	}
	cmd := exec.Command(options.BinaryPath, buildArgs(options)...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &Process{
		PID:      cmd.Process.Pid,
		DebugURL: "http://127.0.0.1:" + strconv.Itoa(options.Port),
		cmd:      cmd,
	}, nil
}

func (p *Process) Stop() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	if err := p.cmd.Process.Kill(); err != nil {
		return err
	}
	_, _ = p.cmd.Process.Wait()
	return nil
}

func buildArgs(options LaunchOptions) []string {
	args := []string{
		"--remote-debugging-address=127.0.0.1",
		"--remote-debugging-port=" + strconv.Itoa(options.Port),
		"--user-data-dir=" + options.UserDataDir,
		"--no-first-run",
		"--no-default-browser-check",
	}
	if options.Headless {
		args = append(args, "--headless=new")
	}
	if options.URL != "" {
		args = append(args, options.URL)
	}
	return args
}
```

- [ ] **Step 8: Verify browser package tests pass**

Run:

```bash
gofmt -w internal/browser
go test ./internal/browser
```

Expected: PASS.

- [ ] **Step 9: Commit browser launcher**

Run:

```bash
git add internal/browser
git commit -m "feat: add browser launcher"
```

## Task 4: CDP Interface and Page Service

**Files:**

- Create: `internal/cdp/client.go`
- Create: `internal/cdp/chromedp.go`
- Create: `internal/cdp/fake_test.go`
- Create: `internal/page/service.go`
- Create: `internal/page/service_test.go`

- [ ] **Step 1: Write failing page service tests**

Create `internal/page/service_test.go`:

```go
package page

import (
	"context"
	"testing"

	"github.com/izzzzzi/agent-aget/internal/cdp"
)

func TestReadLimitsTextLines(t *testing.T) {
	driver := &cdp.FakeDriver{
		State: cdp.PageState{
			URL:   "https://example.com",
			Title: "Example",
			Text:  "one\ntwo\nthree\nfour",
			Links: []cdp.Element{{Selector: "a:nth-of-type(1)", Text: "More", Href: "https://example.com/more"}},
		},
	}
	service := NewService(driver)
	got, err := service.Read(context.Background(), ReadOptions{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != []string{"one", "two"} {
		t.Fatalf("text = %#v", got.Text)
	}
	if got.Truncated != true {
		t.Fatalf("truncated = %v", got.Truncated)
	}
	if len(got.Links) != 1 || got.Links[0].Text != "More" {
		t.Fatalf("links = %#v", got.Links)
	}
}

func TestClickAndTypeDelegateToDriver(t *testing.T) {
	driver := &cdp.FakeDriver{}
	service := NewService(driver)
	if err := service.Click(context.Background(), "#login"); err != nil {
		t.Fatal(err)
	}
	if err := service.Type(context.Background(), "#email", "me@example.com"); err != nil {
		t.Fatal(err)
	}
	if driver.Clicked != "#login" {
		t.Fatalf("clicked = %q", driver.Clicked)
	}
	if driver.TypedSelector != "#email" || driver.TypedText != "me@example.com" {
		t.Fatalf("typed selector/text = %q/%q", driver.TypedSelector, driver.TypedText)
	}
}
```

- [ ] **Step 2: Run page tests and verify failure**

Run:

```bash
go test ./internal/page
```

Expected: FAIL because page and cdp packages are missing.

- [ ] **Step 3: Add CDP interface and fake**

Create `internal/cdp/client.go`:

```go
package cdp

import "context"

type Element struct {
	Selector string `json:"selector"`
	Text     string `json:"text,omitempty"`
	Href     string `json:"href,omitempty"`
	Type     string `json:"type,omitempty"`
	Name     string `json:"name,omitempty"`
}

type PageState struct {
	URL     string    `json:"url"`
	Title   string    `json:"title"`
	Text    string    `json:"-"`
	Links   []Element `json:"links,omitempty"`
	Buttons []Element `json:"buttons,omitempty"`
	Inputs  []Element `json:"inputs,omitempty"`
}

type Driver interface {
	Read(ctx context.Context) (PageState, error)
	Click(ctx context.Context, selector string) error
	Type(ctx context.Context, selector, text string) error
	Screenshot(ctx context.Context, path string) error
	Close(ctx context.Context) error
}
```

Create `internal/cdp/fake_test.go`:

```go
package cdp

import "context"

type FakeDriver struct {
	State         PageState
	Clicked       string
	TypedSelector string
	TypedText     string
	ScreenshotPath string
	Closed        bool
}

func (f *FakeDriver) Read(ctx context.Context) (PageState, error) {
	return f.State, nil
}

func (f *FakeDriver) Click(ctx context.Context, selector string) error {
	f.Clicked = selector
	return nil
}

func (f *FakeDriver) Type(ctx context.Context, selector, text string) error {
	f.TypedSelector = selector
	f.TypedText = text
	return nil
}

func (f *FakeDriver) Screenshot(ctx context.Context, path string) error {
	f.ScreenshotPath = path
	return nil
}

func (f *FakeDriver) Close(ctx context.Context) error {
	f.Closed = true
	return nil
}
```

- [ ] **Step 4: Add page service**

Create `internal/page/service.go`:

```go
package page

import (
	"context"
	"strings"

	"github.com/izzzzzi/agent-aget/internal/cdp"
)

type Service struct {
	driver cdp.Driver
}

type ReadOptions struct {
	Limit int
}

type ReadResult struct {
	OK        bool          `json:"ok"`
	URL       string        `json:"url"`
	Title     string        `json:"title"`
	Text      []string      `json:"text"`
	Truncated bool          `json:"truncated"`
	Links     []cdp.Element `json:"links,omitempty"`
	Buttons   []cdp.Element `json:"buttons,omitempty"`
	Inputs    []cdp.Element `json:"inputs,omitempty"`
}

func NewService(driver cdp.Driver) *Service {
	return &Service{driver: driver}
}

func (s *Service) Read(ctx context.Context, options ReadOptions) (ReadResult, error) {
	state, err := s.driver.Read(ctx)
	if err != nil {
		return ReadResult{}, err
	}
	lines := compactLines(state.Text)
	limit := options.Limit
	if limit <= 0 {
		limit = 80
	}
	truncated := false
	if len(lines) > limit {
		lines = lines[:limit]
		truncated = true
	}
	return ReadResult{
		OK:        true,
		URL:       state.URL,
		Title:     state.Title,
		Text:      lines,
		Truncated: truncated,
		Links:     state.Links,
		Buttons:   state.Buttons,
		Inputs:    state.Inputs,
	}, nil
}

func (s *Service) Click(ctx context.Context, selector string) error {
	return s.driver.Click(ctx, selector)
}

func (s *Service) Type(ctx context.Context, selector, text string) error {
	return s.driver.Type(ctx, selector, text)
}

func (s *Service) Screenshot(ctx context.Context, path string) error {
	return s.driver.Screenshot(ctx, path)
}

func compactLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}
```

- [ ] **Step 5: Add chromedp dependency and implementation**

Run:

```bash
go get github.com/chromedp/chromedp@latest
```

Create `internal/cdp/chromedp.go`:

```go
package cdp

import (
	"context"

	"github.com/chromedp/chromedp"
)

type ChromeDPDriver struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func NewChromeDPDriver(parent context.Context, debugURL string) (*ChromeDPDriver, error) {
	allocatorCtx, allocatorCancel := chromedp.NewRemoteAllocator(parent, debugURL)
	ctx, cancel := chromedp.NewContext(allocatorCtx)
	return &ChromeDPDriver{
		ctx: ctx,
		cancel: func() {
			cancel()
			allocatorCancel()
		},
	}, nil
}

func (d *ChromeDPDriver) Read(ctx context.Context) (PageState, error) {
	var state PageState
	var text string
	if err := chromedp.Run(d.ctx,
		chromedp.Location(&state.URL),
		chromedp.Title(&state.Title),
		chromedp.Text("body", &text, chromedp.ByQuery),
	); err != nil {
		return PageState{}, err
	}
	state.Text = text
	return state, nil
}

func (d *ChromeDPDriver) Click(ctx context.Context, selector string) error {
	return chromedp.Run(d.ctx, chromedp.Click(selector, chromedp.ByQuery))
}

func (d *ChromeDPDriver) Type(ctx context.Context, selector, text string) error {
	return chromedp.Run(d.ctx, chromedp.SendKeys(selector, text, chromedp.ByQuery))
}

func (d *ChromeDPDriver) Screenshot(ctx context.Context, path string) error {
	var body []byte
	if err := chromedp.Run(d.ctx, chromedp.FullScreenshot(&body, 90)); err != nil {
		return err
	}
	return writeScreenshot(path, body)
}

func (d *ChromeDPDriver) Close(ctx context.Context) error {
	d.cancel()
	return nil
}
```

Add helper to the same file:

```go
func writeScreenshot(path string, body []byte) error {
	return os.WriteFile(path, body, 0o600)
}
```

Add `os` to the import list:

```go
import (
	"context"
	"os"

	"github.com/chromedp/chromedp"
)
```

- [ ] **Step 6: Verify page tests pass**

Run:

```bash
gofmt -w internal/cdp internal/page
go test ./internal/page
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit CDP and page service**

Run:

```bash
git add go.mod go.sum internal/cdp internal/page
git commit -m "feat: add page control service"
```

## Task 5: CLI Commands for Sessions and Pages

**Files:**

- Create: `internal/cli/session.go`
- Create: `internal/cli/session_test.go`
- Create: `internal/cli/page.go`
- Create: `internal/cli/page_test.go`
- Create: `internal/cli/open.go`
- Create: `internal/cli/open_test.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Write failing session command tests**

Create `internal/cli/session_test.go`:

```go
package cli

import (
	"encoding/json"
	"testing"
)

func TestSessionListEmpty(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	stdout, stderr, err := executeForTest("session", "list")
	if err != nil {
		t.Fatalf("session list failed: %v stderr=%s", err, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %v", got["ok"])
	}
	if len(got["sessions"].([]any)) != 0 {
		t.Fatalf("sessions = %v", got["sessions"])
	}
}
```

- [ ] **Step 2: Run session command tests and verify failure**

Run:

```bash
go test ./internal/cli -run Session
```

Expected: FAIL because session command is not wired.

- [ ] **Step 3: Implement session list and wire root**

Create `internal/cli/session.go`:

```go
package cli

import (
	"errors"

	sessionstore "github.com/izzzzzi/agent-aget/internal/session"
	"github.com/izzzzzi/agent-aget/internal/state"
	"github.com/spf13/cobra"
)

func newSessionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage browser sessions",
		Args:  noPositionalArgs,
	}
	cmd.AddCommand(newSessionListCommand(), newSessionCloseCommand(), newSessionGCCommand())
	return cmd
}

func newSessionListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List sessions",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			records, err := sessionstore.NewRegistry(state.SessionsDir()).List()
			if err != nil {
				return writeError(cmd, "session_list_failed", err.Error(), nil)
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sessions": records})
		},
	}
}

func newSessionCloseCommand() *cobra.Command {
	var sid string
	cmd := &cobra.Command{
		Use:   "close",
		Short: "Close a session",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sid == "" {
				return writeInvalidArgs(cmd, "--sid is required")
			}
			registry := sessionstore.NewRegistry(state.SessionsDir())
			if err := registry.Delete(sid); err != nil {
				if errors.Is(err, sessionstore.ErrNotFound) {
					return writeError(cmd, "session_not_found", "session not found", map[string]any{"sid": sid})
				}
				return writeError(cmd, "session_close_failed", err.Error(), map[string]any{"sid": sid})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	return cmd
}

func newSessionGCCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "gc",
		Short: "Clean stale sessions",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeJSON(cmd, map[string]any{"ok": true, "removed": []string{}})
		},
	}
}
```

Modify `internal/cli/root.go` to include session command:

```go
cmd.AddCommand(
	newVersionCommand(),
	newSessionCommand(),
)
```

- [ ] **Step 4: Verify session command test passes**

Run:

```bash
gofmt -w internal/cli
go test ./internal/cli -run Session
```

Expected: PASS.

- [ ] **Step 5: Write failing open and page command tests**

Create `internal/cli/open_test.go`:

```go
package cli

import (
	"encoding/json"
	"testing"
)

func TestOpenRequiresURL(t *testing.T) {
	stdout, stderr, err := executeForTest("open")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q", stdout)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stderr), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != "invalid_args" {
		t.Fatalf("code = %v", got["code"])
	}
}
```

Create `internal/cli/page_test.go`:

```go
package cli

import (
	"encoding/json"
	"testing"
)

func TestPageReadRequiresSID(t *testing.T) {
	stdout, stderr, err := executeForTest("page", "read")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q", stdout)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stderr), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != "invalid_args" {
		t.Fatalf("code = %v", got["code"])
	}
}
```

- [ ] **Step 6: Run open/page command tests and verify failure**

Run:

```bash
go test ./internal/cli -run 'Open|Page'
```

Expected: FAIL because commands are missing.

- [ ] **Step 7: Implement open and page commands with working error paths**

Create `internal/cli/open.go`:

```go
package cli

import (
	"path/filepath"
	"time"

	"github.com/izzzzzi/agent-aget/internal/browser"
	"github.com/izzzzzi/agent-aget/internal/ids"
	sessionstore "github.com/izzzzzi/agent-aget/internal/session"
	"github.com/izzzzzi/agent-aget/internal/state"
	"github.com/spf13/cobra"
)

func newOpenCommand() *cobra.Command {
	var name string
	var headful bool
	var browserPath string
	cmd := &cobra.Command{
		Use:   "open URL",
		Short: "Open a URL in a browser session",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return writeInvalidArgs(cmd, "open requires exactly one URL")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			binary, err := browser.ResolveBinary(browserPath)
			if err != nil {
				return writeError(cmd, "browser_not_found", err.Error(), nil)
			}
			sid, err := ids.NewSessionID()
			if err != nil {
				return writeError(cmd, "sid_failed", err.Error(), nil)
			}
			port, err := browser.FindFreePort()
			if err != nil {
				return writeError(cmd, "port_failed", err.Error(), nil)
			}
			process, err := browser.Launch(browser.LaunchOptions{
				BinaryPath:  binary,
				URL:         args[0],
				UserDataDir: filepath.Join(state.ProfilesDir(), sid),
				Port:        port,
				Headless:    !headful,
			})
			if err != nil {
				return writeError(cmd, "browser_start_failed", err.Error(), nil)
			}
			now := time.Now().UTC()
			record := sessionstore.Record{
				SID:        sid,
				Name:       name,
				URL:        args[0],
				BrowserPID: process.PID,
				DebugURL:   process.DebugURL,
				Headless:   !headful,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			if err := sessionstore.NewRegistry(state.SessionsDir()).Save(record); err != nil {
				_ = process.Stop()
				return writeError(cmd, "session_save_failed", err.Error(), map[string]any{"sid": sid})
			}
			return writeJSON(cmd, map[string]any{
				"ok":      true,
				"sid":     sid,
				"session": name,
				"browser": map[string]any{"headless": !headful},
				"page":    map[string]any{"url": args[0]},
				"next_commands": map[string]string{
					"read":       "aget page read -s " + sid + " --limit 80",
					"click":      "aget page click -s " + sid + " --selector SELECTOR",
					"type":       "aget page type -s " + sid + " --selector SELECTOR --text TEXT",
					"screenshot": "aget page screenshot -s " + sid,
					"close":      "aget session close -s " + sid,
				},
			})
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "session name")
	cmd.Flags().BoolVar(&headful, "headful", false, "run a visible browser")
	cmd.Flags().StringVar(&browserPath, "browser-path", "", "CloakBrowser-compatible binary path")
	return cmd
}
```

Create `internal/cli/page.go`:

```go
package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/izzzzzi/agent-aget/internal/cdp"
	pagesvc "github.com/izzzzzi/agent-aget/internal/page"
	sessionstore "github.com/izzzzzi/agent-aget/internal/session"
	"github.com/izzzzzi/agent-aget/internal/state"
	"github.com/spf13/cobra"
)

func newPageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "Interact with the active page",
		Args:  noPositionalArgs,
	}
	cmd.AddCommand(newPageReadCommand(), newPageClickCommand(), newPageTypeCommand(), newPageScreenshotCommand())
	return cmd
}

func requireSID(cmd *cobra.Command, sid string) error {
	if sid == "" {
		return writeInvalidArgs(cmd, "--sid is required")
	}
	return nil
}

func pageServiceForSession(sid string) (*pagesvc.Service, error) {
	record, err := sessionstore.NewRegistry(state.SessionsDir()).Get(sid)
	if err != nil {
		return nil, err
	}
	driver, err := cdp.NewChromeDPDriver(context.Background(), record.DebugURL)
	if err != nil {
		return nil, err
	}
	return pagesvc.NewService(driver), nil
}

func writePageSessionError(cmd *cobra.Command, sid string, err error) error {
	if errors.Is(err, sessionstore.ErrNotFound) {
		return writeError(cmd, "session_not_found", "session not found", map[string]any{"sid": sid})
	}
	return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
}

func newPageReadCommand() *cobra.Command {
	var sid string
	var limit int
	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read current page state",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireSID(cmd, sid); err != nil {
				return err
			}
			service, err := pageServiceForSession(sid)
			if err != nil {
				return writePageSessionError(cmd, sid, err)
			}
			result, err := service.Read(context.Background(), pagesvc.ReadOptions{Limit: limit})
			if err != nil {
				return writeError(cmd, "page_read_failed", err.Error(), map[string]any{"sid": sid})
			}
			return writeJSON(cmd, result)
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().IntVar(&limit, "limit", 80, "maximum text lines")
	return cmd
}

func newPageClickCommand() *cobra.Command {
	var sid string
	var selector string
	cmd := &cobra.Command{
		Use:   "click",
		Short: "Click an element",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireSID(cmd, sid); err != nil {
				return err
			}
			if selector == "" {
				return writeInvalidArgs(cmd, "--selector is required")
			}
			service, err := pageServiceForSession(sid)
			if err != nil {
				return writePageSessionError(cmd, sid, err)
			}
			if err := service.Click(context.Background(), selector); err != nil {
				return writeError(cmd, "page_click_failed", err.Error(), map[string]any{"sid": sid, "selector": selector})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid, "selector": selector})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "CSS selector")
	return cmd
}

func newPageTypeCommand() *cobra.Command {
	var sid string
	var selector string
	var text string
	cmd := &cobra.Command{
		Use:   "type",
		Short: "Type text into an element",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireSID(cmd, sid); err != nil {
				return err
			}
			if selector == "" {
				return writeInvalidArgs(cmd, "--selector is required")
			}
			service, err := pageServiceForSession(sid)
			if err != nil {
				return writePageSessionError(cmd, sid, err)
			}
			if err := service.Type(context.Background(), selector, text); err != nil {
				return writeError(cmd, "page_type_failed", err.Error(), map[string]any{"sid": sid, "selector": selector})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid, "selector": selector, "text_len": len(text)})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&selector, "selector", "", "CSS selector")
	cmd.Flags().StringVar(&text, "text", "", "text to type")
	return cmd
}

func newPageScreenshotCommand() *cobra.Command {
	var sid string
	var path string
	cmd := &cobra.Command{
		Use:   "screenshot",
		Short: "Capture a screenshot",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireSID(cmd, sid); err != nil {
				return err
			}
			if path == "" {
				path = filepath.Join(state.ArtifactsDir(), sid+"-"+time.Now().UTC().Format("20060102T150405Z")+".png")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return writeError(cmd, "screenshot_dir_failed", err.Error(), map[string]any{"path": path})
			}
			service, err := pageServiceForSession(sid)
			if err != nil {
				return writePageSessionError(cmd, sid, err)
			}
			if err := service.Screenshot(context.Background(), path); err != nil {
				return writeError(cmd, "screenshot_failed", err.Error(), map[string]any{"sid": sid, "path": path})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid, "path": path})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().StringVar(&path, "path", "", "screenshot output path")
	return cmd
}
```

Modify `internal/cli/root.go`:

```go
cmd.AddCommand(
	newVersionCommand(),
	newSessionCommand(),
	newOpenCommand(),
	newPageCommand(),
)
```

- [ ] **Step 8: Verify command validation passes**

Run:

```bash
gofmt -w internal/cli
go test ./internal/cli
go test ./...
```

Expected: PASS.

- [ ] **Step 9: Commit command surface**

Run:

```bash
git add internal/cli
git commit -m "feat: add browser command surface"
```

## Task 6: Command Error Contract Tests

**Files:**

- Test: `internal/cli/open_test.go`
- Test: `internal/cli/page_test.go`

- [ ] **Step 1: Extend open tests for missing browser path**

Add to `internal/cli/open_test.go`:

```go
func TestOpenWithMissingBrowserReturnsJSONError(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	stdout, stderr, err := executeForTest("open", "https://example.com", "--browser-path", "/missing/browser")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q", stdout)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stderr), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != "browser_not_found" {
		t.Fatalf("code = %v", got["code"])
	}
}
```

- [ ] **Step 2: Add page missing-session test**

Add to `internal/cli/page_test.go`:

```go
func TestPageReadMissingSession(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	stdout, stderr, err := executeForTest("page", "read", "-s", "missing1")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q", stdout)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stderr), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != "session_not_found" {
		t.Fatalf("code = %v", got["code"])
	}
}
```

- [ ] **Step 3: Verify command error tests pass**

Run:

```bash
go test ./internal/cli -run 'Open|Page'
```

Expected: PASS.

- [ ] **Step 4: Run full tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit command error tests**

Run:

```bash
git add internal/cli/open_test.go internal/cli/page_test.go
git commit -m "test: cover command error contracts"
```

## Task 7: Integration Smoke Test with Optional Browser

**Files:**

- Create: `internal/integration/browser_test.go`

- [ ] **Step 1: Add optional integration test**

Create `internal/integration/browser_test.go`:

```go
package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBrowserSessionSmoke(t *testing.T) {
	browserPath := os.Getenv("AGET_BROWSER_PATH")
	if browserPath == "" {
		t.Skip("AGET_BROWSER_PATH is not set")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Smoke</title></head><body><button id="ok">OK</button><p>Hello browser</p></body></html>`))
	}))
	defer server.Close()

	stateDir := t.TempDir()
	open := exec.Command("go", "run", "./cmd/aget", "open", server.URL, "--browser-path", browserPath)
	open.Dir = filepath.Join("..", "..")
	open.Env = append(os.Environ(), "AGET_STATE_DIR="+stateDir)
	openOut, err := open.Output()
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	var opened map[string]any
	if err := json.Unmarshal(openOut, &opened); err != nil {
		t.Fatal(err)
	}
	sid := opened["sid"].(string)

	read := exec.Command("go", "run", "./cmd/aget", "page", "read", "-s", sid, "--limit", "10")
	read.Dir = filepath.Join("..", "..")
	read.Env = append(os.Environ(), "AGET_STATE_DIR="+stateDir)
	readOut, err := read.Output()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !json.Valid(readOut) {
		t.Fatalf("read output is not json: %s", readOut)
	}
}
```

- [ ] **Step 2: Run integration test without browser**

Run:

```bash
go test ./internal/integration
```

Expected: PASS with skip message because `AGET_BROWSER_PATH` is not set.

- [ ] **Step 3: Run full tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit integration smoke**

Run:

```bash
git add internal/integration
git commit -m "test: add optional browser smoke test"
```

## Task 8: npm Wrapper, Release Config, CI, and Docs

**Files:**

- Create: `package.json`
- Create: `bin/aget.js`
- Create: `scripts/platform.js`
- Create: `scripts/install.js`
- Create: `scripts/smoke-test.js`
- Create: `scripts/release-contract-test.js`
- Create: `.goreleaser.yaml`
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`
- Create: `.github/dependabot.yml`
- Create: `.gitignore`
- Create: `.markdownlint-cli2.yaml`
- Create: `README.md`
- Create: `README.en.md`
- Create: `AGENT_INSTRUCTIONS.md`
- Create: `LICENSE`

- [ ] **Step 1: Add npm package metadata**

Create `package.json`:

```json
{
  "name": "agent-aget",
  "version": "0.1.0",
  "description": "Browser workflow helper for LLM agents",
  "license": "MIT",
  "repository": {
    "type": "git",
    "url": "git+https://github.com/izzzzzi/agent-aget.git"
  },
  "keywords": [
    "browser",
    "llm",
    "agent",
    "cli",
    "cdp"
  ],
  "bin": {
    "aget": "bin/aget.js"
  },
  "scripts": {
    "test": "node scripts/smoke-test.js",
    "postinstall": "node scripts/install.js",
    "check": "test -z \"$(gofmt -l .)\" && go vet ./... && go test ./... && npm run smoke && npm pack --dry-run",
    "smoke": "node scripts/smoke-test.js",
    "release:contract": "node scripts/release-contract-test.js",
    "pack:dry": "npm pack --dry-run"
  },
  "engines": {
    "node": ">=18"
  },
  "files": [
    "bin/aget.js",
    "scripts",
    "README.md",
    "README.en.md",
    "LICENSE"
  ]
}
```

- [ ] **Step 2: Add npm wrapper scripts**

Create `scripts/platform.js`:

```js
'use strict';

function target(platform = process.platform, arch = process.arch) {
  const osMap = { linux: 'linux', darwin: 'darwin', win32: 'windows' };
  const archMap = { x64: 'amd64', arm64: 'arm64' };
  const os = osMap[platform];
  const mappedArch = archMap[arch];
  if (!os) {
    throw new Error(`Unsupported platform: ${platform}`);
  }
  if (!mappedArch) {
    throw new Error(`Unsupported architecture: ${arch}`);
  }
  if (os === 'windows' && mappedArch === 'arm64') {
    throw new Error('Unsupported platform: windows/arm64');
  }
  return {
    os,
    arch: mappedArch,
    ext: os === 'windows' ? '.exe' : '',
    archiveExt: os === 'windows' ? '.zip' : '.tar.gz',
  };
}

module.exports = { target };
```

Create `bin/aget.js`:

```js
#!/usr/bin/env node
'use strict';

const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { target } = require('../scripts/platform');

const info = target();
const binary = path.join(__dirname, '..', 'native', `aget${info.ext}`);
const result = spawnSync(binary, process.argv.slice(2), { stdio: 'inherit' });

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}

process.exit(result.status === null ? 1 : result.status);
```

Create `scripts/install.js` by copying `agent_ssh/scripts/install.js` and replacing:

```text
assh -> aget
agent-assh -> agent-aget
github.com/izzzzzi/agent-assh -> github.com/izzzzzi/agent-aget
AGENT_ASSH_SKIP_DOWNLOAD -> AGENT_AGET_SKIP_DOWNLOAD
```

- [ ] **Step 3: Add smoke and release contract scripts**

Create `scripts/release-contract-test.js` by copying `agent_ssh/scripts/release-contract-test.js` and replacing:

```text
assh -> aget
agent-assh -> agent-aget
```

Create `scripts/smoke-test.js` by copying `agent_ssh/scripts/smoke-test.js` and replacing:

```text
assh -> aget
agent-assh -> agent-aget
```

- [ ] **Step 4: Add GoReleaser config**

Create `.goreleaser.yaml`:

```yaml
version: 2

project_name: aget

before:
  hooks:
    - go mod tidy

builds:
  - id: aget
    main: ./cmd/aget
    binary: aget
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w
      - -X github.com/izzzzzi/agent-aget/internal/cli.version={{.Version}}
      - -X github.com/izzzzzi/agent-aget/internal/cli.commit={{.Commit}}
      - -X github.com/izzzzzi/agent-aget/internal/cli.date={{.Date}}

archives:
  - id: default
    ids:
      - aget
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}
    format_overrides:
      - goos: windows
        formats: ["zip"]

checksum:
  name_template: checksums.txt

release:
  github:
    owner: izzzzzi
    name: agent-aget

snapshot:
  version_template: "{{ .Version }}"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
```

- [ ] **Step 5: Add CI and release workflows**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches:
      - main
  pull_request:

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version: "1.22"
      - run: test -z "$(gofmt -l .)"
      - run: go vet ./...
      - run: go test ./...

  npm-smoke:
    name: npm Smoke
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-node@v6
        with:
          node-version: "22"
      - run: npm run smoke
      - run: npm pack --dry-run
```

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v6
        with:
          go-version: "1.22"
      - uses: actions/setup-node@v6
        with:
          node-version: "22"
          registry-url: https://registry.npmjs.org
      - name: Verify npm version matches tag
        run: 'test "v$(node -p "require(''./package.json'').version")" = "$GITHUB_REF_NAME"'
      - uses: goreleaser/goreleaser-action@v7
        with:
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      - run: npm publish --access public
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
```

Create `.github/dependabot.yml`:

```yaml
version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
  - package-ecosystem: "npm"
    directory: "/"
    schedule:
      interval: "weekly"
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
```

- [ ] **Step 6: Add docs and housekeeping**

Create `.gitignore`:

```gitignore
dist/
native/
node_modules/
.DS_Store
coverage.out
```

Create `.markdownlint-cli2.yaml`:

```yaml
config:
  MD013: false
```

Create `README.md` with Russian quick start and command examples from the design document. Create `README.en.md` with the same commands in English. Create `AGENT_INSTRUCTIONS.md` with a short instruction block:

```markdown
# Agent Instructions

Use `aget` for browser work.

Start with `aget open URL -n NAME`. Use the returned `sid` for `aget page read`, `aget page click`, `aget page type`, `aget page screenshot`, and `aget session close`.

Do not paste secrets into examples or logs. Put secrets into environment variables and pass them to commands only when needed.
```

Create `LICENSE` with the MIT license text and copyright owner `izzzzzi`.

- [ ] **Step 7: Run release and package checks**

Run:

```bash
gofmt -w cmd internal
go mod tidy
go test ./...
npm run smoke
npm pack --dry-run
npm run release:contract
```

Expected: PASS. `release:contract` may download GoReleaser; if network is unavailable, record the failure output and run the rest.

- [ ] **Step 8: Commit release scaffolding**

Run:

```bash
git add .github .gitignore .goreleaser.yaml .markdownlint-cli2.yaml AGENT_INSTRUCTIONS.md LICENSE README.md README.en.md bin package.json scripts go.mod go.sum
git commit -m "chore: add release and npm packaging"
```

## Final Verification

- [ ] Run:

```bash
gofmt -w cmd internal
go vet ./...
go test ./...
npm run smoke
npm pack --dry-run
git status --short
```

Expected:

- `gofmt` has no remaining changes after formatting.
- `go vet ./...` exits 0.
- `go test ./...` exits 0.
- `npm run smoke` exits 0.
- `npm pack --dry-run` exits 0.
- `git status --short` shows only intentional uncommitted files, or nothing after all commits.

## Self-Review

Spec coverage:

- Separate Go CLI: Task 1.
- Headless default and `--headful`: Tasks 3, 5, and 6.
- CloakBrowser-compatible external binary: Task 3.
- CDP command execution: Task 4.
- Persistent session registry: Task 2 and Task 6.
- `open`, `page read`, `page click`, `page type`, `page screenshot`, `session list|close|gc`: Tasks 5 and 6.
- npm wrapper and cross-platform release model: Task 8.
- Optional browser-dependent integration test: Task 7.

Known MVP limitation captured intentionally: `session close` removes registry metadata but does not yet gracefully terminate a previously launched process by PID on every platform. That can be improved after the first end-to-end browser workflow is stable.
