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

func TestPageNestedHelpReturnsJSONError(t *testing.T) {
	stdout, stderr, err := executeForTest("page", "read", "--help")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
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

func replaceChromeDPDriverForTest(t *testing.T, driver *recordingDriver) func() {
	t.Helper()
	old := newChromeDPDriver
	newChromeDPDriver = func(ctx context.Context, debugURL string) (cdp.Driver, error) {
		driver.debugURL = debugURL
		return driver, nil
	}
	return func() {
		newChromeDPDriver = old
	}
}

type recordingDriver struct {
	debugURL      string
	clicked       string
	typedSelector string
	typedText     string
}

func (d *recordingDriver) Read(context.Context) (cdp.PageState, error) {
	return cdp.PageState{}, errors.New("unexpected read")
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

func (d *recordingDriver) Screenshot(context.Context, string) error {
	return errors.New("unexpected screenshot")
}

func (d *recordingDriver) Close(context.Context) error {
	return nil
}
