package cookies

import (
	"context"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/network"
)

func TestDomainForURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://github.com/settings", "github.com"},
		{"http://example.com", "example.com"},
		{"https://sub.example.com:8080/path?a=1", "sub.example.com"},
		{"https://localhost:3000/", "localhost"},
		{"just-text", "just-text"},
		{"", ""},
		{"https://192.168.1.1:8443/", "192.168.1.1"},
	}

	for _, tt := range tests {
		got := DomainForURL(tt.url)
		if got != tt.expected {
			t.Errorf("DomainForURL(%q) = %q, want %q", tt.url, got, tt.expected)
		}
	}
}

func TestApplyDomain(t *testing.T) {
	// Call the function to ensure it compiles and runs
	// Tests that empty domains get filled
	EmptyDomainTest(t)
}

func EmptyDomainTest(t *testing.T) {
	// Parse some inline cookies that have no domain
	cookies := ParseInline("a=1; b=2")
	ApplyDomain(cookies, "https://example.com/path")

	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}
	for i, c := range cookies {
		if c.Domain != "example.com" {
			t.Errorf("cookie %d: expected domain example.com, got %s", i, c.Domain)
		}
	}
}

func TestApplyDomain_ExistingDomainPreserved(t *testing.T) {
	cookies := ParseInline("a=1")
	// Manually set a domain
	cookies[0].Domain = "other.com"
	ApplyDomain(cookies, "https://example.com/path")

	if cookies[0].Domain != "other.com" {
		t.Errorf("expected existing domain to be preserved, got %s", cookies[0].Domain)
	}
}

func TestInjectCookiesActionReturnsBatchError(t *testing.T) {
	params := []*network.CookieParam{{Name: "sid", Value: "abc", Domain: "example.com", Path: "/"}}
	err := InjectCookiesAction(params).Do(context.Background())
	if err == nil {
		t.Fatal("expected injection error")
	}
	if !strings.Contains(err.Error(), "failed to inject cookie batch") {
		t.Fatalf("error = %q, want batch failure", err.Error())
	}
}
