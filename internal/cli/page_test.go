package cli

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/izzzzzi/agent-aget/internal/cdp"
	sessionstore "github.com/izzzzzi/agent-aget/internal/session"
	"github.com/izzzzzi/agent-aget/internal/state"
)

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

	stdout, stderr, err := executeForTest("page", "read", "-s", "missing")
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
	debugURL      string
	clicked       string
	typedSelector string
	typedText     string
	closed        bool
}

func (d *recordingDriver) Read(context.Context) (cdp.PageState, error) {
	return cdp.PageState{}, errors.New("unexpected read")
}

func (d *recordingDriver) Snapshot(context.Context) (cdp.SnapshotState, error) {
	return cdp.SnapshotState{}, errors.New("unexpected snapshot")
}

func (d *recordingDriver) Click(_ context.Context, selector string) error {
	d.clicked = selector
	return nil
}

func (d *recordingDriver) Type(_ context.Context, selector, text string) error {
	d.typedSelector = selector
	d.typedText = text
	return nil
}

func (d *recordingDriver) Fill(context.Context, string, string) error {
	return errors.New("unexpected fill")
}

func (d *recordingDriver) Press(context.Context, string) error {
	return errors.New("unexpected press")
}

func (d *recordingDriver) Scroll(context.Context, string, int) error {
	return errors.New("unexpected scroll")
}

func (d *recordingDriver) Wait(context.Context, cdp.WaitOptions) error {
	return errors.New("unexpected wait")
}

func (d *recordingDriver) Get(context.Context, cdp.GetOptions) (string, error) {
	return "", errors.New("unexpected get")
}

func (d *recordingDriver) Screenshot(context.Context, string) error {
	return errors.New("unexpected screenshot")
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

func (d *blockingDriver) Snapshot(context.Context) (cdp.SnapshotState, error) {
	return cdp.SnapshotState{}, errors.New("unexpected snapshot")
}

func (d *blockingDriver) Click(context.Context, string) error {
	return errors.New("unexpected click")
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

func (d *blockingDriver) Screenshot(context.Context, string) error {
	return errors.New("unexpected screenshot")
}

func (d *blockingDriver) Close(context.Context) error {
	return nil
}
