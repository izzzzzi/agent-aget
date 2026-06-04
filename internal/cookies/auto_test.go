package cookies

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCookies_File(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "cookies.txt")

	content := "# Netscape\n.test.com\tTRUE\t/\tFALSE\t2000000000\tsess\tval\n"
	if err := os.WriteFile(fpath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cookies, err := ParseCookies(fpath)
	if err != nil {
		t.Fatalf("ParseCookies failed: %v", err)
	}
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != "sess" {
		t.Errorf("expected name sess, got %s", cookies[0].Name)
	}
}

func TestParseCookies_Inline(t *testing.T) {
	cookies, err := ParseCookies("a=1; b=2")
	if err != nil {
		t.Fatalf("ParseCookies failed: %v", err)
	}
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}
}

func TestParseCookies_DetectOrder(t *testing.T) {
	// File that looks like "name=value" should still be parsed as file
	dir := t.TempDir()
	fpath := filepath.Join(dir, "foo=bar")
	if err := os.WriteFile(fpath, []byte("# Netscape\n.test.com\tTRUE\t/\tFALSE\t2000000000\tsess\tval\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cookies, err := ParseCookies(fpath)
	if err != nil {
		t.Fatalf("ParseCookies failed for file path containing =: %v", err)
	}
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
}

func TestParseCookies_NoDetect(t *testing.T) {
	_, err := ParseCookies("some-random-text-without-equals")
	if err == nil {
		t.Fatal("expected error for undetectable input")
	}
}

func TestParseCookies_EmptyInline(t *testing.T) {
	cookies, err := ParseCookies("=")
	if err != nil {
		t.Fatalf("unexpected error for '=' inline: %v", err)
	}
	if len(cookies) != 0 {
		t.Fatalf("expected 0 cookies for '=' only, got %d", len(cookies))
	}
}
