package session

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRegistrySaveGetListDelete(t *testing.T) {
	registry := NewRegistry(filepath.Join(t.TempDir(), "sessions"))
	record := Record{
		SID:        "abc12345",
		Name:       "work",
		URL:        "https://example.com",
		Title:      "Example",
		BrowserPID: 123,
		DebugURL:   "http://127.0.0.1:9222",
		Headless:   true,
		CreatedAt:  time.Unix(10, 0).UTC(),
		UpdatedAt:  time.Unix(20, 0).UTC(),
	}

	if err := registry.Save(record); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := registry.Get("abc12345")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != record {
		t.Fatalf("Get() = %+v, want %+v", got, record)
	}

	records, err := registry.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(List()) = %d, want 1", len(records))
	}
	if records[0] != record {
		t.Fatalf("List()[0] = %+v, want %+v", records[0], record)
	}

	if err := registry.Delete("abc12345"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = registry.Get("abc12345")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after Delete() error = %v, want ErrNotFound", err)
	}
}

func TestRegistryRejectsInvalidSIDs(t *testing.T) {
	registry := NewRegistry(filepath.Join(t.TempDir(), "sessions"))
	invalidSIDs := []string{"../outside", "a/b", "", "abc1234/"}

	for _, sid := range invalidSIDs {
		t.Run(sid, func(t *testing.T) {
			record := Record{SID: sid}

			if err := registry.Save(record); !errors.Is(err, ErrInvalidSID) {
				t.Fatalf("Save() error = %v, want ErrInvalidSID", err)
			}

			if _, err := registry.Get(sid); !errors.Is(err, ErrInvalidSID) {
				t.Fatalf("Get() error = %v, want ErrInvalidSID", err)
			}

			if err := registry.Delete(sid); !errors.Is(err, ErrInvalidSID) {
				t.Fatalf("Delete() error = %v, want ErrInvalidSID", err)
			}
		})
	}
}

func TestRegistrySaveTightensExistingPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows does not expose Unix permission bits")
	}

	dir := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := filepath.Join(dir, "abc12345.json")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	registry := NewRegistry(dir)
	if err := registry.Save(Record{SID: "abc12345"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat(dir) error = %v", err)
	}
	if got, want := dirInfo.Mode().Perm(), os.FileMode(0o700); got != want {
		t.Fatalf("dir mode = %v, want %v", got, want)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(file) error = %v", err)
	}
	if got, want := fileInfo.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Fatalf("file mode = %v, want %v", got, want)
	}
}
