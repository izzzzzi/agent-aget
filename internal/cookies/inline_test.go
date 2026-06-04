package cookies

import (
	"testing"
)

func TestParseInline_Single(t *testing.T) {
	cookies := ParseInline("session_id=abc123")
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != "session_id" {
		t.Errorf("expected name session_id, got %s", cookies[0].Name)
	}
	if cookies[0].Value != "abc123" {
		t.Errorf("expected value abc123, got %s", cookies[0].Value)
	}
}

func TestParseInline_Multiple(t *testing.T) {
	cookies := ParseInline("a=1; b=2; c=3")
	if len(cookies) != 3 {
		t.Fatalf("expected 3 cookies, got %d", len(cookies))
	}
	if cookies[0].Value != "1" {
		t.Errorf("expected first value 1, got %s", cookies[0].Value)
	}
	if cookies[2].Value != "3" {
		t.Errorf("expected third value 3, got %s", cookies[2].Value)
	}
}

func TestParseInline_ExtraSpaces(t *testing.T) {
	cookies := ParseInline("  a = 1  ;  b = 2  ")
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}
	if cookies[0].Name != "a" || cookies[0].Value != "1" {
		t.Errorf("first cookie: got %s=%s", cookies[0].Name, cookies[0].Value)
	}
	if cookies[1].Name != "b" || cookies[1].Value != "2" {
		t.Errorf("second cookie: got %s=%s", cookies[1].Name, cookies[1].Value)
	}
}

func TestParseInline_Empty(t *testing.T) {
	cookies := ParseInline("")
	if len(cookies) != 0 {
		t.Errorf("expected 0 cookies, got %d", len(cookies))
	}

	cookies = ParseInline("   ")
	if len(cookies) != 0 {
		t.Errorf("expected 0 cookies for whitespace, got %d", len(cookies))
	}
}

func TestParseInline_NoEquals(t *testing.T) {
	// Pairs without "=" should be skipped silently
	cookies := ParseInline("a=1; b; c=3")
	if len(cookies) != 2 {
		t.Errorf("expected 2 cookies (skip malformed), got %d", len(cookies))
	}
}

func TestParseInline_EmptyName(t *testing.T) {
	cookies := ParseInline("=value; a=1")
	if len(cookies) != 1 {
		t.Errorf("expected 1 cookie (skip empty name), got %d", len(cookies))
	}
}
