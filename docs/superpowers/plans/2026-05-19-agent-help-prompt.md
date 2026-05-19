# Agent Help Prompt Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace dead-end disabled help with JSON agent help and add `aget prompt` / `aget agent-instructions` for LLM-agent usage instructions.

**Architecture:** Add a small `internal/agenthelp` package that owns help and prompt payloads. Wire Cobra help handling to emit JSON through the existing response helpers, keep operational errors on stderr, and add prompt commands without changing existing operational command responses.

**Tech Stack:** Go 1.22, Cobra, existing `internal/response` JSON helpers, Node npm packaging scripts.

---

## File Structure

Create:

```text
internal/agenthelp/help.go
internal/agenthelp/help_test.go
internal/cli/prompt.go
internal/cli/prompt_test.go
```

Modify:

```text
internal/cli/browser.go
internal/cli/open.go
internal/cli/page.go
internal/cli/root.go
internal/cli/root_test.go
internal/cli/session.go
internal/cli/version.go
internal/cli/page_test.go
scripts/release-contract-test.js
README.md
README.en.md
package.json
```

Responsibilities:

- `internal/agenthelp`: centralizes agent help and prompt payloads.
- `internal/cli/root.go`: installs Cobra JSON help handling, writes JSON help to stdout, updates invalid-args hints.
- `internal/cli/prompt.go`: registers `prompt` and `agent-instructions` commands.
- `internal/cli/{browser,open,page,session,version}.go`: replace the old disabled-help setup with JSON agent help setup.
- README files: document JSON agent help and prompt commands.
- release contract/package metadata: ensure advertised `AGENT_INSTRUCTIONS.md` ships in npm package.

## Task 1: Agent Help Payloads

**Files:**

- Create: `internal/agenthelp/help.go`
- Create: `internal/agenthelp/help_test.go`

- [ ] **Step 1: Write failing package tests**

Create `internal/agenthelp/help_test.go`:

```go
package agenthelp

import "testing"

func TestRootHelpPayload(t *testing.T) {
	payload := RootHelp()
	if payload.OK != true {
		t.Fatalf("OK = %v, want true", payload.OK)
	}
	if payload.Tool != "aget" {
		t.Fatalf("Tool = %q, want aget", payload.Tool)
	}
	if payload.Audience != "llm_agent" {
		t.Fatalf("Audience = %q, want llm_agent", payload.Audience)
	}
	if payload.Kind != "agent_help" {
		t.Fatalf("Kind = %q, want agent_help", payload.Kind)
	}
	if payload.AgentPromptCommand != "aget prompt" {
		t.Fatalf("AgentPromptCommand = %q", payload.AgentPromptCommand)
	}
	for _, key := range []string{"browser_status", "open", "page_read", "session_close", "prompt"} {
		if payload.Commands[key] == "" {
			t.Fatalf("Commands[%q] missing", key)
		}
	}
}

func TestGroupHelpPayload(t *testing.T) {
	payload, ok := GroupHelp("page")
	if !ok {
		t.Fatal("GroupHelp(page) not found")
	}
	if payload.CommandGroup != "page" {
		t.Fatalf("CommandGroup = %q, want page", payload.CommandGroup)
	}
	for _, key := range []string{"read", "click", "type", "screenshot"} {
		if payload.Commands[key] == "" {
			t.Fatalf("Commands[%q] missing", key)
		}
	}
}

func TestGroupHelpUnknown(t *testing.T) {
	if _, ok := GroupHelp("missing"); ok {
		t.Fatal("GroupHelp(missing) ok = true, want false")
	}
}

func TestPromptPayload(t *testing.T) {
	payload := Prompt()
	if payload.OK != true {
		t.Fatalf("OK = %v, want true", payload.OK)
	}
	if payload.Kind != "agent_prompt" {
		t.Fatalf("Kind = %q, want agent_prompt", payload.Kind)
	}
	if payload.Prompt == "" {
		t.Fatal("Prompt is empty")
	}
	for _, want := range []string{"JSON", "aget open URL", "sid", "aget page read", "aget session close"} {
		if !contains(payload.Prompt, want) {
			t.Fatalf("Prompt missing %q: %s", want, payload.Prompt)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && index(s, sub) >= 0)
}

func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
go test -count=1 ./internal/agenthelp
```

Expected: FAIL with undefined `RootHelp`, `GroupHelp`, and `Prompt`.

- [ ] **Step 3: Implement agenthelp package**

Create `internal/agenthelp/help.go`:

