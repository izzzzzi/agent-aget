package cookies

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
)

// ParseCookies auto-detects the cookie input format and parses accordingly.
// If input is a path to an existing file, it's parsed as Netscape format.
// Otherwise, if it contains "=", it's parsed as inline name=value pairs.
// If neither, an error is returned.
func ParseCookies(input string) ([]*network.CookieParam, error) {
	// Check file first
	if info, err := os.Stat(input); err == nil && !info.IsDir() {
		f, err := os.Open(input)
		if err != nil {
			return nil, fmt.Errorf("opening cookie file: %w", err)
		}
		defer f.Close()

		cookies, err := ParseNetscape(f, time.Now())
		if err != nil {
			return nil, fmt.Errorf("parsing cookie file: %w", err)
		}
		return cookies, nil
	}

	// Inline check
	if strings.Contains(input, "=") {
		return ParseInline(input), nil
	}

	return nil, fmt.Errorf("could not detect cookie format: %q is not a file and contains no '='", input)
}
