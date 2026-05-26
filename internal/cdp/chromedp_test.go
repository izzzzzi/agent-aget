package cdp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestWaitForPageURLAcceptsNonBlankPageTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/list" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"target-1","type":"page","url":"https://example.com/"}]`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := WaitForPageURL(ctx, server.URL, "https://example.com"); err != nil {
		t.Fatalf("WaitForPageURL returned error for normalized URL: %v", err)
	}
}

func TestClickReturnsCanceledContextWithoutRunningAction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	driver := &ChromeDPDriver{
		ctx: context.Background(),
		run: func(ctx context.Context, actions ...chromedp.Action) error {
			t.Fatal("runner should not be called for an already canceled context")
			return nil
		},
	}

	err := driver.Click(ctx, "#login")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Click error = %v, want context.Canceled", err)
	}
}

func TestFillReturnsCanceledContextWithoutRunningAction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	driver := &ChromeDPDriver{
		ctx: context.Background(),
		run: func(ctx context.Context, actions ...chromedp.Action) error {
			t.Fatal("runner should not be called for an already canceled context")
			return nil
		},
	}

	err := driver.Fill(ctx, "#email", "me@example.com")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Fill error = %v, want context.Canceled", err)
	}
}

func TestPressReturnsCanceledContextWithoutRunningAction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	driver := &ChromeDPDriver{
		ctx: context.Background(),
		run: func(ctx context.Context, actions ...chromedp.Action) error {
			t.Fatal("runner should not be called for an already canceled context")
			return nil
		},
	}

	err := driver.Press(ctx, "Enter")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Press error = %v, want context.Canceled", err)
	}
}

func TestWaitTextReturnsCanceledContextWithoutRunningAction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	driver := &ChromeDPDriver{
		ctx: context.Background(),
		run: func(ctx context.Context, actions ...chromedp.Action) error {
			t.Fatal("runner should not be called for an already canceled context")
			return nil
		},
	}

	err := driver.Wait(ctx, WaitOptions{Text: "Ready"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait error = %v, want context.Canceled", err)
	}
}

func TestRunActionsCancelsWhenCallContextIsCanceled(t *testing.T) {
	callCtx, cancelCall := context.WithCancel(context.Background())
	started := make(chan context.Context, 1)
	done := make(chan error, 1)
	driver := &ChromeDPDriver{
		ctx: context.Background(),
		run: func(ctx context.Context, actions ...chromedp.Action) error {
			started <- ctx
			<-ctx.Done()
			return ctx.Err()
		},
	}

	go func() {
		done <- driver.Click(callCtx, "#login")
	}()

	var runCtx context.Context
	select {
	case runCtx = <-started:
	case <-time.After(time.Second):
		t.Fatal("runner was not called")
	}

	cancelCall()

	select {
	case <-runCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("runner context was not canceled")
	}

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Click error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Click did not return after call context cancellation")
	}
}

func TestRunActionsKeepsTargetContextAliveAfterSuccessfulRun(t *testing.T) {
	driverCtx, cancelDriver := context.WithCancel(context.Background())
	defer cancelDriver()
	started := make(chan context.Context, 1)
	driver := &ChromeDPDriver{
		ctx: driverCtx,
		run: func(ctx context.Context, actions ...chromedp.Action) error {
			started <- ctx
			return nil
		},
	}

	if err := driver.Click(context.Background(), "#login"); err != nil {
		t.Fatal(err)
	}

	var runCtx context.Context
	select {
	case runCtx = <-started:
	case <-time.After(time.Second):
		t.Fatal("runner was not called")
	}

	select {
	case <-runCtx.Done():
		t.Fatal("run context was canceled after successful action")
	case <-time.After(10 * time.Millisecond):
	}
}

