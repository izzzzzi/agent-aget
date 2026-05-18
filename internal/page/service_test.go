package page

import (
	"context"
	"testing"

	"github.com/izzzzzi/agent-aget/internal/cdp"
)

type fakeDriver struct {
	state          cdp.PageState
	clicked        string
	typedSelector  string
	typedText      string
	screenshotPath string
	closed         bool
}

func (f *fakeDriver) Read(ctx context.Context) (cdp.PageState, error) {
	return f.state, nil
}

func (f *fakeDriver) Click(ctx context.Context, selector string) error {
	f.clicked = selector
	return nil
}

func (f *fakeDriver) Type(ctx context.Context, selector, text string) error {
	f.typedSelector = selector
	f.typedText = text
	return nil
}

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
