# Agent CLI Core Pack Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the v0.2 agent-friendly browser CLI surface: snapshots with refs, ref-aware actions, wait/get/scroll/press/fill, batch execution, doctor diagnostics, and updated agent help/docs.

**Architecture:** Keep the current external-agent model and JSON contract. Add focused packages for snapshot ref storage and diagnostics, extend the existing `page.Service` and `cdp.Driver` interfaces, and wire new Cobra commands through the same `lookupSession` and command timeout flow used today.

**Tech Stack:** Go 1.22+, Cobra, chromedp/CDP, local JSON state files under `AGET_STATE_DIR`, existing Node smoke/package scripts.

---

## File Structure

- Modify `internal/state/dirs.go`: add `SnapshotsDir()` for per-session snapshot ref caches.
- Create `internal/snapshot/store.go`: persist and load latest snapshot refs per `sid`.
- Create `internal/snapshot/store_test.go`: cover save/load, missing refs, private file mode, and invalid sid handling.
- Modify `internal/cdp/client.go`: add element metadata and driver primitives for snapshot, fill, press, scroll, wait, get.
- Modify `internal/cdp/chromedp.go`: implement the new driver primitives with chromedp actions and JS evaluation.
- Modify `internal/cdp/chromedp_test.go`: cover command context behavior for new primitives where practical.
- Modify `internal/page/service.go`: add service DTOs and methods for snapshot, ref resolution, fill, press, wait, scroll, get.
- Modify `internal/page/service_test.go`: cover snapshot shaping, ref cache interactions, private text handling, and service delegation.
- Modify `internal/cli/page.go`: add `snapshot`, `fill`, `press`, `wait`, `scroll`, and `get`; make `click` accept `--selector` or `--ref`.
- Modify `internal/cli/page_test.go`: cover CLI validation and JSON shape for all page commands.
- Create `internal/cli/batch.go`: implement root-level `aget batch -s SID --stdin`.
- Create `internal/cli/batch_test.go`: cover command parsing, stop-on-first-error, sensitive text omission, and success responses.
- Create `internal/doctor/doctor.go`: run non-destructive install/runtime checks and return structured results.
- Create `internal/doctor/doctor_test.go`: cover check aggregation and failure reporting with injected dependencies.
- Create `internal/cli/doctor.go`: wire `aget doctor`.
- Create `internal/cli/doctor_test.go`: cover JSON shape and help behavior.
- Modify `internal/cli/root.go`: register `batch` and `doctor`; update help arity rules for new positional commands.
- Modify `internal/agenthelp/help.go`: add new command examples to root/page help and prompt text.
- Modify `internal/agenthelp/help_test.go`: update expected command coverage.
- Modify `README.md`, `README.en.md`, and `AGENT_INSTRUCTIONS.md`: document the v0.2 workflow and examples.

## Task 1: Snapshot State Directory and Ref Store

**Files:**
- Modify: `internal/state/dirs.go`
- Create: `internal/snapshot/store.go`
- Create: `internal/snapshot/store_test.go`

- [ ] **Step 1: Write failing tests for snapshot store**

Create `internal/snapshot/store_test.go`:

```go
package snapshot

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestStoreSaveLoadAndResolve(t *testing.T) {
	store := NewStore(t.TempDir())
	record := Record{
		SID:       "abc12345",
		URL:       "https://example.com",
		Title:     "Example",
		CreatedAt: time.Unix(10, 0).UTC(),
		Elements: []Element{
			{Ref: "@e1", Kind: "button", Text: "Submit", Selector: "button[type=submit]", Visible: true, Enabled: true},
			{Ref: "@i1", Kind: "input", Selector: "input[name=email]", Visible: true, Enabled: true},
		},
	}

	if err := store.Save(record); err != nil {
		t.Fatal(err)
	}

	got, err := store.Load("abc12345")
	if err != nil {
		t.Fatal(err)
	}
	if got.SID != "abc12345" || got.URL != "https://example.com" || len(got.Elements) != 2 {
		t.Fatalf("loaded record = %#v", got)
	}

	element, err := store.Resolve("abc12345", "@i1")
	if err != nil {
		t.Fatal(err)
	}
	if element.Selector != "input[name=email]" || element.Kind != "input" {
		t.Fatalf("resolved element = %#v", element)
	}
}

func TestStoreResolveMissingRef(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Save(Record{SID: "abc12345", Elements: []Element{{Ref: "@e1", Selector: "button"}}}); err != nil {
		t.Fatal(err)
	}
	_, err := store.Resolve("abc12345", "@missing")
	if !errors.Is(err, ErrRefNotFound) {
		t.Fatalf("err = %v, want ErrRefNotFound", err)
	}
}

func TestStoreLoadMissingSnapshot(t *testing.T) {
	store := NewStore(t.TempDir())
	_, err := store.Load("abc12345")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestStoreRejectsInvalidSID(t *testing.T) {
	store := NewStore(t.TempDir())
	err := store.Save(Record{SID: "../bad", Elements: []Element{{Ref: "@e1", Selector: "button"}}})
	if !errors.Is(err, ErrInvalidSID) {
		t.Fatalf("err = %v, want ErrInvalidSID", err)
	}
}

func TestStoreWritesPrivateFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows permissions do not map to unix private mode")
	}
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Save(Record{SID: "abc12345", Elements: []Element{{Ref: "@e1", Selector: "button"}}}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "abc12345.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %v, want 0600", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/snapshot ./internal/state
```

