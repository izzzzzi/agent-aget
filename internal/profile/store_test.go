package profile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")
	store := NewStore(path)

	if err := store.Create("ozon", true); err != nil {
		t.Fatal(err)
	}

	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Name != "ozon" || !records[0].CookiesImported {
		t.Fatalf("records = %#v", records)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("profiles.json is empty")
	}
}

func TestCreateDuplicate(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "profiles.json"))
	if err := store.Create("ozon", false); err != nil {
		t.Fatal(err)
	}
	err := store.Create("ozon", false)
	if !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
}

func TestGet(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "profiles.json"))
	if err := store.Create("samokat", true); err != nil {
		t.Fatal(err)
	}

	rec, err := store.Get("samokat")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Name != "samokat" || !rec.CookiesImported {
		t.Fatalf("record = %#v", rec)
	}

	_, err = store.Get("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "profiles.json"))
	if err := store.Create("ozon", false); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("ozon"); err != nil {
		t.Fatal(err)
	}
	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("records = %#v", records)
	}

	err = store.Delete("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestEmptyList(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "profiles.json"))
	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("records = %#v", records)
	}
}
