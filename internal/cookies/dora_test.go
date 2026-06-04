package cookies

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseNetscape_DoraHacks(t *testing.T) {
	// Real-world format: tab-separated, mixed secure, URL-encoded values, colon in name
	data := `# Netscape HTTP Cookie File
example.com	FALSE	/	TRUE	1781130055	session	abc-def-ghi
.example.com	TRUE	/	FALSE	1781821259	token	6a20a8ca5f892b156149fad0030c4641
.example.com	TRUE	/	FALSE	1783117260	user_info	%7B%22id%22%3A123%2C%22name%22%3A%22test%22%7D
.example.com	TRUE	/	FALSE	1815085261	_ga	GA1.1.1234567890.1234567890
example.com	FALSE	/	FALSE	1812061261	cookie:accepted	true
.example.com	TRUE	/	FALSE	1815085381	_ga_ABC123	GS1.1.1234567890
`

	cookies, err := ParseNetscape(strings.NewReader(data), time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ParseNetscape failed: %v", err)
	}

	if len(cookies) != 6 {
		t.Fatalf("expected 6 cookies, got %d", len(cookies))
	}

	// First cookie: host-only domain (no leading dot), secure
	if cookies[0].Domain != "example.com" {
		t.Errorf("cookie 0 domain = %q, want example.com", cookies[0].Domain)
	}
	if cookies[0].Name != "session" {
		t.Errorf("cookie 0 name = %q, want session", cookies[0].Name)
	}
	if !cookies[0].Secure {
		t.Errorf("cookie 0: expected secure=true")
	}

	// Second cookie: suffixed domain (leading dot stripped)
	if cookies[1].Domain != "example.com" {
		t.Errorf("cookie 1 domain = %q (should have leading dot stripped)", cookies[1].Domain)
	}
	if cookies[1].Name != "token" {
		t.Errorf("cookie 1 name = %q, want token", cookies[1].Name)
	}
	if cookies[1].Secure {
		t.Errorf("cookie 1: expected secure=false (FALSE in file)")
	}

	// Third cookie: URL-encoded JSON value
	if cookies[2].Name != "user_info" {
		t.Errorf("cookie 2 name = %q, want user_info", cookies[2].Name)
	}
	if len(cookies[2].Value) < 10 {
		t.Errorf("cookie 2 value too short (%d chars)", len(cookies[2].Value))
	}

	// Fifth cookie: colon in name
	if cookies[4].Name != "cookie:accepted" {
		t.Errorf("cookie 4 name = %q, want cookie:accepted", cookies[4].Name)
	}
	if cookies[4].Value != "true" {
		t.Errorf("cookie 4 value = %q, want true", cookies[4].Value)
	}

	// Fourth cookie: _ga format
	if cookies[3].Name != "_ga" {
		t.Errorf("cookie 3 name = %q, want _ga", cookies[3].Name)
	}

	// All should have SameSite = Lax
	for i, c := range cookies {
		if string(c.SameSite) != "Lax" {
			t.Errorf("cookie %d: expected SameSite=Lax, got %s", i, c.SameSite)
		}
	}
}

func TestParseNetscape_DoraHacks_AutoDetect(t *testing.T) {
	dir := t.TempDir()
	fpath := dir + "/cookies.txt"
	data := []byte("example.com\tFALSE\t/\tTRUE\t1781130055\tsession\tabc\n.example.com\tTRUE\t/\tFALSE\t1781821259\ttoken\tdef\n")
	if err := os.WriteFile(fpath, data, 0644); err != nil {
		t.Fatal(err)
	}

	cookies, err := ParseCookies(fpath)
	if err != nil {
		t.Fatalf("ParseCookies failed: %v", err)
	}
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}
}