```go
package agenthelp

type HelpPayload struct {
	OK                 bool              `json:"ok"`
	Tool               string            `json:"tool"`
	Audience           string            `json:"audience"`
	Kind               string            `json:"kind"`
	CommandGroup       string            `json:"command_group,omitempty"`
	AgentPromptCommand string            `json:"agent_prompt_command"`
	Docs               []string          `json:"docs,omitempty"`
	Workflow           []string          `json:"workflow"`
	Commands           map[string]string `json:"commands"`
}

type PromptPayload struct {
	OK       bool   `json:"ok"`
	Tool     string `json:"tool"`
	Audience string `json:"audience"`
	Kind     string `json:"kind"`
	Prompt   string `json:"prompt"`
}

func RootHelp() HelpPayload {
	return HelpPayload{
		OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
		AgentPromptCommand: "aget prompt",
		Docs: []string{"AGENT_INSTRUCTIONS.md", "README.md"},
		Workflow: []string{
			"Use browser status first if you need to verify the managed browser",
			"Open a URL with aget open and keep the returned sid",
			"Continue with returned sid and next_commands",
			"Use page read for text extraction before deciding actions",
			"Use page screenshot when visual state matters",
			"Always close sessions with aget session close when finished",
		},
		Commands: map[string]string{
			"browser_status":  "aget browser status",
			"browser_install": "aget browser install",
			"open":            "aget open URL -n NAME",
			"page_read":       "aget page read -s SID --limit 80",
			"page_click":      "aget page click -s SID --selector CSS",
			"page_type":       "aget page type -s SID --selector CSS --text TEXT",
			"page_screenshot": "aget page screenshot -s SID --path ./page.png",
			"session_list":    "aget session list",
			"session_close":   "aget session close -s SID",
			"prompt":          "aget prompt",
		},
	}
}

func GroupHelp(name string) (HelpPayload, bool) {
	groups := map[string]HelpPayload{
		"page": {
			OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
			CommandGroup: "page", AgentPromptCommand: "aget prompt",
			Workflow: []string{
				"Use page read before click/type when possible",
				"Use CSS selectors for click/type",
				"Use screenshot when text output is insufficient",
			},
			Commands: map[string]string{
				"read":       "aget page read -s SID --limit 80",
				"click":      "aget page click -s SID --selector CSS",
				"type":       "aget page type -s SID --selector CSS --text TEXT",
				"screenshot": "aget page screenshot -s SID --path ./page.png",
			},
		},
		"browser": {
			OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
			CommandGroup: "browser", AgentPromptCommand: "aget prompt",
			Workflow: []string{
				"Use browser status to inspect the managed browser cache without network access",
				"Use browser install to download the pinned managed Chrome for Testing",
				"Use browser path to get the managed browser executable path",
			},
			Commands: map[string]string{
				"status":  "aget browser status",
				"install": "aget browser install",
				"path":    "aget browser path",
			},
		},
		"session": {
			OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
			CommandGroup: "session", AgentPromptCommand: "aget prompt",
			Workflow: []string{
				"Use returned sid values to continue browser workflows",
				"List sessions when you need to recover active sid values",
				"Always close sessions when finished",
			},
			Commands: map[string]string{
				"list":  "aget session list",
				"close": "aget session close -s SID",
				"gc":    "aget session gc",
			},
		},
		"open": {
			OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
			CommandGroup: "open", AgentPromptCommand: "aget prompt",
			Workflow: []string{
				"Open a URL and keep the returned sid",
				"Follow next_commands from the response",
			},
			Commands: map[string]string{"open": "aget open URL -n NAME"},
		},
		"version": {
			OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
			CommandGroup: "version", AgentPromptCommand: "aget prompt",
			Workflow: []string{"Use version for diagnostics only"},
			Commands: map[string]string{"version": "aget version"},
		},
		"prompt": {
			OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
			CommandGroup: "prompt", AgentPromptCommand: "aget prompt",
			Workflow: []string{"Load this prompt when an LLM agent needs usage instructions"},
			Commands: map[string]string{
				"prompt":             "aget prompt",
				"agent_instructions": "aget agent-instructions",
			},
		},
	}
	payload, ok := groups[name]
	return payload, ok
}

func Prompt() PromptPayload {
	return PromptPayload{
		OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_prompt",
		Prompt: "You are using aget, a browser workflow CLI for LLM agents. All operational commands return JSON. Use `aget browser status` to inspect the managed browser when needed. Start with `aget open URL`, save the returned `sid`, then use `aget page read -s SID --limit 80` for text, `aget page click -s SID --selector CSS` for clicks, `aget page type -s SID --selector CSS --text TEXT` for input, and `aget page screenshot -s SID --path ./page.png` when visual state matters. Continue with returned `next_commands` and always run `aget session close -s SID` when finished.",
	}
}
```

