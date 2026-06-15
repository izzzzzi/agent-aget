package inspect

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionsAPI(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	srv := New(0) // port doesn't matter; we test handlers directly
	handler := http.HandlerFunc(srv.handleSessions)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp []any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if len(resp) != 0 {
		t.Fatalf("expected empty session list, got %d items", len(resp))
	}
}

func TestSnapshotAPI(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	srv := New(0)
	handler := http.HandlerFunc(srv.handleSnapshot)

	req := httptest.NewRequest(http.MethodGet, "/api/snapshot/missing-sid", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing snapshot, got %d", w.Code)
	}
}

func TestIndexHTML(t *testing.T) {
	t.Setenv("AGET_STATE_DIR", t.TempDir())
	srv := New(0)
	handler := http.HandlerFunc(srv.handleIndex)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !containsHtml(w.Body.String(), "aget inspect") {
		t.Fatal("dashboard HTML missing title")
	}
}

func containsHtml(s, sub string) bool {
	for start := 0; start+len(sub) <= len(s); start++ {
		match := true
		for i := 0; i < len(sub); i++ {
			if s[start+i] != sub[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