Expected: FAIL because `internal/snapshot` and `state.SnapshotsDir` do not exist.

- [ ] **Step 3: Implement snapshot directory and store**

Add to `internal/state/dirs.go`:

```go
func SnapshotsDir() string {
	return filepath.Join(BaseDir(), "snapshots")
}
```

Create `internal/snapshot/store.go`:

```go
package snapshot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var (
	ErrNotFound    = errors.New("snapshot not found")
	ErrRefNotFound = errors.New("ref not found")
	ErrInvalidSID  = errors.New("invalid session id")
)

type Element struct {
	Ref      string `json:"ref"`
	Kind     string `json:"kind"`
	Text     string `json:"text,omitempty"`
	Selector string `json:"selector"`
	Href     string `json:"href,omitempty"`
	Type     string `json:"type,omitempty"`
	Name     string `json:"name,omitempty"`
	Visible  bool   `json:"visible"`
	Enabled  bool   `json:"enabled"`
}

type Record struct {
	SID       string    `json:"sid"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	Elements   []Element `json:"elements"`
}

type Store struct {
	dir string
}

func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) Save(record Record) error {
	if err := validateSID(record.SID); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(s.dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := s.path(record.SID)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func (s *Store) Load(sid string) (Record, error) {
	if err := validateSID(sid); err != nil {
		return Record{}, err
	}
	data, err := os.ReadFile(s.path(sid))
	if errors.Is(err, os.ErrNotExist) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Store) Resolve(sid, ref string) (Element, error) {
	record, err := s.Load(sid)
	if err != nil {
		return Element{}, err
	}
	for _, element := range record.Elements {
		if element.Ref == ref {
			return element, nil
		}
	}
	return Element{}, ErrRefNotFound
}

func (s *Store) path(sid string) string {
	return filepath.Join(s.dir, sid+".json")
}

func validateSID(sid string) error {
	if sid == "" || sid != filepath.Base(sid) {
		return ErrInvalidSID
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/snapshot ./internal/state
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/state/dirs.go internal/snapshot/store.go internal/snapshot/store_test.go
git commit -m "feat: add snapshot ref store"
```

## Task 2: CDP Driver Primitives

**Files:**
- Modify: `internal/cdp/client.go`
- Modify: `internal/cdp/chromedp.go`
- Modify: `internal/cdp/chromedp_test.go`

- [ ] **Step 1: Write failing driver tests for command context handling**

Append to `internal/cdp/chromedp_test.go`:

```go
func TestFillReturnsCanceledContextWithoutRunningAction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	driver := &ChromeDPDriver{
		ctx: context.Background(),
		run: func(ctx context.Context, actions ...chromedp.Action) error {
			t.Fatal("runner should not be called")
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
			t.Fatal("runner should not be called")
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
			t.Fatal("runner should not be called")
			return nil
		},
	}
	err := driver.Wait(ctx, WaitOptions{Text: "Ready"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait error = %v, want context.Canceled", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/cdp
```

Expected: FAIL because `Fill`, `Press`, `Wait`, and `WaitOptions` are undefined.

- [ ] **Step 3: Extend `cdp.Driver` types**

Modify `internal/cdp/client.go`:

```go
type Element struct {
	Ref      string `json:"ref,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Selector string `json:"selector"`
	Text     string `json:"text,omitempty"`
	Href     string `json:"href,omitempty"`
	Type     string `json:"type,omitempty"`
	Name     string `json:"name,omitempty"`
	Visible  bool   `json:"visible,omitempty"`
	Enabled  bool   `json:"enabled,omitempty"`
}

type SnapshotState struct {
	URL      string    `json:"url"`
	Title    string    `json:"title"`
	Elements []Element `json:"elements"`
}

type WaitOptions struct {
	Selector string
	Text     string
	URL      string
	Load     string
}

type GetOptions struct {
	Kind     string
	Selector string
}

type Driver interface {
	Read(ctx context.Context) (PageState, error)
	Snapshot(ctx context.Context) (SnapshotState, error)
	Click(ctx context.Context, selector string) error
	Type(ctx context.Context, selector, text string) error
	Fill(ctx context.Context, selector, text string) error
	Press(ctx context.Context, key string) error
	Scroll(ctx context.Context, direction string, pixels int) error
	Wait(ctx context.Context, options WaitOptions) error
	Get(ctx context.Context, options GetOptions) (string, error)
	Screenshot(ctx context.Context, path string) error
	Close(ctx context.Context) error
}
```

- [ ] **Step 4: Implement driver primitives**

Add implementations to `internal/cdp/chromedp.go`. Use this code as the starting point:

```go
func (d *ChromeDPDriver) Snapshot(ctx context.Context) (SnapshotState, error) {
	var state SnapshotState
	var raw string
	script := `(() => {
	  const selectorFor = (el) => {
	    if (el.id) return '#' + CSS.escape(el.id);
	    const name = el.getAttribute('name');
	    if (name) return el.tagName.toLowerCase() + '[name="' + CSS.escape(name) + '"]';
	    const testid = el.getAttribute('data-testid');
	    if (testid) return '[data-testid="' + CSS.escape(testid) + '"]';
	    let selector = el.tagName.toLowerCase();
	    let current = el;
	    while (current && current.parentElement && selector.split('>').length < 5) {
	      const parent = current.parentElement;
	      const same = Array.from(parent.children).filter(child => child.tagName === current.tagName);
	      if (same.length > 1) selector = current.tagName.toLowerCase() + ':nth-of-type(' + (same.indexOf(current) + 1) + ')>' + selector;
	      current = parent;
	    }
	    return selector;
	  };
	  const visible = (el) => {
	    const rect = el.getBoundingClientRect();
	    const style = window.getComputedStyle(el);
	    return rect.width > 0 && rect.height > 0 && style.visibility !== 'hidden' && style.display !== 'none';
	  };
	  const enabled = (el) => !el.disabled && el.getAttribute('aria-disabled') !== 'true';
	  const candidates = Array.from(document.querySelectorAll('a,button,input,textarea,select,[role=button],[role=link],[tabindex]'));
	  return JSON.stringify(candidates.slice(0, 200).map((el) => ({
	    kind: (el.tagName || '').toLowerCase(),
	    selector: selectorFor(el),
	    text: (el.innerText || el.value || el.getAttribute('aria-label') || el.getAttribute('placeholder') || '').trim().slice(0, 200),
	    href: el.href || '',
	    type: el.getAttribute('type') || '',
	    name: el.getAttribute('name') || '',
	    visible: visible(el),
	    enabled: enabled(el)
	  })));
	})()`
	if err := d.runActionsWithTransientRetry(ctx,
		waitForReadableBody(),
		chromedp.Location(&state.URL),
		chromedp.Title(&state.Title),
		chromedp.Evaluate(script, &raw),
	); err != nil {
		return SnapshotState{}, err
	}
	if err := json.Unmarshal([]byte(raw), &state.Elements); err != nil {
		return SnapshotState{}, err
	}
	return state, nil
}

func (d *ChromeDPDriver) Fill(ctx context.Context, selector, text string) error {
	return d.runActions(ctx,
		chromedp.SetValue(selector, "", chromedp.ByQuery),
		chromedp.SendKeys(selector, text, chromedp.ByQuery),
	)
}

func (d *ChromeDPDriver) Press(ctx context.Context, key string) error {
	return d.runActions(ctx, chromedp.KeyEvent(key))
}

func (d *ChromeDPDriver) Scroll(ctx context.Context, direction string, pixels int) error {
	if pixels <= 0 {
		pixels = 800
	}
	x, y := 0, 0
	switch direction {
	case "up":
		y = -pixels
	case "down":
		y = pixels
	case "left":
		x = -pixels
	case "right":
		x = pixels
	default:
		return fmt.Errorf("unsupported scroll direction %q", direction)
	}
	return d.runActions(ctx, chromedp.Evaluate(fmt.Sprintf(`window.scrollBy(%d, %d)`, x, y), nil))
}

func (d *ChromeDPDriver) Wait(ctx context.Context, options WaitOptions) error {
	switch {
	case options.Selector != "":
		return d.runActions(ctx, chromedp.WaitVisible(options.Selector, chromedp.ByQuery))
	case options.Text != "":
		expr := fmt.Sprintf(`document.body && document.body.innerText.includes(%q)`, options.Text)
		return d.runActions(ctx, chromedp.Poll(expr, nil, chromedp.WithPollingInterval(100*time.Millisecond)))
	case options.URL != "":
		expr := fmt.Sprintf(`location.href.includes(%q)`, strings.Trim(options.URL, "*"))
		return d.runActions(ctx, chromedp.Poll(expr, nil, chromedp.WithPollingInterval(100*time.Millisecond)))
	case options.Load != "":
		return d.runActions(ctx, chromedp.WaitReady("body", chromedp.ByQuery))
	default:
		return errors.New("wait condition required")
	}
}

func (d *ChromeDPDriver) Get(ctx context.Context, options GetOptions) (string, error) {
	var out string
	switch options.Kind {
	case "url":
		err := d.runActions(ctx, chromedp.Location(&out))
		return out, err
	case "title":
		err := d.runActions(ctx, chromedp.Title(&out))
		return out, err
	case "text":
		err := d.runActions(ctx, chromedp.Text(options.Selector, &out, chromedp.ByQuery))
		return out, err
	case "html":
		err := d.runActions(ctx, chromedp.InnerHTML(options.Selector, &out, chromedp.ByQuery))
		return out, err
	case "value":
		script := fmt.Sprintf(`document.querySelector(%q)?.value ?? ""`, options.Selector)
		err := d.runActions(ctx, chromedp.Evaluate(script, &out))
		return out, err
	default:
		return "", fmt.Errorf("unsupported get kind %q", options.Kind)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run:

```bash
gofmt -w internal/cdp/client.go internal/cdp/chromedp.go internal/cdp/chromedp_test.go
go test ./internal/cdp
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cdp/client.go internal/cdp/chromedp.go internal/cdp/chromedp_test.go
git commit -m "feat: add cdp browser primitives"
```

## Task 3: Page Service Snapshot, Ref Resolution, and Actions

**Files:**
- Modify: `internal/page/service.go`
- Modify: `internal/page/service_test.go`

- [ ] **Step 1: Update fake driver and write failing service tests**

Replace the `fakeDriver` in `internal/page/service_test.go` with a version that satisfies the expanded driver interface, then add these tests:

```go
type fakeDriver struct {
	state          cdp.PageState
	snapshot       cdp.SnapshotState
	clicked        string
	typedSelector  string
	typedText      string
	filledSelector string
	filledText     string
	pressedKey     string
	scrolledDir    string
	scrolledPixels int
	waitOptions    cdp.WaitOptions
	getOptions     cdp.GetOptions
	getValue       string
	screenshotPath string
	closed         bool
}

func (f *fakeDriver) Read(ctx context.Context) (cdp.PageState, error) { return f.state, nil }
func (f *fakeDriver) Snapshot(ctx context.Context) (cdp.SnapshotState, error) { return f.snapshot, nil }
func (f *fakeDriver) Click(ctx context.Context, selector string) error { f.clicked = selector; return nil }
func (f *fakeDriver) Type(ctx context.Context, selector, text string) error { f.typedSelector = selector; f.typedText = text; return nil }
func (f *fakeDriver) Fill(ctx context.Context, selector, text string) error { f.filledSelector = selector; f.filledText = text; return nil }
func (f *fakeDriver) Press(ctx context.Context, key string) error { f.pressedKey = key; return nil }
func (f *fakeDriver) Scroll(ctx context.Context, direction string, pixels int) error {
	f.scrolledDir = direction
	f.scrolledPixels = pixels
	return nil
}
func (f *fakeDriver) Wait(ctx context.Context, options cdp.WaitOptions) error { f.waitOptions = options; return nil }
func (f *fakeDriver) Get(ctx context.Context, options cdp.GetOptions) (string, error) {
	f.getOptions = options
	return f.getValue, nil
}
func (f *fakeDriver) Screenshot(ctx context.Context, path string) error { f.screenshotPath = path; return nil }
func (f *fakeDriver) Close(ctx context.Context) error { f.closed = true; return nil }

func TestSnapshotAssignsRefsAndNextCommands(t *testing.T) {
	driver := &fakeDriver{snapshot: cdp.SnapshotState{
		URL:   "https://example.com",
		Title: "Example",
		Elements: []cdp.Element{
			{Kind: "button", Text: "Submit", Selector: "button[type=submit]", Visible: true, Enabled: true},
			{Kind: "input", Selector: "input[name=email]", Visible: true, Enabled: true},
		},
	}}
	service := NewService(driver)
	got, err := service.Snapshot(context.Background(), SnapshotOptions{SID: "abc12345"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Elements[0].Ref != "@e1" || got.Elements[1].Ref != "@i1" {
		t.Fatalf("refs = %#v", got.Elements)
	}
	if got.NextCommands["click"] == "" || got.NextCommands["fill"] == "" {
		t.Fatalf("next commands = %#v", got.NextCommands)
	}
}

func TestRefAwareActionsResolveFromSnapshot(t *testing.T) {
	driver := &fakeDriver{}
	service := NewService(driver)
	resolver := StaticResolver(map[string]string{"@e1": "button[type=submit]", "@i1": "input[name=email]"})
	if err := service.Click(context.Background(), ActionTarget{Ref: "@e1", ResolveRef: resolver}); err != nil {
		t.Fatal(err)
	}
	if err := service.Fill(context.Background(), ActionTarget{Ref: "@i1", ResolveRef: resolver}, "secret@example.com"); err != nil {
		t.Fatal(err)
	}
	if driver.clicked != "button[type=submit]" || driver.filledSelector != "input[name=email]" {
		t.Fatalf("clicked/fill = %q/%q", driver.clicked, driver.filledSelector)
	}
	if driver.filledText != "secret@example.com" {
		t.Fatalf("filled text = %q", driver.filledText)
	}
}

func TestWaitScrollGetDelegateToDriver(t *testing.T) {
	driver := &fakeDriver{getValue: "Example"}
	service := NewService(driver)
	if err := service.Wait(context.Background(), cdp.WaitOptions{Text: "Ready"}); err != nil {
		t.Fatal(err)
	}
	if err := service.Scroll(context.Background(), "down", 500); err != nil {
		t.Fatal(err)
	}
	value, err := service.Get(context.Background(), GetOptions{Kind: "text", Target: ActionTarget{Selector: "main"}})
	if err != nil {
		t.Fatal(err)
	}
	if driver.waitOptions.Text != "Ready" || driver.scrolledDir != "down" || driver.scrolledPixels != 500 {
		t.Fatalf("driver calls not recorded: %#v", driver)
	}
	if value != "Example" || driver.getOptions.Selector != "main" {
		t.Fatalf("get value/options = %q/%#v", value, driver.getOptions)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/page
```

Expected: FAIL because service DTOs and methods are undefined.

- [ ] **Step 3: Implement page service DTOs and methods**

Modify `internal/page/service.go` to add:

```go
type RefResolver func(ref string) (string, error)

func StaticResolver(refs map[string]string) RefResolver {
	return func(ref string) (string, error) {
		selector, ok := refs[ref]
		if !ok {
			return "", ErrRefNotFound
		}
		return selector, nil
	}
}

var ErrRefNotFound = errors.New("ref not found")

type ActionTarget struct {
	Selector   string
	Ref        string
	ResolveRef RefResolver
}

func (t ActionTarget) SelectorValue() (string, error) {
	if t.Selector != "" {
		return t.Selector, nil
	}
	if t.Ref == "" || t.ResolveRef == nil {
		return "", ErrRefNotFound
	}
	return t.ResolveRef(t.Ref)
}

type SnapshotOptions struct {
	SID string
}

type SnapshotResult struct {
	OK           bool          `json:"ok"`
	SID          string        `json:"sid"`
	URL          string        `json:"url"`
	Title        string        `json:"title"`
	Elements     []cdp.Element `json:"elements"`
	NextCommands map[string]string `json:"next_commands,omitempty"`
}

type GetOptions struct {
	Kind   string
	Target ActionTarget
}

func (s *Service) Snapshot(ctx context.Context, options SnapshotOptions) (SnapshotResult, error) {
	state, err := s.driver.Snapshot(ctx)
	if err != nil {
		return SnapshotResult{}, err
	}
	elements := assignRefs(state.Elements)
	return SnapshotResult{
		OK:       true,
		SID:      options.SID,
		URL:      state.URL,
		Title:    state.Title,
		Elements: elements,
		NextCommands: map[string]string{
			"click": "aget page click -s " + options.SID + " --ref REF",
			"fill":  "aget page fill -s " + options.SID + " --ref REF --text TEXT",
			"get":   "aget page get -s " + options.SID + " text --ref REF",
		},
	}, nil
}

func assignRefs(elements []cdp.Element) []cdp.Element {
	result := make([]cdp.Element, len(elements))
	counts := map[string]int{}
	for i, element := range elements {
		prefix := "e"
		if element.Kind == "input" || element.Kind == "textarea" || element.Kind == "select" {
			prefix = "i"
		}
		counts[prefix]++
		element.Ref = fmt.Sprintf("@%s%d", prefix, counts[prefix])
		result[i] = element
	}
	return result
}

func (s *Service) Click(ctx context.Context, target ActionTarget) error {
	selector, err := target.SelectorValue()
	if err != nil {
		return err
	}
	return s.driver.Click(ctx, selector)
}

func (s *Service) Type(ctx context.Context, selector, text string) error {
	return s.driver.Type(ctx, selector, text)
}

func (s *Service) Fill(ctx context.Context, target ActionTarget, text string) error {
	selector, err := target.SelectorValue()
	if err != nil {
		return err
	}
	return s.driver.Fill(ctx, selector, text)
}

func (s *Service) Press(ctx context.Context, key string) error {
	return s.driver.Press(ctx, key)
}

func (s *Service) Wait(ctx context.Context, options cdp.WaitOptions) error {
	return s.driver.Wait(ctx, options)
}

func (s *Service) Scroll(ctx context.Context, direction string, pixels int) error {
	return s.driver.Scroll(ctx, direction, pixels)
}

func (s *Service) Get(ctx context.Context, options GetOptions) (string, error) {
	selector := options.Target.Selector
	if options.Kind != "url" && options.Kind != "title" {
		resolved, err := options.Target.SelectorValue()
		if err != nil {
			return "", err
		}
		selector = resolved
	}
	return s.driver.Get(ctx, cdp.GetOptions{Kind: options.Kind, Selector: selector})
}
```

Also update the existing `Click` call sites in tests to use `ActionTarget{Selector: "#login"}`.

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
gofmt -w internal/page/service.go internal/page/service_test.go
go test ./internal/page
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/page/service.go internal/page/service_test.go
git commit -m "feat: add page service agent actions"
```

## Task 4: Page CLI Commands and Snapshot Persistence

**Files:**
- Modify: `internal/cli/page.go`
- Modify: `internal/cli/page_test.go`

- [ ] **Step 1: Extend test drivers and write failing CLI tests**

Update `recordingDriver` and `blockingDriver` in `internal/cli/page_test.go` to satisfy the expanded `cdp.Driver` interface. Add fields for snapshot, fill, press, scroll, wait, and get.

Append tests:

```go
func TestPageSnapshotSavesRefs(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{snapshot: cdp.SnapshotState{
		URL:   "https://example.com",
		Title: "Example",
		Elements: []cdp.Element{{Kind: "button", Text: "Submit", Selector: "button[type=submit]", Visible: true, Enabled: true}},
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
		Elements: []cdp.Element{{Kind: "button", Selector: "button[type=submit]", Visible: true, Enabled: true}},
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/cli
```

Expected: FAIL because page subcommands and updated service signatures are not wired.

- [ ] **Step 3: Wire page CLI commands**

In `internal/cli/page.go`:

- Add `newPageSnapshotCommand`, `newPageFillCommand`, `newPagePressCommand`, `newPageWaitCommand`, `newPageScrollCommand`, and `newPageGetCommand`.
- Update `newPageCommand()` to register all new subcommands.
- Change `newPageClickCommand()` to accept both `--selector` and `--ref`, with exactly one required.
- Keep `page type` selector-only for backward compatibility.
- Save snapshot refs after successful `page snapshot`:

```go
store := snapshot.NewStore(state.SnapshotsDir())
record := snapshot.Record{SID: sid, URL: result.URL, Title: result.Title, CreatedAt: time.Now().UTC()}
for _, element := range result.Elements {
	record.Elements = append(record.Elements, snapshot.Element{
		Ref: element.Ref, Kind: element.Kind, Text: element.Text, Selector: element.Selector,
		Href: element.Href, Type: element.Type, Name: element.Name, Visible: element.Visible, Enabled: element.Enabled,
	})
}
if err := store.Save(record); err != nil {
	return writeError(cmd, "snapshot_save_failed", err.Error(), map[string]any{"sid": sid})
}
```

- Resolve refs with:

```go
func resolveSnapshotRef(sid string) page.RefResolver {
	return func(ref string) (string, error) {
		element, err := snapshot.NewStore(state.SnapshotsDir()).Resolve(sid, ref)
		if err != nil {
			return "", err
		}
		return element.Selector, nil
	}
}
```

- Map `snapshot.ErrRefNotFound` and `page.ErrRefNotFound` to `ref_not_found`.
- Responses for text inputs must include `text_len`, never raw text.

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
gofmt -w internal/cli/page.go internal/cli/page_test.go
go test ./internal/cli
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/page.go internal/cli/page_test.go
git commit -m "feat: add agent page commands"
```

## Task 5: Batch Command

**Files:**
- Create: `internal/cli/batch.go`
- Create: `internal/cli/batch_test.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Write failing batch CLI tests**

Create `internal/cli/batch_test.go`:

```go
package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBatchExecutesCommandsAndStopsOnFailure(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	saveTestSession(t, "abc12345", "http://127.0.0.1:9222")
	driver := &recordingDriver{getValue: "Example"}
	restore := replaceChromeDPDriverForTest(t, driver)
	defer restore()

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"batch", "-s", "abc12345", "--stdin"})
	cmd.SetIn(strings.NewReader(`[
		{"cmd":"fill","selector":"input[name=email]","text":"secret@example.com"},
		{"cmd":"press","key":"Enter"},
		{"cmd":"wait"}
	]`))
	var stdout, stderr strings.Builder
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected batch failure")
	}
	var got map[string]any
	if json.Unmarshal([]byte(stdout.String()), &got) != nil {
		t.Fatalf("stdout is not json: %s", stdout.String())
	}
	if got["ok"] != false || got["failed_index"] != float64(2) {
		t.Fatalf("batch response = %#v", got)
	}
	if strings.Contains(stdout.String(), "secret@example.com") {
		t.Fatalf("batch leaked secret text: %s", stdout.String())
	}
	if driver.filledSelector != "input[name=email]" || driver.pressedKey != "Enter" {
		t.Fatalf("driver state = %#v", driver)
	}
}

func TestBatchRequiresStdinFlag(t *testing.T) {
	stdout, stderr, err := executeForTest("batch", "-s", "abc12345")
	if err == nil {
		t.Fatal("expected error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertInvalidArgsJSON(t, stderr)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/cli
```

Expected: FAIL because `batch` is not registered.

- [ ] **Step 3: Implement batch command**

Create `internal/cli/batch.go` with:

```go
package cli

import (
	"encoding/json"
	"io"

	"github.com/izzzzzi/agent-aget/internal/cdp"
	"github.com/izzzzzi/agent-aget/internal/page"
	"github.com/spf13/cobra"
)

type batchStep struct {
	Cmd       string `json:"cmd"`
	Selector  string `json:"selector,omitempty"`
	Ref       string `json:"ref,omitempty"`
	Text      string `json:"text,omitempty"`
	Key       string `json:"key,omitempty"`
	Direction string `json:"direction,omitempty"`
	Pixels    int    `json:"pixels,omitempty"`
	Kind      string `json:"kind,omitempty"`
}

func newBatchCommand() *cobra.Command {
	var sid string
	var useStdin bool
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Execute multiple page commands",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !useStdin {
				return writeInvalidArgs(cmd, "--stdin required")
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			var steps []batchStep
			body, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return writeError(cmd, "batch_read_failed", err.Error(), map[string]any{"sid": sid})
			}
			if err := json.Unmarshal(body, &steps); err != nil {
				return writeError(cmd, "batch_parse_failed", err.Error(), map[string]any{"sid": sid})
			}
			ctx, cancel := pageOperationContext()
			defer cancel()
			driver, err := newChromeDPDriver(ctx, record.DebugURL)
			if err != nil {
				return writeError(cmd, "page_connect_failed", err.Error(), map[string]any{"sid": sid})
			}
			svc := page.NewService(driver)
			results := make([]map[string]any, 0, len(steps))
			for i, step := range steps {
				result, err := runBatchStep(ctx, sid, svc, step)
				if err != nil {
					return writeJSON(cmd, map[string]any{
						"ok": false, "sid": sid, "results": results, "failed_index": i,
						"error": map[string]any{"code": "page_action_failed", "message": err.Error()},
					})
				}
				results = append(results, result)
			}
			return writeJSON(cmd, map[string]any{"ok": true, "sid": sid, "results": results})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "read JSON command list from stdin")
	configureAgentHelp(cmd)
	return cmd
}

func runBatchStep(ctx context.Context, sid string, svc *page.Service, step batchStep) (map[string]any, error) {
	target := page.ActionTarget{Selector: step.Selector, Ref: step.Ref, ResolveRef: resolveSnapshotRef(sid)}
	switch step.Cmd {
	case "fill":
		if err := svc.Fill(ctx, target, step.Text); err != nil { return nil, err }
		return map[string]any{"ok": true, "cmd": "fill", "text_len": len(step.Text)}, nil
	case "press":
		if err := svc.Press(ctx, step.Key); err != nil { return nil, err }
		return map[string]any{"ok": true, "cmd": "press", "key": step.Key}, nil
	case "wait":
		if err := svc.Wait(ctx, cdp.WaitOptions{Selector: step.Selector, Text: step.Text}); err != nil { return nil, err }
		return map[string]any{"ok": true, "cmd": "wait"}, nil
	case "snapshot":
		result, err := svc.Snapshot(ctx, page.SnapshotOptions{SID: sid})
		if err != nil { return nil, err }
		return map[string]any{"ok": true, "cmd": "snapshot", "elements": result.Elements}, nil
	default:
		return nil, fmt.Errorf("unsupported batch cmd %q", step.Cmd)
	}
}
```

Add missing imports (`context`, `fmt`) and register `newBatchCommand()` in `NewRootCommand`.

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
gofmt -w internal/cli/batch.go internal/cli/batch_test.go internal/cli/root.go
go test ./internal/cli
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/batch.go internal/cli/batch_test.go internal/cli/root.go
git commit -m "feat: add batch command"
```

## Task 6: Doctor Diagnostics

**Files:**
- Create: `internal/doctor/doctor.go`
- Create: `internal/doctor/doctor_test.go`
- Create: `internal/cli/doctor.go`
- Create: `internal/cli/doctor_test.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Write failing doctor package tests**

Create `internal/doctor/doctor_test.go`:

```go
package doctor

import (
	"errors"
	"testing"
)

func TestRunAggregatesChecks(t *testing.T) {
	runner := Runner{
		Checks: []Check{
			{Name: "state_dir", Run: func() Detail { return Detail{OK: true, Message: "writable"} }},
			{Name: "browser", Run: func() Detail { return Detail{OK: false, Message: "missing", Remediation: "run `aget browser install`"} }},
		},
	}
	got := runner.Run()
	if got.OK {
		t.Fatal("overall OK = true, want false")
	}
	if len(got.Checks) != 2 || got.Checks[1].Remediation == "" {
		t.Fatalf("checks = %#v", got.Checks)
	}
}

func TestDetailFromError(t *testing.T) {
	got := DetailFromError(errors.New("boom"), "repair")
	if got.OK || got.Message != "boom" || got.Remediation != "repair" {
		t.Fatalf("detail = %#v", got)
	}
}
```

- [ ] **Step 2: Write failing CLI test**

Create `internal/cli/doctor_test.go`:

```go
package cli

import (
	"encoding/json"
	"testing"
)

func TestDoctorReturnsJSON(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	stdout, stderr, err := executeForTest("doctor")
	if err != nil && stdout == "" {
		t.Fatalf("doctor returned no JSON: err=%v stderr=%s", err, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not json: %s", stdout)
	}
	if got["ok"] == nil || got["checks"] == nil {
		t.Fatalf("doctor response = %#v", got)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
go test ./internal/doctor ./internal/cli
```

Expected: FAIL because doctor package and CLI command do not exist.

- [ ] **Step 4: Implement doctor package and CLI**

Create `internal/doctor/doctor.go`:

```go
package doctor

type Detail struct {
	Name        string `json:"name,omitempty"`
	OK          bool   `json:"ok"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

type Result struct {
	OK     bool     `json:"ok"`
	Checks []Detail `json:"checks"`
}

type Check struct {
	Name string
	Run  func() Detail
}

type Runner struct {
	Checks []Check
}

func (r Runner) Run() Result {
	result := Result{OK: true, Checks: make([]Detail, 0, len(r.Checks))}
	for _, check := range r.Checks {
		detail := check.Run()
		detail.Name = check.Name
		if !detail.OK {
			result.OK = false
		}
		result.Checks = append(result.Checks, detail)
	}
	return result
}

func DetailFromError(err error, remediation string) Detail {
	if err == nil {
		return Detail{OK: true, Message: "ok"}
	}
	return Detail{OK: false, Message: err.Error(), Remediation: remediation}
}
```

Create `internal/cli/doctor.go` with checks for writable state dirs and browser resolution. Keep launch/CDP checks as best-effort if browser resolution succeeds:

```go
package cli

import (
	"os"
	"path/filepath"

	"github.com/izzzzzi/agent-aget/internal/browser"
	"github.com/izzzzzi/agent-aget/internal/doctor"
	"github.com/izzzzzi/agent-aget/internal/state"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check aget installation and runtime readiness",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result := doctor.Runner{Checks: []doctor.Check{
				{Name: "state_dir", Run: checkWritableDir(state.BaseDir())},
				{Name: "sessions_dir", Run: checkWritableDir(state.SessionsDir())},
				{Name: "artifacts_dir", Run: checkWritableDir(state.ArtifactsDir())},
				{Name: "snapshots_dir", Run: checkWritableDir(state.SnapshotsDir())},
				{Name: "browser", Run: func() doctor.Detail {
					resolved, err := browser.Resolve("")
					if err != nil {
						return doctor.DetailFromError(err, "run `aget browser install`, set AGET_BROWSER_PATH, or pass --browser-path to open")
					}
					return doctor.Detail{OK: true, Message: resolved.Browser + " at " + resolved.Path}
				}},
			}}.Run()
			return writeJSON(cmd, result)
		},
	}
	configureAgentHelp(cmd)
	return cmd
}

func checkWritableDir(dir string) func() doctor.Detail {
	return func() doctor.Detail {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return doctor.DetailFromError(err, "check directory permissions")
		}
		probe := filepath.Join(dir, ".doctor-write-test")
		if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
			return doctor.DetailFromError(err, "check directory permissions")
		}
		_ = os.Remove(probe)
		return doctor.Detail{OK: true, Message: "writable"}
	}
}
```

Register `newDoctorCommand()` in `NewRootCommand`.

- [ ] **Step 5: Run tests to verify they pass**

Run:

```bash
gofmt -w internal/doctor/doctor.go internal/doctor/doctor_test.go internal/cli/doctor.go internal/cli/doctor_test.go internal/cli/root.go
go test ./internal/doctor ./internal/cli
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/doctor/doctor.go internal/doctor/doctor_test.go internal/cli/doctor.go internal/cli/doctor_test.go internal/cli/root.go
git commit -m "feat: add doctor diagnostics"
```

## Task 7: Agent Help and Documentation

**Files:**
- Modify: `internal/agenthelp/help.go`
- Modify: `internal/agenthelp/help_test.go`
- Modify: `README.md`
- Modify: `README.en.md`
- Modify: `AGENT_INSTRUCTIONS.md`

- [ ] **Step 1: Write failing help coverage test**

Add to `internal/agenthelp/help_test.go`:

```go
func TestRootHelpIncludesAgentCoreCommands(t *testing.T) {
	commands := RootHelp().Commands
	for _, key := range []string{"page_snapshot", "page_fill", "page_wait", "page_get", "batch", "doctor"} {
		if commands[key] == "" {
			t.Fatalf("command %s missing from root help: %#v", key, commands)
		}
	}
}

func TestPageHelpIncludesRefWorkflow(t *testing.T) {
	payload, ok := GroupHelp("page")
	if !ok {
		t.Fatal("page help missing")
	}
	for _, key := range []string{"snapshot", "click_ref", "fill", "wait", "get"} {
		if payload.Commands[key] == "" {
			t.Fatalf("command %s missing from page help: %#v", key, payload.Commands)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/agenthelp
```

Expected: FAIL because help payloads do not include new commands.

- [ ] **Step 3: Update help and prompt text**

Modify `internal/agenthelp/help.go`:

- In root workflow, prefer `page snapshot` after opening.
- Add root commands:

```go
"doctor":        "aget doctor",
"page_snapshot": "aget page snapshot -s SID",
"page_fill":     "aget page fill -s SID --ref REF --text TEXT",
"page_wait":     "aget page wait -s SID --text TEXT",
"page_get":      "aget page get -s SID text --ref REF",
"batch":         "aget batch -s SID --stdin",
```

- In page group workflow, explain `snapshot -> ref action -> read/get/screenshot`.
- Add page commands:

```go
"snapshot":  "aget page snapshot -s SID",
"click_ref": "aget page click -s SID --ref REF",
"fill":      "aget page fill -s SID --ref REF --text TEXT",
"press":     "aget page press -s SID --key Enter",
"wait":      "aget page wait -s SID --text TEXT",
"scroll":    "aget page scroll -s SID --direction down --px 800",
"get":       "aget page get -s SID text --ref REF",
```

- Update `Prompt()` to instruct agents to use `snapshot` and refs before selectors when possible.

- [ ] **Step 4: Update docs**

Add examples to `README.md`, `README.en.md`, and `AGENT_INSTRUCTIONS.md`:

```text
Prefer `aget page snapshot -s SID` for actions. It returns refs like `@e1` and `@i1`.
Use `aget page click -s SID --ref @e1` and `aget page fill -s SID --ref @i1 --text TEXT`.
Use `aget page wait`, `aget page get`, `aget page scroll`, and `aget batch` for multi-step workflows.
Run `aget doctor` when install or browser startup fails.
```

- [ ] **Step 5: Run tests to verify they pass**

Run:

```bash
gofmt -w internal/agenthelp/help.go internal/agenthelp/help_test.go
go test ./internal/agenthelp
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/agenthelp/help.go internal/agenthelp/help_test.go README.md README.en.md AGENT_INSTRUCTIONS.md
git commit -m "docs: document agent cli core commands"
```

## Task 8: Full Verification and Release Contract

**Files:**
- Modify if needed: `scripts/smoke-test.js`
- Modify if needed: `scripts/release-contract-test.js`

- [ ] **Step 1: Run full Go tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run package checks**

Run:

```bash
npm test
```

Expected: PASS for smoke and browser install tests.

- [ ] **Step 3: Run release contract if available**

Run:

```bash
npm run release:contract
```

Expected: PASS. If it fails because command help output changed, update `scripts/release-contract-test.js` with the new expected command map and rerun.

- [ ] **Step 4: Run full check**

Run:

```bash
npm run check
```

Expected: PASS. This runs gofmt check, go vet, Go tests, npm smoke tests, browser install tests, and dry pack.

- [ ] **Step 5: Commit final contract updates**

If Step 3 or Step 4 required script/package metadata changes:

```bash
git add scripts/smoke-test.js scripts/release-contract-test.js package.json README.md README.en.md AGENT_INSTRUCTIONS.md
git commit -m "test: update agent cli release contract"
```

If there are no final changes, do not create an empty commit.

## Self-Review

- Spec coverage: snapshot refs are covered by Tasks 1, 3, and 4; fill/press/wait/scroll/get by Tasks 2-4; batch by Task 5; doctor by Task 6; help/docs by Task 7; verification by Task 8.
- Scope check: the plan excludes Webwright-style autonomous task runners, model-backed extraction, cloud orchestration, proxy/captcha work, and durable refs across navigation, matching the spec non-goals.
- Placeholder scan: no TBD/TODO placeholders remain. Each task includes concrete files, test snippets, implementation snippets, commands, expected outcomes, and commit commands.
- Type consistency: `cdp.Element`, `snapshot.Element`, `page.ActionTarget`, `page.SnapshotResult`, and batch step field names are consistent across tasks.
