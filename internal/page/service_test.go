package page

import (
	"context"
	"errors"
	"testing"

	"github.com/izzzzzi/agent-aget/internal/cdp"
	"github.com/izzzzzi/agent-aget/internal/snapshot"
)

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
	getResult      string
	screenshotPath string
	findCriteria   cdp.FindCriteria
	findSelector   string
	findErr        error
	closed         bool
}

func (f *fakeDriver) Read(ctx context.Context) (cdp.PageState, error) {
	return f.state, nil
}

func (f *fakeDriver) Snapshot(ctx context.Context) (cdp.SnapshotState, error) {
	return f.snapshot, nil
}

func (f *fakeDriver) Find(ctx context.Context, criteria cdp.FindCriteria) (string, error) {
	f.findCriteria = criteria
	return f.findSelector, f.findErr
}

func (f *fakeDriver) Click(ctx context.Context, selector string) error {
	f.clicked = selector
	return nil
}

func (f *fakeDriver) ClickForce(ctx context.Context, selector string) error {
	f.clicked = selector
	return nil
}

func (f *fakeDriver) WaitAppear(ctx context.Context, selector string) error {
	f.waitOptions = cdp.WaitOptions{Selector: selector}
	return nil
}

func (f *fakeDriver) Type(ctx context.Context, selector, text string) error {
	f.typedSelector = selector
	f.typedText = text
	return nil
}

func (f *fakeDriver) Fill(ctx context.Context, selector, text string) error {
	f.filledSelector = selector
	f.filledText = text
	return nil
}

func (f *fakeDriver) Press(ctx context.Context, key string) error {
	f.pressedKey = key
	return nil
}

func (f *fakeDriver) Scroll(ctx context.Context, direction string, pixels int) error {
	f.scrolledDir = direction
	f.scrolledPixels = pixels
	return nil
}

func (f *fakeDriver) Wait(ctx context.Context, options cdp.WaitOptions) error {
	f.waitOptions = options
	return nil
}

func (f *fakeDriver) Get(ctx context.Context, options cdp.GetOptions) (string, error) {
	f.getOptions = options
	return f.getResult, nil
}

func (f *fakeDriver) Select(ctx context.Context, selector, value string) error { return nil }

func (f *fakeDriver) Is(ctx context.Context, selector, prop string) (bool, error) { return true, nil }

func (f *fakeDriver) Eval(ctx context.Context, expression string) (string, error) { return `"ok"`, nil }

func (f *fakeDriver) Check(ctx context.Context, selector string) error { return nil }

func (f *fakeDriver) Uncheck(ctx context.Context, selector string) error { return nil }

func (f *fakeDriver) Hover(ctx context.Context, selector string) error { return nil }

func (f *fakeDriver) Focus(ctx context.Context, selector string) error { return nil }

func (f *fakeDriver) Upload(ctx context.Context, selector string, files []string) error { return nil }

func (f *fakeDriver) DialogAccept(ctx context.Context, promptText string) error { return nil }

func (f *fakeDriver) DialogDismiss(ctx context.Context) error { return nil }

func (f *fakeDriver) Screenshot(ctx context.Context, path string) error {
	f.screenshotPath = path
	return nil
}

func (f *fakeDriver) Close(ctx context.Context) error {
	f.closed = true
	return nil
}

