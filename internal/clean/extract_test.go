package clean

import (
	"reflect"
	"testing"
)

func TestExtractEmpty(t *testing.T) {
	for _, in := range [][]string{nil, {}} {
		kept, dropped := Extract(in)
		if len(kept) != 0 {
			t.Errorf("Extract(%v): expected empty kept, got %v", in, kept)
		}
		if kept == nil {
			t.Errorf("Extract(%v): kept must be non-nil", in)
		}
		if dropped != 0 {
			t.Errorf("Extract(%v): expected 0 dropped, got %d", in, dropped)
		}
	}
}

func TestExtractNoBoilerplateUnchanged(t *testing.T) {
	in := []string{
		"Introduction to widgets",
		"A widget is a small component.",
		"Widgets can be composed together.",
	}
	kept, dropped := Extract(in)
	if dropped != 0 {
		t.Errorf("expected 0 dropped, got %d", dropped)
	}
	if !reflect.DeepEqual(kept, in) {
		t.Errorf("content-only input must be returned identically.\n got: %v\nwant: %v", kept, in)
	}
}

func TestExtractDropsBoilerplate(t *testing.T) {
	in := []string{
		"We use cookies to improve your experience.",
		"Accept all",
		"Reject all",
		"Skip to main content",
		"Real article heading",
		"Back to top",
		"Cookie Policy",
		"The actual body text of the page.",
	}
	want := []string{
		"Real article heading",
		"The actual body text of the page.",
	}
	kept, dropped := Extract(in)
	if !reflect.DeepEqual(kept, want) {
		t.Errorf("kept mismatch.\n got: %v\nwant: %v", kept, want)
	}
	if dropped != 6 {
		t.Errorf("expected 6 dropped, got %d", dropped)
	}
}

func TestExtractCollapsesDuplicates(t *testing.T) {
	in := []string{"Home", "Home", "About", "Home", "About"}
	want := []string{"Home", "About"}
	kept, dropped := Extract(in)
	if !reflect.DeepEqual(kept, want) {
		t.Errorf("kept mismatch.\n got: %v\nwant: %v", kept, want)
	}
	if dropped != 3 {
		t.Errorf("expected 3 dropped, got %d", dropped)
	}
}

func TestExtractIdempotent(t *testing.T) {
	in := []string{
		"We use cookies.",
		"Heading",
		"Heading",
		"Body paragraph one.",
		"Back to top",
		"Body paragraph two.",
	}
	once, _ := Extract(in)
	twice, dropped := Extract(once)
	if !reflect.DeepEqual(once, twice) {
		t.Errorf("Extract not idempotent.\n once: %v\ntwice: %v", once, twice)
	}
	if dropped != 0 {
		t.Errorf("second pass should drop nothing, got %d", dropped)
	}
}

func TestExtractPreservesSentenceContainingKeyword(t *testing.T) {
	// "cookies" inside a real sentence must survive; only whole-line banners go.
	in := []string{
		"Our recipe for chocolate chip cookies is famous.",
		"You can accept all major credit cards here.",
	}
	kept, dropped := Extract(in)
	if dropped != 0 {
		t.Errorf("expected 0 dropped, got %d (kept=%v)", dropped, kept)
	}
	if !reflect.DeepEqual(kept, in) {
		t.Errorf("sentences containing keywords must be preserved.\n got: %v\nwant: %v", kept, in)
	}
}

func TestExtractUnicodeSafe(t *testing.T) {
	in := []string{"Привет, мир 🌍", "日本語のテキスト", "Привет, мир 🌍"}
	want := []string{"Привет, мир 🌍", "日本語のテキスト"}
	kept, dropped := Extract(in)
	if !reflect.DeepEqual(kept, want) {
		t.Errorf("unicode mismatch.\n got: %v\nwant: %v", kept, want)
	}
	if dropped != 1 {
		t.Errorf("expected 1 dropped, got %d", dropped)
	}
}
