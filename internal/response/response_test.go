package response

import (
	"encoding/json"
	"testing"
)

func TestMarshalAppendsNewline(t *testing.T) {
	body, err := Marshal(map[string]any{"ok": true, "sid": "abc12345"})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"ok\":true,\"sid\":\"abc12345\"}\n"
	if string(body) != want {
		t.Fatalf("body = %q, want %q", string(body), want)
	}
}

func TestMarshalErrorShape(t *testing.T) {
	body, err := MarshalError("session_not_found", "session missing", map[string]any{"sid": "abc12345"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != false {
		t.Fatalf("ok = %v, want false", got["ok"])
	}
	if got["code"] != "session_not_found" {
		t.Fatalf("code = %v", got["code"])
	}
	if got["message"] != "session missing" {
		t.Fatalf("message = %v", got["message"])
	}
	details, ok := got["details"].(map[string]any)
	if !ok {
		t.Fatalf("details = %v, want object", got["details"])
	}
	if details["sid"] != "abc12345" {
		t.Fatalf("details.sid = %v", details["sid"])
	}
}
