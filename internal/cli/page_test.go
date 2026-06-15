package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/izzzzzi/agent-aget/internal/cdp"
	sessionstore "github.com/izzzzzi/agent-aget/internal/session"
	"github.com/izzzzzi/agent-aget/internal/state"
)

func TestResolveCleanModePrecedence(t *testing.T) {
	t.Setenv("AGET_CLEAN", "")
	cases := []struct {
		name         string
		args         []string
		env          string
		sessionClean bool
		want         bool
	}{
		{"default off", nil, "", false, false},
		{"session default on", nil, "", true, true},
		{"env on overrides session off", nil, "1", false, true},
		{"explicit --clean", []string{"--clean"}, "", false, true},
		{"explicit --no-clean beats session on", []string{"--no-clean"}, "", true, false},
		{"explicit --no-clean beats env on", []string{"--no-clean"}, "1", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AGET_CLEAN", tc.env)
			cmd := newPageReadCommand()
			cmd.SetArgs(append([]string{"-s", "x"}, tc.args...))
			if err := cmd.ParseFlags(append([]string{"-s", "x"}, tc.args...)); err != nil {
				t.Fatal(err)
			}
			cleanFlag, _ := cmd.Flags().GetBool("clean")
			noCleanFlag, _ := cmd.Flags().GetBool("no-clean")
			got := resolveCleanMode(cmd, tc.sessionClean, cleanFlag, noCleanFlag)
			if got != tc.want {
				t.Fatalf("resolveCleanMode = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPageReadRequiresSID(t *testing.T) {
	stdout, stderr, err := executeForTest("page", "read")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestPageClickRequiresSelector(t *testing.T) {
	stdout, stderr, err := executeForTest("page", "click", "-s", "abc12345")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestPageReadMissingSessionReturnsSessionNotFound(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())

	stdout, stderr, err := executeForTest("page", "read", "-s", "deadbeef")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertErrorCodeJSON(t, stderr, "session_not_found")
}

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

func TestPageClickCallsDriver(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	stdout, stderr, err := executeForTest("page", "click", "-s", "abc12345", "--selector", "button.submit")
	if err != nil {
		t.Fatalf("page click failed: %v stderr=%s", err, stderr)
	}
	if driver.debugURL != "http://127.0.0.1:9222" {
		t.Fatalf("debugURL = %q", driver.debugURL)
	}
	if driver.clicked != "button.submit" {
		t.Fatalf("clicked = %q", driver.clicked)
	}
	if driver.closed {
		t.Fatal("driver was closed; page commands must preserve the browser target for later session commands")
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["sid"] != "abc12345" || got["selector"] != "button.submit" {
		t.Fatalf("response = %#v", got)
	}
}

func TestPageFindResolvesSelector(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{findSelector: "#go"}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	stdout, stderr, err := executeForTest("page", "find", "-s", "abc12345", "--role", "button", "--name", "Submit")
	if err != nil {
		t.Fatalf("page find failed: %v stderr=%s", err, stderr)
	}
	if driver.findCriteria.Role != "button" || driver.findCriteria.Name != "Submit" {
		t.Fatalf("criteria = %#v", driver.findCriteria)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["selector"] != "#go" {
		t.Fatalf("response = %#v", got)
	}
	if _, hasAction := got["action"]; hasAction {
		t.Fatalf("no action requested but action present: %#v", got)
	}
}

func TestPageFindAndClick(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{findSelector: "#go"}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	stdout, stderr, err := executeForTest("page", "find", "-s", "abc12345", "--text", "Submit", "--action", "click")
	if err != nil {
		t.Fatalf("page find click failed: %v stderr=%s", err, stderr)
	}
	if driver.clicked != "#go" {
		t.Fatalf("clicked = %q, want #go", driver.clicked)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["action"] != "click" || got["selector"] != "#go" {
		t.Fatalf("response = %#v", got)
	}
}

func TestPageFindAmbiguousReturnsError(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{findErr: cdp.ErrAmbiguousMatch}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	_, stderr, err := executeForTest("page", "find", "-s", "abc12345", "--role", "link", "--name", "Details")
	if err == nil {
		t.Fatal("expected error for ambiguous match")
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stderr), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != "locator_ambiguous" {
		t.Fatalf("code = %v, want locator_ambiguous", got["code"])
	}
}

func TestPageFindRequiresCriteria(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	_, _, err := executeForTest("page", "find", "-s", "abc12345")
	if err == nil {
		t.Fatal("expected error when no locator flags given")
	}
}

func TestPageTypeCallsDriver(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	stdout, stderr, err := executeForTest("page", "type", "-s", "abc12345", "--selector", "input[name=q]", "--text", "hello")
	if err != nil {
		t.Fatalf("page type failed: %v stderr=%s", err, stderr)
	}
	if driver.typedSelector != "input[name=q]" || driver.typedText != "hello" {
		t.Fatalf("typed selector/text = %q/%q", driver.typedSelector, driver.typedText)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["sid"] != "abc12345" || got["selector"] != "input[name=q]" || got["text_len"] != float64(5) {
		t.Fatalf("response = %#v", got)
	}
}

func TestPageReadUsesCommandTimeout(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &blockingDriver{}
	restoreDriver := replaceChromeDPDriverForTest(t, driver)
	defer restoreDriver()
	restoreTimeout := replacePageCommandTimeoutForTest(20 * time.Millisecond)
	defer restoreTimeout()

	stdout, stderr, err := executeForTest("page", "read", "-s", "abc12345")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertErrorCodeJSON(t, stderr, "page_read_failed")
	if !driver.readCanceled {
		t.Fatal("driver read did not observe context cancellation")
	}
}

func TestPageSnapshotSavesRefs(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{snapshot: cdp.SnapshotState{
		URL:   "https://example.com",
		Title: "Example",
		Elements: []cdp.Element{
			{Kind: "button", Text: "Submit", Selector: "button[type=submit]", Visible: true, Enabled: true},
		},
	}}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	stdout, stderr, err := executeForTest("page", "snapshot", "-s", "abc12345")
	if err != nil {
		t.Fatalf("page snapshot failed: %v stderr=%s", err, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	elements := got["elements"].([]any)
	first := elements[0].(map[string]any)
	if first["ref"] != "@e1" {
		t.Fatalf("first ref = %v", first["ref"])
	}
}

func TestPageClickByRefUsesSavedSnapshot(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{snapshot: cdp.SnapshotState{
		URL: "https://example.com",
		Elements: []cdp.Element{
			{Kind: "button", Selector: "button[type=submit]", Visible: true, Enabled: true},
		},
	}}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	if _, stderr, err := executeForTest("page", "snapshot", "-s", "abc12345"); err != nil {
		t.Fatalf("snapshot failed: %v stderr=%s", err, stderr)
	}
	if _, stderr, err := executeForTest("page", "click", "-s", "abc12345", "--ref", "@e1"); err != nil {
		t.Fatalf("click by ref failed: %v stderr=%s", err, stderr)
	}
	if driver.clicked != "button[type=submit]" {
		t.Fatalf("clicked = %q", driver.clicked)
	}
}

func TestPageClickRejectsSelectorAndRefTogether(t *testing.T) {
	stdout, stderr, err := executeForTest("page", "click", "-s", "abc12345", "--selector", "button", "--ref", "@e1")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestPageFillDoesNotEchoText(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	stdout, stderr, err := executeForTest("page", "fill", "-s", "abc12345", "--selector", "input[name=email]", "--text", "secret@example.com")
	if err != nil {
		t.Fatalf("fill failed: %v stderr=%s", err, stderr)
	}
	if strings.Contains(stdout, "secret@example.com") {
		t.Fatalf("stdout leaked text: %s", stdout)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["text_len"] != float64(18) {
		t.Fatalf("text_len = %v", got["text_len"])
	}
	if driver.filledSelector != "input[name=email]" || driver.filledText != "secret@example.com" {
		t.Fatalf("filled selector/text = %q/%q", driver.filledSelector, driver.filledText)
	}
}

func TestPageFillRequiresTextFlag(t *testing.T) {
	stdout, stderr, err := executeForTest("page", "fill", "-s", "abc12345", "--selector", "input[name=email]")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestPageWaitRequiresExactlyOneCondition(t *testing.T) {
	stdout, stderr, err := executeForTest("page", "wait", "-s", "abc12345", "--selector", "#ready", "--text", "Ready")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestPageGetURLDoesNotRequireTarget(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{getValue: "https://example.com"}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	stdout, stderr, err := executeForTest("page", "get", "-s", "abc12345", "url")
	if err != nil {
		t.Fatalf("get url failed: %v stderr=%s", err, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["value"] != "https://example.com" {
		t.Fatalf("value = %v", got["value"])
	}
}

func TestPageGetRejectsUnsupportedKindBeforeSessionLookup(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	stdout, stderr, err := executeForTest("page", "get", "-s", "missing", "bogus")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestPageSelectCallsDriver(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{snapshot: cdp.SnapshotState{
		URL: "https://example.com",
		Elements: []cdp.Element{
			{Kind: "select", Selector: "select[name=direction]", Visible: true, Enabled: true},
		},
	}}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	if _, stderr, err := executeForTest("page", "snapshot", "-s", "abc12345"); err != nil {
		t.Fatalf("snapshot failed: %v stderr=%s", err, stderr)
	}
	stdout, stderr, err := executeForTest("page", "select", "-s", "abc12345", "--ref", "@i1", "--value", "Option A")
	if err != nil {
		t.Fatalf("page select failed: %v stderr=%s", err, stderr)
	}
	if driver.selectedValue != "Option A" {
		t.Fatalf("selectedValue = %q", driver.selectedValue)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["sid"] != "abc12345" || got["ref"] != "@i1" {
		t.Fatalf("response = %#v", got)
	}
}

func TestPageSelectRequiresValue(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	stdout, stderr, err := executeForTest("page", "select", "-s", "abc12345", "--selector", "select[name=direction]")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}

func TestPageIsReturnsState(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{snapshot: cdp.SnapshotState{
		URL: "https://example.com",
		Elements: []cdp.Element{
			{Kind: "button", Selector: "#btn", Visible: true, Enabled: true},
		},
	}, isResult: true}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	if _, stderr, err := executeForTest("page", "snapshot", "-s", "abc12345"); err != nil {
		t.Fatalf("snapshot failed: %v stderr=%s", err, stderr)
	}
	stdout, stderr, err := executeForTest("page", "is", "-s", "abc12345", "--ref", "@e1", "visible")
	if err != nil {
		t.Fatalf("page is failed: %v stderr=%s", err, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["value"] != true {
		t.Fatalf("value = %v", got["value"])
	}
}

func TestPageJSCallsEval(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	stdout, stderr, err := executeForTest("page", "js", "-s", "abc12345", "--expr", "document.title")
	if err != nil {
		t.Fatalf("page js failed: %v stderr=%s", err, stderr)
	}
	if driver.evalExpression != "document.title" {
		t.Fatalf("evalExpression = %q", driver.evalExpression)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["result"]; !ok {
		t.Fatalf("response missing result: %#v", got)
	}
}

func TestPageCheckCallsDriver(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{snapshot: cdp.SnapshotState{
		URL: "https://example.com",
		Elements: []cdp.Element{
			{Kind: "input", Type: "checkbox", Selector: "input[name=agree]", Visible: true, Enabled: true},
		},
	}}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	if _, stderr, err := executeForTest("page", "snapshot", "-s", "abc12345"); err != nil {
		t.Fatalf("snapshot failed: %v stderr=%s", err, stderr)
	}
	stdout, stderr, err := executeForTest("page", "check", "-s", "abc12345", "--ref", "@i1")
	if err != nil {
		t.Fatalf("page check failed: %v stderr=%s", err, stderr)
	}
	if driver.checkedSelector == "" {
		t.Fatal("check was not called")
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("response = %#v", got)
	}
}

func TestPageUncheckCallsDriver(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{snapshot: cdp.SnapshotState{
		URL: "https://example.com",
		Elements: []cdp.Element{
			{Kind: "input", Type: "checkbox", Selector: "input[name=agree]", Visible: true, Enabled: true},
		},
	}}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	if _, stderr, err := executeForTest("page", "snapshot", "-s", "abc12345"); err != nil {
		t.Fatalf("snapshot failed: %v stderr=%s", err, stderr)
	}
	stdout, stderr, err := executeForTest("page", "uncheck", "-s", "abc12345", "--ref", "@i1")
	if err != nil {
		t.Fatalf("page uncheck failed: %v stderr=%s", err, stderr)
	}
	if driver.uncheckedSelector == "" {
		t.Fatal("uncheck was not called")
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("response = %#v", got)
	}
}

func TestPageHoverCallsDriver(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{snapshot: cdp.SnapshotState{
		URL: "https://example.com",
		Elements: []cdp.Element{
			{Kind: "button", Selector: ".menu-btn", Visible: true, Enabled: true},
		},
	}}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	if _, stderr, err := executeForTest("page", "snapshot", "-s", "abc12345"); err != nil {
		t.Fatalf("snapshot failed: %v stderr=%s", err, stderr)
	}
	stdout, stderr, err := executeForTest("page", "hover", "-s", "abc12345", "--ref", "@e1")
	if err != nil {
		t.Fatalf("page hover failed: %v stderr=%s", err, stderr)
	}
	if driver.hoveredSelector == "" {
		t.Fatal("hover was not called")
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("response = %#v", got)
	}
}

func TestPageFocusCallsDriver(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{snapshot: cdp.SnapshotState{
		URL: "https://example.com",
		Elements: []cdp.Element{
			{Kind: "input", Selector: "input[name=q]", Visible: true, Enabled: true},
		},
	}}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	if _, stderr, err := executeForTest("page", "snapshot", "-s", "abc12345"); err != nil {
		t.Fatalf("snapshot failed: %v stderr=%s", err, stderr)
	}
	stdout, stderr, err := executeForTest("page", "focus", "-s", "abc12345", "--ref", "@i1")
	if err != nil {
		t.Fatalf("page focus failed: %v stderr=%s", err, stderr)
	}
	if driver.focusedSelector == "" {
		t.Fatal("focus was not called")
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("response = %#v", got)
	}
}

func TestPageUploadCallsDriver(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{snapshot: cdp.SnapshotState{
		URL: "https://example.com",
		Elements: []cdp.Element{
			{Kind: "input", Type: "file", Selector: "input[type=file]", Visible: true, Enabled: true},
		},
	}}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	if _, stderr, err := executeForTest("page", "snapshot", "-s", "abc12345"); err != nil {
		t.Fatalf("snapshot failed: %v stderr=%s", err, stderr)
	}
	stdout, stderr, err := executeForTest("page", "upload", "-s", "abc12345", "--ref", "@i1", "--file", "/tmp/test.pdf")
	if err != nil {
		t.Fatalf("page upload failed: %v stderr=%s", err, stderr)
	}
	if driver.uploadedSelector == "" {
		t.Fatal("upload was not called")
	}
	if len(driver.uploadedFiles) != 1 || driver.uploadedFiles[0] != "/tmp/test.pdf" {
		t.Fatalf("uploadedFiles = %v", driver.uploadedFiles)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("response = %#v", got)
	}
}

func TestPageDialogAcceptCallsDriver(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	stdout, stderr, err := executeForTest("page", "dialog-accept", "-s", "abc12345")
	if err != nil {
		t.Fatalf("page dialog-accept failed: %v stderr=%s", err, stderr)
	}
	if !driver.dialogAccepted {
		t.Fatal("dialog-accept was not called")
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("response = %#v", got)
	}
}

func TestPageDialogDismissCallsDriver(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	stdout, stderr, err := executeForTest("page", "dialog-dismiss", "-s", "abc12345")
	if err != nil {
		t.Fatalf("page dialog-dismiss failed: %v stderr=%s", err, stderr)
	}
	if !driver.dialogDismissed {
		t.Fatal("dialog-dismiss was not called")
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("response = %#v", got)
	}
}

func TestPageTypeByRefUsesSavedSnapshot(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{snapshot: cdp.SnapshotState{
		URL: "https://example.com",
		Elements: []cdp.Element{
			{Kind: "input", Selector: "input[name=q]", Visible: true, Enabled: true},
		},
	}}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	if _, stderr, err := executeForTest("page", "snapshot", "-s", "abc12345"); err != nil {
		t.Fatalf("snapshot failed: %v stderr=%s", err, stderr)
	}
	stdout, stderr, err := executeForTest("page", "type", "-s", "abc12345", "--ref", "@i1", "--text", "hello")
	if err != nil {
		t.Fatalf("type by ref failed: %v stderr=%s", err, stderr)
	}
	if driver.typedSelector != "input[name=q]" || driver.typedText != "hello" {
		t.Fatalf("typed selector/text = %q/%q", driver.typedSelector, driver.typedText)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true || got["ref"] != "@i1" {
		t.Fatalf("response = %#v", got)
	}
}

func TestPageWaitByRefUsesSavedSnapshot(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{snapshot: cdp.SnapshotState{
		URL: "https://example.com",
		Elements: []cdp.Element{
			{Kind: "button", Selector: "#ready-btn", Visible: true, Enabled: true},
		},
	}}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	if _, stderr, err := executeForTest("page", "snapshot", "-s", "abc12345"); err != nil {
		t.Fatalf("snapshot failed: %v stderr=%s", err, stderr)
	}
	if _, stderr, err := executeForTest("page", "wait", "-s", "abc12345", "--ref", "@e1"); err != nil {
		t.Fatalf("wait by ref failed: %v stderr=%s", err, stderr)
	}
	if driver.waitOptions.Selector != "#ready-btn" {
		t.Fatalf("wait selector = %q", driver.waitOptions.Selector)
	}
}

func saveTestSession(t *testing.T, sid, debugURL string) {
	t.Helper()
	now := time.Now().UTC()
	record := sessionstore.Record{
		SID:        sid,
		URL:        "https://example.com",
		BrowserPID: 1234,
		DebugURL:   debugURL,
		Headless:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := sessionstore.NewRegistry(state.SessionsDir()).Save(record); err != nil {
		t.Fatal(err)
	}
}

func replaceChromeDPDriverForTest(t *testing.T, driver cdp.Driver) func() {
	t.Helper()
	old := newChromeDPDriver
	newChromeDPDriver = func(ctx context.Context, debugURL string) (cdp.Driver, error) {
		if recorder, ok := driver.(*recordingDriver); ok {
			recorder.debugURL = debugURL
		}
		return driver, nil
	}
	return func() {
		newChromeDPDriver = old
	}
}

func replacePageCommandTimeoutForTest(timeout time.Duration) func() {
	old := pageCommandTimeout
	pageCommandTimeout = timeout
	return func() {
		pageCommandTimeout = old
	}
}

type recordingDriver struct {
	debugURL          string
	snapshot          cdp.SnapshotState
	clicked           string
	typedSelector     string
	typedText         string
	filledSelector    string
	filledText        string
	selectedSelector  string
	selectedValue     string
	pressedKey        string
	scrolledDir       string
	scrolledPixels    int
	waitOptions       cdp.WaitOptions
	getOptions        cdp.GetOptions
	getValue          string
	isProp            string
	isResult          bool
	evalExpression    string
	checkedSelector   string
	uncheckedSelector string
	hoveredSelector   string
	focusedSelector   string
	uploadedSelector  string
	uploadedFiles     []string
	dialogAccepted    bool
	dialogText        string
	dialogDismissed   bool
	findCriteria      cdp.FindCriteria
	findSelector      string
	findErr           error
	closed            bool
}

func (d *recordingDriver) Read(context.Context) (cdp.PageState, error) {
	return cdp.PageState{}, errors.New("unexpected read")
}

func (d *recordingDriver) Snapshot(context.Context) (cdp.SnapshotState, error) {
	return d.snapshot, nil
}

func (d *recordingDriver) Find(_ context.Context, criteria cdp.FindCriteria) (string, error) {
	d.findCriteria = criteria
	return d.findSelector, d.findErr
}

func (d *recordingDriver) Click(_ context.Context, selector string) error {
	d.clicked = selector
	return nil
}

func (d *recordingDriver) ClickForce(_ context.Context, selector string) error {
	d.clicked = selector
	return nil
}

func (d *recordingDriver) WaitAppear(_ context.Context, selector string) error {
	d.waitOptions = cdp.WaitOptions{Selector: selector}
	return nil
}

func (d *recordingDriver) Type(_ context.Context, selector, text string) error {
	d.typedSelector = selector
	d.typedText = text
	return nil
}

func (d *recordingDriver) Fill(_ context.Context, selector, text string) error {
	d.filledSelector = selector
	d.filledText = text
	return nil
}

func (d *recordingDriver) Press(_ context.Context, key string) error {
	d.pressedKey = key
	return nil
}

func (d *recordingDriver) Scroll(_ context.Context, direction string, pixels int) error {
	d.scrolledDir = direction
	d.scrolledPixels = pixels
	return nil
}

func (d *recordingDriver) Wait(_ context.Context, options cdp.WaitOptions) error {
	d.waitOptions = options
	return nil
}

func (d *recordingDriver) Get(_ context.Context, options cdp.GetOptions) (string, error) {
	d.getOptions = options
	return d.getValue, nil
}

func (d *recordingDriver) Select(_ context.Context, selector, value string) error {
	d.selectedSelector = selector
	d.selectedValue = value
	return nil
}

func (d *recordingDriver) Is(_ context.Context, selector, prop string) (bool, error) {
	d.isProp = prop
	return d.isResult, nil
}

func (d *recordingDriver) Eval(_ context.Context, expression string) (string, error) {
	d.evalExpression = expression
	return `"ok"`, nil
}

func (d *recordingDriver) Check(_ context.Context, selector string) error {
	d.checkedSelector = selector
	return nil
}

func (d *recordingDriver) Uncheck(_ context.Context, selector string) error {
	d.uncheckedSelector = selector
	return nil
}

func (d *recordingDriver) Hover(_ context.Context, selector string) error {
	d.hoveredSelector = selector
	return nil
}

func (d *recordingDriver) Focus(_ context.Context, selector string) error {
	d.focusedSelector = selector
	return nil
}

func (d *recordingDriver) Upload(_ context.Context, selector string, files []string) error {
	d.uploadedSelector = selector
	d.uploadedFiles = files
	return nil
}

func (d *recordingDriver) DialogAccept(_ context.Context, promptText string) error {
	d.dialogAccepted = true
	d.dialogText = promptText
	return nil
}

func (d *recordingDriver) DialogDismiss(_ context.Context) error {
	d.dialogDismissed = true
	return nil
}

func (d *recordingDriver) Screenshot(context.Context, string) error {
	return errors.New("unexpected screenshot")
}

func (d *recordingDriver) AnnotatedScreenshot(context.Context, string, []cdp.Element) error {
	return errors.New("unexpected annotated screenshot")
}

func (d *recordingDriver) Close(context.Context) error {
	d.closed = true
	return nil
}

type blockingDriver struct {
	readCanceled bool
}

func (d *blockingDriver) Read(ctx context.Context) (cdp.PageState, error) {
	<-ctx.Done()
	d.readCanceled = true
	return cdp.PageState{}, ctx.Err()
}

func (d *blockingDriver) Find(context.Context, cdp.FindCriteria) (string, error) {
	return "", nil
}

func (d *blockingDriver) Snapshot(context.Context) (cdp.SnapshotState, error) {
	return cdp.SnapshotState{}, errors.New("unexpected snapshot")
}

func (d *blockingDriver) Click(context.Context, string) error {
	return errors.New("unexpected click")
}

func (d *blockingDriver) ClickForce(context.Context, string) error {
	return errors.New("unexpected click")
}

func (d *blockingDriver) WaitAppear(context.Context, string) error {
	return errors.New("unexpected wait")
}

func (d *blockingDriver) Type(context.Context, string, string) error {
	return errors.New("unexpected type")
}

func (d *blockingDriver) Fill(context.Context, string, string) error {
	return errors.New("unexpected fill")
}

func (d *blockingDriver) Press(context.Context, string) error {
	return errors.New("unexpected press")
}

func (d *blockingDriver) Scroll(context.Context, string, int) error {
	return errors.New("unexpected scroll")
}

func (d *blockingDriver) Wait(context.Context, cdp.WaitOptions) error {
	return errors.New("unexpected wait")
}

func (d *blockingDriver) Get(context.Context, cdp.GetOptions) (string, error) {
	return "", errors.New("unexpected get")
}

func (d *blockingDriver) Select(context.Context, string, string) error {
	return errors.New("unexpected select")
}

func (d *blockingDriver) Is(context.Context, string, string) (bool, error) {
	return false, errors.New("unexpected is")
}

func (d *blockingDriver) Eval(context.Context, string) (string, error) {
	return "", errors.New("unexpected eval")
}

func (d *blockingDriver) Check(context.Context, string) error {
	return errors.New("unexpected check")
}

func (d *blockingDriver) Uncheck(context.Context, string) error {
	return errors.New("unexpected uncheck")
}

func (d *blockingDriver) Hover(context.Context, string) error {
	return errors.New("unexpected hover")
}

func (d *blockingDriver) Focus(context.Context, string) error {
	return errors.New("unexpected focus")
}

func (d *blockingDriver) Upload(context.Context, string, []string) error {
	return errors.New("unexpected upload")
}

func (d *blockingDriver) DialogAccept(context.Context, string) error {
	return errors.New("unexpected dialog-accept")
}

func (d *blockingDriver) DialogDismiss(context.Context) error {
	return errors.New("unexpected dialog-dismiss")
}

func (d *blockingDriver) Screenshot(context.Context, string) error {
	return errors.New("unexpected screenshot")
}

func (d *blockingDriver) AnnotatedScreenshot(context.Context, string, []cdp.Element) error {
	return nil
}

func (d *blockingDriver) Close(context.Context) error {
	return nil
}