func TestReadRetriesTransientExecutionContextError(t *testing.T) {
	attempts := 0
	driver := &ChromeDPDriver{
		ctx: context.Background(),
		run: func(ctx context.Context, actions ...chromedp.Action) error {
			attempts++
			if attempts == 1 {
				return fmt.Errorf("Execution context was destroyed. (-32000)")
			}
			return nil
		},
	}

	if _, err := driver.Read(context.Background()); err != nil {
		t.Fatalf("Read returned error after transient retry: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestScreenshotRetriesTransientZeroWidthError(t *testing.T) {
	attempts := 0
	driver := &ChromeDPDriver{
		ctx: context.Background(),
		run: func(ctx context.Context, actions ...chromedp.Action) error {
			attempts++
			if attempts == 1 {
				return fmt.Errorf("Cannot take screenshot with 0 width. (-32000)")
			}
			return nil
		},
	}

	path := filepath.Join(t.TempDir(), "screenshot.png")
	if err := driver.Screenshot(context.Background(), path); err != nil {
		t.Fatalf("Screenshot returned error after transient retry: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestSnapshotSelectorsTargetDuplicateButtons(t *testing.T) {
	elements := snapshotElementsForHTML(t, `<!doctype html>
<html><head><title>selectors</title></head><body>
  <section><button>First</button><button>Second</button></section>
  <section><button>Third</button><button>Fourth</button></section>
</body></html>`)

	want := map[string]string{
		"First":  "html > body > section:nth-of-type(1) > button:nth-of-type(1)",
		"Second": "html > body > section:nth-of-type(1) > button:nth-of-type(2)",
		"Third":  "html > body > section:nth-of-type(2) > button:nth-of-type(1)",
		"Fourth": "html > body > section:nth-of-type(2) > button:nth-of-type(2)",
	}
	assertSnapshotSelectors(t, elements, want)
}

func TestSnapshotSelectorsTargetNestedElements(t *testing.T) {
	elements := snapshotElementsForHTML(t, `<!doctype html>
<html><head><title>selectors</title></head><body>
  <form><label><span>Email</span><input placeholder="Email"></label></form>
</body></html>`)

	assertSnapshotSelectors(t, elements, map[string]string{
		"Email": "html > body > form > label > input",
	})
}

func TestSnapshotSelectorsAvoidRepeatedNameShortcuts(t *testing.T) {
	elements := snapshotElementsForHTML(t, `<!doctype html>
<html><head><title>selectors</title></head><body>
  <form><input name="q" placeholder="Search one"><input name="q" placeholder="Search two"></form>
</body></html>`)

	assertSnapshotSelectors(t, elements, map[string]string{
		"Search one": "html > body > form > input:nth-of-type(1)",
		"Search two": "html > body > form > input:nth-of-type(2)",
	})
}

func TestSnapshotSelectorsUseEscapedUniqueShortcuts(t *testing.T) {
	elements := snapshotElementsForHTML(t, `<!doctype html>
<html><head><title>selectors</title></head><body>
  <button id="save:primary">Save</button>
  <button data-testid='quote"button'>Quote</button>
  <input name="user.email" placeholder="Email">
</body></html>`)

	assertSnapshotSelectors(t, elements, map[string]string{
		"Save":  "#save\\:primary",
		"Quote": `[data-testid="quote\"button"]`,
		"Email": `input[name="user.email"]`,
	})
}

func TestSnapshotDoesNotExposeInputValues(t *testing.T) {
	elements := snapshotElementsForHTML(t, `<!doctype html>
<html><head><title>selectors</title></head><body>
  <input name="email" value="secret@example.com" placeholder="Email">
  <input name="token" type="password" value="super-secret-token" aria-label="API token">
  <textarea name="notes" placeholder="Notes">private notes</textarea>
</body></html>`)

	seen := map[string]Element{}
	for _, element := range elements {
		seen[element.Name] = element
		if strings.Contains(element.Text, "secret@example.com") ||
			strings.Contains(element.Text, "super-secret-token") ||
			strings.Contains(element.Text, "private notes") {
			t.Fatalf("snapshot leaked input value in %#v", element)
		}
	}
	if seen["email"].Text != "Email" {
		t.Fatalf("email text = %q, want placeholder", seen["email"].Text)
	}
	if seen["token"].Text != "API token" {
		t.Fatalf("token text = %q, want aria-label", seen["token"].Text)
	}
	if seen["notes"].Text != "Notes" {
		t.Fatalf("notes text = %q, want placeholder", seen["notes"].Text)
	}
}

func TestCallContextPreservesCallDeadline(t *testing.T) {
	callDeadline := time.Now().Add(50 * time.Millisecond)
	callCtx, cancelCall := context.WithDeadline(context.Background(), callDeadline)
	defer cancelCall()
	driver := &ChromeDPDriver{ctx: context.Background()}

	runCtx, cancelRun, err := driver.callContext(callCtx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelRun()

	gotDeadline, ok := runCtx.Deadline()
	if !ok {
		t.Fatal("run context has no deadline")
	}
	if !gotDeadline.Equal(callDeadline) {
		t.Fatalf("deadline = %v, want %v", gotDeadline, callDeadline)
	}

	<-runCtx.Done()
	if !errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		t.Fatalf("run context error = %v, want context.DeadlineExceeded", runCtx.Err())
	}
}

func TestCallContextUsesEarliestDeadline(t *testing.T) {
	driverCtx, cancelDriver := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancelDriver()
	callDeadline := time.Now().Add(100 * time.Millisecond)
	callCtx, cancelCall := context.WithDeadline(context.Background(), callDeadline)
	defer cancelCall()
	driver := &ChromeDPDriver{ctx: driverCtx}

	runCtx, cancelRun, err := driver.callContext(callCtx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelRun()

	gotDeadline, ok := runCtx.Deadline()
	if !ok {
		t.Fatal("run context has no deadline")
	}
	if !gotDeadline.Equal(callDeadline) {
		t.Fatalf("deadline = %v, want earliest %v", gotDeadline, callDeadline)
	}
}

func TestWriteScreenshotOverwritesWithPrivateMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "screenshot.png")
	if err := os.WriteFile(path, []byte("old"), 0o666); err != nil {
		t.Fatal(err)
	}

	if err := writeScreenshot(path, []byte("new")); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "new" {
		t.Fatalf("body = %q, want %q", string(body), "new")
	}

	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %v, want 0600", got)
	}
}

func TestFetchFirstPageTargetID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/list" {
			t.Fatalf("path = %q, want /json/list", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"browser","type":"browser","url":""},
			{"id":"page-one","type":"page","url":"https://habr.com/ru/articles/776402/"},
			{"id":"page-two","type":"page","url":"about:blank"}
		]`))
	}))
	defer server.Close()

	id, err := fetchFirstPageTargetID(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if id.String() != "page-one" {
		t.Fatalf("id = %q, want page-one", id)
	}
}

func TestNewChromeDPDriverSurvivesParentCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/list" {
			t.Fatalf("path = %q, want /json/list", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"page-one","type":"page","url":"https://example.com"}]`))
	}))
	defer server.Close()

	parent, cancel := context.WithCancel(context.Background())
	driver, err := NewChromeDPDriver(parent, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close(context.Background())

	cancel()

	if err := driver.ctx.Err(); err != nil {
		t.Fatalf("driver context was canceled with parent: %v", err)
	}
}