- [ ] **Step 4: Verify agenthelp tests pass**

Run:

```bash
go test -count=1 ./internal/agenthelp
```

Expected: PASS.

- [ ] **Step 5: Commit agenthelp payloads**

Run:

```bash
git add internal/agenthelp
git commit -m "feat: add agent help payloads"
```

## Task 2: CLI Help and Prompt Commands

**Files:**

- Create: `internal/cli/prompt.go`
- Create: `internal/cli/prompt_test.go`
- Modify: `internal/cli/browser.go`
- Modify: `internal/cli/open.go`
- Modify: `internal/cli/page.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/root_test.go`
- Modify: `internal/cli/session.go`
- Modify: `internal/cli/version.go`
- Modify: `internal/cli/page_test.go`

- [ ] **Step 1: Write failing root help tests**

Update `internal/cli/root_test.go`.

Replace `TestVersionHelpReturnsJSONError` with:

```go
func TestVersionHelpReturnsAgentHelp(t *testing.T) {
	stdout, stderr, err := executeForTest("version", "--help")
	if err != nil {
		t.Fatalf("expected help success, got err=%v stderr=%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["kind"] != "agent_help" || got["command_group"] != "version" {
		t.Fatalf("unexpected help payload: %#v", got)
	}
}
```

Add:

```go
func TestRootHelpReturnsAgentHelp(t *testing.T) {
	stdout, stderr, err := executeForTest("--help")
	if err != nil {
		t.Fatalf("expected help success, got err=%v stderr=%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["tool"] != "aget" || got["audience"] != "llm_agent" || got["agent_prompt_command"] != "aget prompt" {
		t.Fatalf("unexpected root help payload: %#v", got)
	}
	commands, ok := got["commands"].(map[string]any)
	if !ok {
		t.Fatalf("commands missing or wrong type: %#v", got["commands"])
	}
	if commands["open"] == "" || commands["page_read"] == "" {
		t.Fatalf("core commands missing: %#v", commands)
	}
}

func TestBrowserHelpReturnsAgentHelp(t *testing.T) {
	stdout, stderr, err := executeForTest("browser", "--help")
	if err != nil {
		t.Fatalf("expected help success, got err=%v stderr=%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["kind"] != "agent_help" || got["command_group"] != "browser" {
		t.Fatalf("unexpected browser help payload: %#v", got)
	}
	commands, ok := got["commands"].(map[string]any)
	if !ok {
		t.Fatalf("commands missing or wrong type: %#v", got["commands"])
	}
	if commands["status"] == "" || commands["install"] == "" || commands["path"] == "" {
		t.Fatalf("browser commands missing: %#v", commands)
	}
}

func TestInvalidArgsHintPointsToPrompt(t *testing.T) {
	_, stderr, err := executeForTest()
	if err == nil {
		t.Fatal("expected error")
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stderr), &got); err != nil {
		t.Fatal(err)
	}
	details, ok := got["details"].(map[string]any)
	if !ok {
		t.Fatalf("details missing: %#v", got)
	}
	hint, _ := details["hint"].(string)
	if hint == "" || !strings.Contains(hint, "aget prompt") || !strings.Contains(hint, "aget --help") {
		t.Fatalf("hint = %q", hint)
	}
}
```

Add `strings` to the imports.

- [ ] **Step 2: Run root help tests and verify RED**

Run:

```bash
go test -count=1 ./internal/cli -run 'TestRootHelp|TestVersionHelp|TestBrowserHelp|TestInvalidArgsHint'
```

Expected: FAIL because help is still disabled and hint still says `run aget --help`.

- [ ] **Step 3: Write failing prompt command tests**

Create `internal/cli/prompt_test.go`:

```go
package cli

import (
	"encoding/json"
	"testing"
)

func TestPromptCommandReturnsAgentPrompt(t *testing.T) {
	stdout, stderr, err := executeForTest("prompt")
	if err != nil {
		t.Fatalf("prompt failed: %v stderr=%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["kind"] != "agent_prompt" || got["tool"] != "aget" || got["audience"] != "llm_agent" {
		t.Fatalf("unexpected prompt payload: %#v", got)
	}
	if got["prompt"] == "" {
		t.Fatalf("prompt missing: %#v", got)
	}
}

func TestAgentInstructionsAliasMatchesPrompt(t *testing.T) {
	promptStdout, _, promptErr := executeForTest("prompt")
	if promptErr != nil {
		t.Fatal(promptErr)
	}
	aliasStdout, stderr, aliasErr := executeForTest("agent-instructions")
	if aliasErr != nil {
		t.Fatalf("agent-instructions failed: %v stderr=%s", aliasErr, stderr)
	}
	if aliasStdout != promptStdout {
		t.Fatalf("alias payload differs\nprompt=%s\nalias=%s", promptStdout, aliasStdout)
	}
}
```

