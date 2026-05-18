package session

import (
	"errors"
	"path/filepath"
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