func TestReadLimitsTextLines(t *testing.T) {
	driver := &fakeDriver{
		state: cdp.PageState{
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
	if len(got.Text) != 2 || got.Text[0] != "one" || got.Text[1] != "two" {
		t.Fatalf("text = %#v", got.Text)
	}
	if got.Truncated != true {
		t.Fatalf("truncated = %v", got.Truncated)
	}
	if len(got.Links) != 1 || got.Links[0].Text != "More" {
		t.Fatalf("links = %#v", got.Links)
	}
	if got.CleanEnabled {
		t.Fatalf("clean must be off by default, got CleanEnabled=true")
	}
}

func TestReadCleanDropsBoilerplate(t *testing.T) {
	driver := &fakeDriver{
		state: cdp.PageState{
			URL:   "https://example.com",
			Title: "Example",
			Text:  "We use cookies to improve your experience.\nAccept all\nReal heading\nBody text.",
		},
	}
	service := NewService(driver)

	// Default (clean off): boilerplate is preserved.
	plain, err := service.Read(context.Background(), ReadOptions{Limit: 80})
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.Text) != 4 {
		t.Fatalf("clean off: expected 4 lines, got %#v", plain.Text)
	}

	// Clean on: boilerplate dropped, content kept.
	got, err := service.Read(context.Background(), ReadOptions{Limit: 80, Clean: true})
	if err != nil {
		t.Fatal(err)
	}
	if !got.CleanEnabled {
		t.Fatalf("CleanEnabled = false, want true")
	}
	if got.CleanDropped != 2 {
		t.Fatalf("CleanDropped = %d, want 2", got.CleanDropped)
	}
	want := []string{"Real heading", "Body text."}
	if len(got.Text) != 2 || got.Text[0] != want[0] || got.Text[1] != want[1] {
		t.Fatalf("text = %#v, want %#v", got.Text, want)
	}
}

func TestClickAndTypeDelegateToDriver(t *testing.T) {
	driver := &fakeDriver{}
	service := NewService(driver)
	if err := service.Click(context.Background(), "#login"); err != nil {
		t.Fatal(err)
	}
	if err := service.Type(context.Background(), "#email", "me@example.com"); err != nil {
		t.Fatal(err)
	}
	if driver.clicked != "#login" {
		t.Fatalf("clicked = %q", driver.clicked)
	}
	if driver.typedSelector != "#email" || driver.typedText != "me@example.com" {
		t.Fatalf("typed selector/text = %q/%q", driver.typedSelector, driver.typedText)
	}
}

type fakeResolver struct {
	elements map[string]snapshot.Element
	saved    snapshot.Record
	prev     snapshot.Record
	prevErr  error
}

func (f *fakeResolver) Save(record snapshot.Record) error {
	f.saved = record
	return nil
}

func (f *fakeResolver) Load(sid string) (snapshot.Record, error) {
	if f.prevErr != nil {
		return snapshot.Record{}, f.prevErr
	}
	return f.prev, nil
}

func (f *fakeResolver) Resolve(sid, ref string) (snapshot.Element, error) {
	element, ok := f.elements[ref]
	if !ok {
		return snapshot.Element{}, snapshot.ErrRefNotFound
	}
	return element, nil
}

func TestSnapshotAssignsRefsAndNextCommands(t *testing.T) {
	driver := &fakeDriver{
		snapshot: cdp.SnapshotState{
			URL:   "https://example.com/login",
			Title: "Login",
			Elements: []cdp.Element{
				{Kind: "button", Selector: "button[type=submit]", Text: "Sign in", Visible: true, Enabled: true},
				{Kind: "input", Selector: "input[name=email]", Type: "email", Name: "email", Visible: true, Enabled: true},
				{Kind: "a", Selector: "a.help", Text: "Help", Href: "https://example.com/help", Visible: true, Enabled: true},
				{Kind: "textarea", Selector: "textarea[name=note]", Name: "note", Visible: true, Enabled: true},
			},
		},
	}
	resolver := &fakeResolver{}
	service := NewServiceWithRefs(driver, resolver)

	got, err := service.Snapshot(context.Background(), SnapshotOptions{SID: "abc12345"})
	if err != nil {
		t.Fatal(err)
	}

	if !got.OK || got.SID != "abc12345" || got.URL != "https://example.com/login" || got.Title != "Login" {
		t.Fatalf("snapshot result = %#v", got)
	}
	wantRefs := []string{"@e1", "@i1", "@e2", "@i2"}
	if len(got.Elements) != len(wantRefs) {
		t.Fatalf("elements = %#v", got.Elements)
	}
	for i, want := range wantRefs {
		if got.Elements[i].Ref != want {
			t.Fatalf("element %d ref = %q, want %q", i, got.Elements[i].Ref, want)
		}
	}
	if len(got.NextCommands) == 0 {
		t.Fatalf("next commands = %#v", got.NextCommands)
	}
	if got.NextCommands[0] != "aget page get -s abc12345 url" {
		t.Fatalf("first next command = %q", got.NextCommands[0])
	}
	if got.NextCommands[3] != "aget page fill -s abc12345 --ref @i1 --text TEXT" {
		t.Fatalf("fill next command = %q", got.NextCommands[3])
	}
	if resolver.saved.SID != "abc12345" || len(resolver.saved.Elements) != 4 {
		t.Fatalf("saved record = %#v", resolver.saved)
	}
	if resolver.saved.Elements[1].Selector != "input[name=email]" || resolver.saved.Elements[1].Ref != "@i1" {
		t.Fatalf("saved input = %#v", resolver.saved.Elements[1])
	}
}

func TestSnapshotDiffComputesDelta(t *testing.T) {
	driver := &fakeDriver{
		snapshot: cdp.SnapshotState{
			URL:   "https://example.com",
			Title: "Example",
			Elements: []cdp.Element{
				{Kind: "button", Name: "Submit", Selector: "#go", Visible: true, Enabled: true},
				{Kind: "link", Text: "About", Selector: "a#about", Visible: true, Enabled: true},
			},
		},
	}
	resolver := &fakeResolver{
		prev: snapshot.Record{
			SID: "abc12345",
			Elements: []snapshot.Element{
				// same identity as button but disabled before -> Changed
				{Kind: "button", Name: "Submit", Selector: "#go", Visible: true, Enabled: false},
				// present before, gone now -> Removed
				{Kind: "link", Text: "Home", Selector: "a#home", Visible: true, Enabled: true},
			},
		},
	}
	service := NewServiceWithRefs(driver, resolver)

	// Without --diff: no diff field.
	plain, err := service.Snapshot(context.Background(), SnapshotOptions{SID: "abc12345"})
	if err != nil {
		t.Fatal(err)
	}
	if plain.Diff != nil {
		t.Fatalf("diff should be nil without --diff, got %#v", plain.Diff)
	}

	// With --diff: added (About), removed (Home), changed (Submit enabled).
	got, err := service.Snapshot(context.Background(), SnapshotOptions{SID: "abc12345", Diff: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Diff == nil {
		t.Fatal("expected diff, got nil")
	}
	if len(got.Diff.Added) != 1 || got.Diff.Added[0].Text != "About" {
		t.Fatalf("added = %#v", got.Diff.Added)
	}
	if len(got.Diff.Removed) != 1 || got.Diff.Removed[0].Text != "Home" {
		t.Fatalf("removed = %#v", got.Diff.Removed)
	}
	if len(got.Diff.Changed) != 1 || got.Diff.Changed[0].After.Enabled != true {
		t.Fatalf("changed = %#v", got.Diff.Changed)
	}
}

func TestClickAndFillResolveRefs(t *testing.T) {
	driver := &fakeDriver{}
	resolver := &fakeResolver{elements: map[string]snapshot.Element{
		"@e1": {Ref: "@e1", Selector: "button[type=submit]"},
		"@i1": {Ref: "@i1", Selector: "input[name=email]"},
	}}
	service := NewServiceWithRefs(driver, resolver)

	if err := service.ClickTarget(context.Background(), ActionTarget{SID: "abc12345", Ref: "@e1"}); err != nil {
		t.Fatal(err)
	}
	if err := service.Fill(context.Background(), FillOptions{
		Target: ActionTarget{SID: "abc12345", Ref: "@i1"},
		Text:   "me@example.com",
	}); err != nil {
		t.Fatal(err)
	}

	if driver.clicked != "button[type=submit]" {
		t.Fatalf("clicked = %q", driver.clicked)
	}
	if driver.filledSelector != "input[name=email]" || driver.filledText != "me@example.com" {
		t.Fatalf("filled selector/text = %q/%q", driver.filledSelector, driver.filledText)
	}
}

func TestMissingRefReturnsErrRefNotFound(t *testing.T) {
	service := NewServiceWithRefs(&fakeDriver{}, &fakeResolver{elements: map[string]snapshot.Element{}})
	err := service.ClickTarget(context.Background(), ActionTarget{SID: "abc12345", Ref: "@missing"})
	if !errors.Is(err, ErrRefNotFound) {
		t.Fatalf("err = %v, want ErrRefNotFound", err)
	}
}

func TestWaitScrollPressAndGetDelegateToDriver(t *testing.T) {
	driver := &fakeDriver{getResult: "Ready"}
	resolver := &fakeResolver{elements: map[string]snapshot.Element{
		"@e1": {Ref: "@e1", Selector: "#status"},
	}}
	service := NewServiceWithRefs(driver, resolver)

	if err := service.Wait(context.Background(), WaitOptions{
		Target: ActionTarget{SID: "abc12345", Ref: "@e1"},
		Text:   "Ready",
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.Scroll(context.Background(), ScrollOptions{Direction: "down", Pixels: 400}); err != nil {
		t.Fatal(err)
	}
	if err := service.Press(context.Background(), PressOptions{Key: "Enter"}); err != nil {
		t.Fatal(err)
	}
	value, err := service.Get(context.Background(), GetOptions{
		Kind:   "text",
		Target: ActionTarget{SID: "abc12345", Ref: "@e1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if driver.waitOptions.Selector != "#status" || driver.waitOptions.Text != "Ready" {
		t.Fatalf("wait options = %#v", driver.waitOptions)
	}
	if driver.scrolledDir != "down" || driver.scrolledPixels != 400 {
		t.Fatalf("scroll = %q/%d", driver.scrolledDir, driver.scrolledPixels)
	}
	if driver.pressedKey != "Enter" {
		t.Fatalf("pressed = %q", driver.pressedKey)
	}
	if driver.getOptions.Kind != "text" || driver.getOptions.Selector != "#status" || value != "Ready" {
		t.Fatalf("get options/result = %#v/%q", driver.getOptions, value)
	}
}

func TestGetURLAndTitleDoNotRequireTarget(t *testing.T) {
	driver := &fakeDriver{getResult: "https://example.com"}
	service := NewServiceWithRefs(driver, &fakeResolver{})

	value, err := service.Get(context.Background(), GetOptions{Kind: "url"})
	if err != nil {
		t.Fatal(err)
	}
	if value != "https://example.com" || driver.getOptions.Kind != "url" || driver.getOptions.Selector != "" {
		t.Fatalf("get url options/result = %#v/%q", driver.getOptions, value)
	}
}

func TestGetTextRequiresTarget(t *testing.T) {
	service := NewServiceWithRefs(&fakeDriver{}, &fakeResolver{})
	_, err := service.Get(context.Background(), GetOptions{Kind: "text"})
	if !errors.Is(err, ErrTargetRequired) {
		t.Fatalf("err = %v, want ErrTargetRequired", err)
	}
}
