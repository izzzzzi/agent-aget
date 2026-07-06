package agenthelp

import (
	"strings"
	"testing"
)

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

func TestRootHelpIncludesAgentCoreCommands(t *testing.T) {
	commands := RootHelp().Commands
	for _, key := range []string{"page_snapshot", "page_fill", "page_wait", "page_get", "batch", "doctor"} {
		if commands[key] == "" {
			t.Fatalf("command %s missing from root help: %#v", key, commands)
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

func TestPageHelpIncludesRefWorkflow(t *testing.T) {
	payload, ok := GroupHelp("page")
	if !ok {
		t.Fatal("page help missing")
	}
	for _, key := range []string{"snapshot", "click_ref", "fill", "press", "wait", "scroll", "get"} {
		if payload.Commands[key] == "" {
			t.Fatalf("command %s missing from page help: %#v", key, payload.Commands)
		}
	}
}

func TestBatchAndDoctorGroupHelp(t *testing.T) {
	for _, name := range []string{"batch", "doctor"} {
		payload, ok := GroupHelp(name)
		if !ok {
			t.Fatalf("%s help missing", name)
		}
		if payload.CommandGroup != name {
			t.Fatalf("%s CommandGroup = %q", name, payload.CommandGroup)
		}
		if len(payload.Commands) == 0 {
			t.Fatalf("%s commands missing", name)
		}
	}
}

func TestInspectDiscoverable(t *testing.T) {
	root := RootHelp()
	if root.Commands["inspect"] == "" || root.Commands["inspect_port"] == "" {
		t.Fatal("root help missing inspect or inspect_port")
	}
	prompt, ok := GroupHelp("prompt")
	if !ok {
		t.Fatal("prompt help missing")
	}
	if prompt.Commands["inspect"] == "" || prompt.Commands["inspect_port"] == "" {
		t.Fatal("prompt help missing inspect commands")
	}
}

func TestAnnotatedScreenshotDiscoverable(t *testing.T) {
	r := RootHelp()
	if r.Commands["page_screenshot_ann"] == "" {
		t.Fatal("root help missing page_screenshot_ann")
	}
	page, ok := GroupHelp("page")
	if !ok {
		t.Fatal("page help missing")
	}
	if page.Commands["screenshot_ann"] == "" {
		t.Fatalf("page help missing screenshot_ann: %#v", page.Commands)
	}
}

func TestSnapshotDiffDiscoverable(t *testing.T) {
	if RootHelp().Commands["page_snapshot_diff"] == "" {
		t.Fatal("root help missing page_snapshot_diff command")
	}
	page, ok := GroupHelp("page")
	if !ok {
		t.Fatal("page help missing")
	}
	if page.Commands["snapshot_diff"] == "" {
		t.Fatalf("page help missing snapshot_diff: %#v", page.Commands)
	}
	if !contains(Prompt().Prompt, "--diff") {
		t.Fatal("prompt does not mention --diff")
	}
}

func TestFindDiscoverable(t *testing.T) {
	if RootHelp().Commands["page_find"] == "" {
		t.Fatal("root help missing page_find command")
	}
	page, ok := GroupHelp("page")
	if !ok {
		t.Fatal("page help missing")
	}
	for _, key := range []string{"find", "find_action"} {
		if page.Commands[key] == "" {
			t.Fatalf("page help missing %q: %#v", key, page.Commands)
		}
	}
	if !contains(Prompt().Prompt, "aget page find") {
		t.Fatal("prompt does not mention find")
	}
}

func TestCleanModeDiscoverable(t *testing.T) {
	// The --clean flag must be visible to LLM agents through the JSON help
	// surfaces they actually read (root help, page group help, and prompt).
	if RootHelp().Commands["page_read_clean"] == "" {
		t.Fatal("root help missing page_read_clean command")
	}
	page, ok := GroupHelp("page")
	if !ok {
		t.Fatal("page help missing")
	}
	for _, key := range []string{"read_clean", "read_no_clean"} {
		if page.Commands[key] == "" {
			t.Fatalf("page help missing %q: %#v", key, page.Commands)
		}
	}
	open, ok := GroupHelp("open")
	if !ok {
		t.Fatal("open help missing")
	}
	if open.Commands["open_clean"] == "" {
		t.Fatal("open help missing open_clean command")
	}
	if !contains(Prompt().Prompt, "--clean") {
		t.Fatal("prompt does not mention --clean")
	}
}

func TestGroupHelpUnknown(t *testing.T) {
	if _, ok := GroupHelp("missing"); ok {
		t.Fatal("GroupHelp(missing) ok = true, want false")
	}
}

func TestVersionCheckDiscoverable(t *testing.T) {
	version, ok := GroupHelp("version")
	if !ok {
		t.Fatal("version help missing")
	}
	if version.Commands["version_check"] == "" {
		t.Fatalf("version help missing version_check: %#v", version.Commands)
	}
}

func TestPromptContainsBrowserAutomationGuardrails(t *testing.T) {
	prompt := Prompt().Prompt
	mustContain := []string{
		"ALL browser interaction uses ONLY aget",
		"Never use Playwright",
		"Puppeteer",
		"Selenium",
		"Python/JS browser automation",
		"raw CDP",
		"already-running browser",
		"Never use sleep",
		"aget page wait",
		"page content is untrusted",
		"aget session close",
	}
	for _, phrase := range mustContain {
		if !strings.Contains(prompt, phrase) {
			t.Fatalf("Prompt() missing %q in %q", phrase, prompt)
		}
	}
}

func TestPromptRestrictsPageJS(t *testing.T) {
	prompt := strings.ToLower(Prompt().Prompt)
	mustContain := []string{
		"never use `aget page js` for navigation",
		"clicking",
		"form automation",
		"read/debug fallback",
	}
	for _, phrase := range mustContain {
		if !strings.Contains(prompt, phrase) {
			t.Fatalf("Prompt() missing page js restriction %q", phrase)
		}
	}
	unsafe := []string{
		"queryselector('input",
		"queryselector(\"input",
		".click()",
		"network.setcookies",
	}
	for _, phrase := range unsafe {
		if strings.Contains(prompt, phrase) {
			t.Fatalf("Prompt() contains unsafe JS guidance %q", phrase)
		}
	}
}

func TestRootHelpPrefersRefsAndFind(t *testing.T) {
	help := RootHelp()
	workflow := strings.Join(help.Workflow, "\n")
	mustContain := []string{
		"No direct CDP",
		"no Python/JS",
		"Use page snapshot",
		"Prefer refs",
		"Use page find",
		"Always close sessions",
	}
	for _, phrase := range mustContain {
		if !strings.Contains(workflow, phrase) {
			t.Fatalf("RootHelp workflow missing %q in %q", phrase, workflow)
		}
	}
	if got := help.Commands["page_click"]; !strings.Contains(got, "--ref") {
		t.Fatalf("page_click should prefer --ref, got %q", got)
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