func snapshotElementsForHTML(t *testing.T, html string) []Element {
	t.Helper()

	if os.Getenv("AGET_RUN_CHROME_TESTS") != "1" {
		t.Skip("set AGET_RUN_CHROME_TESTS=1 to run live Chrome snapshot tests")
	}

	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(
		context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("no-sandbox", true),
		)...,
	)
	defer cancelAllocator()

	ctx, cancel := chromedp.NewContext(allocatorCtx)
	defer cancel()

	var raw string
	if err := chromedp.Run(ctx,
		chromedp.Navigate("data:text/html,"+html),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(snapshotScript(), &raw),
	); err != nil {
		t.Fatal(err)
	}

	var elements []Element
	if err := json.Unmarshal([]byte(raw), &elements); err != nil {
		t.Fatal(err)
	}
	for _, element := range elements {
		var matches int
		script := fmt.Sprintf(`document.querySelectorAll(%q).length`, element.Selector)
		if err := chromedp.Run(ctx, chromedp.Evaluate(script, &matches)); err != nil {
			t.Fatalf("selector %q is invalid: %v", element.Selector, err)
		}
		if matches != 1 {
			t.Fatalf("selector %q matches %d elements, want 1", element.Selector, matches)
		}
	}
	return elements
}

func assertSnapshotSelectors(t *testing.T, elements []Element, want map[string]string) {
	t.Helper()

	seen := make(map[string]Element, len(elements))
	for _, element := range elements {
		seen[element.Text] = element
	}
	for text, selector := range want {
		element, ok := seen[text]
		if !ok {
			t.Fatalf("missing snapshot element with text %q in %#v", text, elements)
		}
		if element.Selector != selector {
			t.Fatalf("selector for %q = %q, want %q", text, element.Selector, selector)
		}
	}
}