- [ ] **Step 4: Run prompt tests and verify RED**

Run:

```bash
go test -count=1 ./internal/cli -run 'TestPrompt|TestAgentInstructions'
```

Expected: FAIL because prompt commands are not registered.

- [ ] **Step 5: Implement CLI help handling and prompt commands**

Create `internal/cli/prompt.go`:

```go
package cli

import (
	"github.com/izzzzzi/agent-aget/internal/agenthelp"
	"github.com/spf13/cobra"
)

func newPromptCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Print LLM agent instructions",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeJSON(cmd, agenthelp.Prompt())
		},
	}
	configureAgentHelp(cmd)
	return cmd
}

func newAgentInstructionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-instructions",
		Short: "Alias for prompt",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeJSON(cmd, agenthelp.Prompt())
		},
	}
	configureAgentHelp(cmd)
	return cmd
}
```

Modify `internal/cli/root.go`:

```go
import (
	"errors"

	"github.com/izzzzzi/agent-aget/internal/agenthelp"
	"github.com/izzzzzi/agent-aget/internal/response"
	"github.com/spf13/cobra"
)

const invalidArgsHint = "run `aget --help` for agent command map or `aget prompt` for full agent instructions"
```

Change `writeInvalidArgs`:

```go
func writeInvalidArgs(cmd *cobra.Command, message string) error {
	return writeError(cmd, "invalid_args", message, map[string]any{"hint": invalidArgsHint})
}
```

Implement JSON help through Cobra's standard help path. Cobra returns `flag.ErrHelp` for `--help` and then calls the command's inherited `HelpFunc`, so this keeps help successful and avoids parsing `SetArgs` manually.

Add these helpers in `internal/cli/root.go`:

```go
func configureAgentHelp(cmd *cobra.Command) {
	cmd.SetHelpFunc(func(helpCmd *cobra.Command, args []string) {
		if err := writeAgentHelp(helpCmd); err != nil {
			helpCmd.PrintErr(err)
		}
	})
}

func writeAgentHelp(cmd *cobra.Command) error {
	if cmd == nil || cmd.CommandPath() == "aget" {
		return writeJSON(cmd, agenthelp.RootHelp())
	}
	group := agentHelpGroup(cmd)
	if payload, ok := agenthelp.GroupHelp(group); ok {
		return writeJSON(cmd, payload)
	}
	return writeJSON(cmd, agenthelp.RootHelp())
}

func agentHelpGroup(cmd *cobra.Command) string {
	if cmd == nil || cmd.CommandPath() == "aget" {
		return ""
	}
	current := cmd
	for current.Parent() != nil && current.Parent().Name() != "aget" {
		current = current.Parent()
	}
	if current.Name() == "agent-instructions" {
		return "prompt"
	}
	return current.Name()
}
```

Register commands:

```go
cmd.AddCommand(newVersionCommand(), newSessionCommand(), newOpenCommand(), newPageCommand(), newBrowserCommand(), newPromptCommand(), newAgentInstructionsCommand())
```

Replace every `disableHelpFlag(cmd)` call in these files with `configureAgentHelp(cmd)`:

```text
internal/cli/browser.go
internal/cli/open.go
internal/cli/page.go
internal/cli/prompt.go
internal/cli/root.go
internal/cli/session.go
internal/cli/version.go
```

Delete `disableHelpFlag`, `noHelpFlag`, and the `noHelpFlag` methods from `internal/cli/root.go`.

Keep `SetFlagErrorFunc` for malformed flags only:

```go
cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
	return writeInvalidArgs(cmd, err.Error())
})
```

- [ ] **Step 6: Update page help test**

Modify `internal/cli/page_test.go`. Replace the existing page help error expectation with:

```go
func TestPageReadHelpReturnsAgentHelp(t *testing.T) {
	stdout, stderr, err := executeForTest("page", "read", "--help")
	if err != nil {
		t.Fatalf("expected help success, got err=%v stderr=%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["kind"] != "agent_help" {
		t.Fatalf("kind = %v", got["kind"])
	}
	if got["command_group"] != "page" {
		t.Fatalf("command_group = %v, want page", got["command_group"])
	}
}

func TestPageGroupHelpReturnsAgentHelp(t *testing.T) {
	stdout, stderr, err := executeForTest("page", "--help")
	if err != nil {
		t.Fatalf("expected help success, got err=%v stderr=%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["kind"] != "agent_help" || got["command_group"] != "page" {
		t.Fatalf("unexpected page help payload: %#v", got)
	}
}
```

If the old test name is `TestPageReadHelpReturnsJSONError`, rename it to the new name.

- [ ] **Step 7: Verify CLI tests pass**

Run:

```bash
gofmt -w internal/agenthelp internal/cli
go test -count=1 ./internal/agenthelp ./internal/cli
```

Expected: PASS.

- [ ] **Step 8: Run broader Go tests**

Run:

```bash
go test -count=1 ./...
```

Expected: PASS.

- [ ] **Step 9: Commit CLI help and prompt commands**

Run:

```bash
git add internal/agenthelp internal/cli
git commit -m "feat: add agent-oriented help prompt"
```

## Task 3: Docs and Package Contract

**Files:**

- Modify: `README.md`
- Modify: `README.en.md`
- Modify: `package.json`
- Modify: `scripts/release-contract-test.js`

- [ ] **Step 1: Write failing release package assertion**

Modify `scripts/release-contract-test.js` in `verifyPackageFiles()` so required files include `AGENT_INSTRUCTIONS.md`:

```js
for (const required of ['browser-manifest.json', 'scripts/browser-install.js', 'AGENT_INSTRUCTIONS.md']) {
  if (!names.includes(required)) {
    throw new Error(`missing npm package file: ${required}`);
  }
}
```

- [ ] **Step 2: Run release contract and verify RED**

Run:

```bash
npm run release:contract
```

Expected: FAIL with `missing npm package file: AGENT_INSTRUCTIONS.md` if `package.json` does not yet include it.

- [ ] **Step 3: Include agent instructions in package**

Modify `package.json` files list:

```json
"files": [
  "bin/aget.js",
  "scripts",
  "browser-manifest.json",
  "README.md",
  "README.en.md",
  "AGENT_INSTRUCTIONS.md",
  "LICENSE"
]
```

- [ ] **Step 4: Update README files**

In `README.md`, add a short section after quick start or JSON contract:

````markdown
## Справка для агентов

`aget --help` возвращает JSON-карту команд для LLM-агента, а не обычный Cobra help:

```bash
aget --help
aget page --help
```

Для полной короткой инструкции загрузите prompt:

```bash
aget prompt
aget agent-instructions
```

Все эти команды сохраняют JSON-контракт CLI.
````

In `README.en.md`, add:

````markdown
## Agent Help

`aget --help` returns a JSON command map for an LLM agent, not standard Cobra help:

```bash
aget --help
aget page --help
```

For the full short instruction prompt, load:

```bash
aget prompt
aget agent-instructions
```

These commands keep the CLI JSON contract.
````

- [ ] **Step 5: Verify docs/package contract**

Run:

```bash
npm pack --dry-run
npm run release:contract
```

Expected: PASS and npm pack output includes `AGENT_INSTRUCTIONS.md`.

- [ ] **Step 6: Run full verification**

Run:

```bash
test -z "$(gofmt -l cmd internal)" && go vet ./... && go test -count=1 ./... && go test -race -count=1 ./... && GOTOOLCHAIN=go1.22.12 go test -count=1 ./... && AGENT_AGET_SKIP_DOWNLOAD=1 npm run smoke && npm run test:browser-install && npm pack --dry-run && npm run release:contract
```

Expected: exit 0.

- [ ] **Step 7: Commit docs/package contract**

Run:

```bash
git add README.md README.en.md package.json scripts/release-contract-test.js
git commit -m "docs: document agent help prompt"
```

## Final Completion Checklist

- [ ] `aget --help` returns `ok:true`, `tool:"aget"`, `audience:"llm_agent"` on stdout.
- [ ] `aget page --help` returns scoped page JSON help on stdout.
- [ ] `aget browser --help` returns scoped browser JSON help on stdout.
- [ ] `aget prompt` and `aget agent-instructions` return identical `agent_prompt` JSON.
- [ ] Invalid args hint mentions both `aget --help` and `aget prompt`.
- [ ] Unknown command with `--help` remains `invalid_args`.
- [ ] `AGENT_INSTRUCTIONS.md` is included in npm package.
- [ ] Full verification exits 0.
