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
	invalidSIDs := []string{"../bad", "abc;rm", "abc def", "abc$(touch x)", "abc|cat", "abc/def", "", "a"}
	for _, sid := range invalidSIDs {
		t.Run(sid, func(t *testing.T) {
			err := store.Save(Record{SID: sid, Elements: []Element{{Ref: "@e1", Selector: "button"}}})
			if !errors.Is(err, ErrInvalidSID) {
				t.Fatalf("err = %v, want ErrInvalidSID", err)
			}
		})
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
