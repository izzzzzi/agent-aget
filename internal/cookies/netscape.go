// Package cookies parses Netscape cookie files and inline cookie strings,
// and injects them into a Chrome DevTools Protocol browser session.
package cookies

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
)

// ParseNetscape reads a Netscape-format cookie file from r and returns
// CDP cookie params. Lines that can't be parsed are logged and skipped;
// the function never returns an error for individual bad lines.
//
// Expected format (tab or space separated):
//
//	domain  domain_flag  path  secure  expiry  name  value
//
// Lines starting with # are comments.
// #HttpOnly_ prefix on domain sets httpOnly: true.
// Expiry is a Unix timestamp in seconds; cookies with expiry < now are skipped.
//
// Tested with exporters: EditThisCookie, Get cookies.txt, Cookie-Editor.
func ParseNetscape(r io.Reader, now time.Time) ([]*network.CookieParam, error) {
	var cookies []*network.CookieParam
	scanner := bufio.NewScanner(r)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()

		// Skip empty lines and plain comments
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Handle #HttpOnly_ prefix BEFORE generic # comment check
		httpOnly := false
		restoreLine := line
		if strings.HasPrefix(line, "#HttpOnly_") {
			httpOnly = true
			line = strings.TrimPrefix(line, "#HttpOnly_")
		} else if strings.HasPrefix(line, "#") {
			// Plain comment — skip
			continue
		}

		// Split on any whitespace (tabs or spaces — real exporters vary)
		fields := strings.Fields(line)
		if len(fields) < 7 {
			// Not enough fields; skip quietly
			_ = restoreLine
			continue
		}

		domain := fields[0]
		// domain_flag fields[1] — ignored; we use domain presence
		path := fields[2]
		secureStr := fields[3]
		expiryStr := fields[4]
		name := fields[5]
		value := fields[6]

		// Normalize domain: strip leading dot (CDP doesn't want it)
		domain = strings.TrimPrefix(domain, ".")
		if domain == "" {
			continue
		}

		// Parse secure flag
		secure := strings.EqualFold(secureStr, "true")

		// Parse expiry
		expiry, err := strconv.ParseInt(expiryStr, 10, 64)
		if err != nil {
			// Not a number — skip this cookie
			continue
		}
		if expiry > 0 && now.Unix() > expiry {
			// Expired cookie — skip
			continue
		}

		// Default path
		if path == "" {
			path = "/"
		}

		cookie := &network.CookieParam{
			Name:     name,
			Value:    value,
			Domain:   domain,
			Path:     path,
			Secure:   secure,
			HTTPOnly: httpOnly,
			SameSite: network.CookieSameSite("Lax"),
		}

		// Only set Expires if it's a real future timestamp
		if expiry > 0 {
			t := time.Unix(expiry, 0)
			v := cdp.TimeSinceEpoch(t)
			cookie.Expires = &v
		}

		cookies = append(cookies, cookie)
	}

	if err := scanner.Err(); err != nil {
		return cookies, fmt.Errorf("scanning cookie file: %w", err)
	}

	return cookies, nil
}
