package cookies

import (
	"context"
	"fmt"
	"log"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const batchSize = 50

// InjectCookiesAction returns a chromedp.Action that injects the given cookies
// via CDP Network.setCookies. Cookies are injected in batches of 50 to avoid
// CDP implementation limits. If a batch fails, a warning is logged and injection
// continues — partial injection is better than none.
//
// This action must run BEFORE any navigation action.
func InjectCookiesAction(cookies []*network.CookieParam) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		if len(cookies) == 0 {
			return nil
		}

		for i := 0; i < len(cookies); i += batchSize {
			end := i + batchSize
			if end > len(cookies) {
				end = len(cookies)
			}

			batch := cookies[i:end]
			if err := network.SetCookies(batch).Do(ctx); err != nil {
				log.Printf("WARNING: failed to inject cookie batch %d-%d: %v", i, end, err)
				// Continue with remaining batches
			}
		}

		return nil
	})
}

// domainForURL returns the hostname from a URL string, or the string itself
// if it can't be parsed. This is used to set the default domain for inline
// cookies that don't specify one.
func DomainForURL(rawURL string) string {
	// Simple extraction: find authority part after ://
	// For "https://example.com/path" → "example.com"
	if len(rawURL) < 8 {
		return rawURL
	}
	// Skip scheme
	start := 0
	for i := 0; i < len(rawURL)-2; i++ {
		if rawURL[i:i+3] == "://" {
			start = i + 3
			break
		}
	}
	if start >= len(rawURL) {
		return rawURL
	}
	// Find end of host (next / or end)
	end := start
	for end < len(rawURL) && rawURL[end] != '/' && rawURL[end] != ':' && rawURL[end] != '?' {
		end++
	}
	if end <= start {
		return rawURL
	}
	return rawURL[start:end]
}

// ApplyDomain sets the Domain field for all cookies that don't have one,
// ensuring they're valid for the target URL's hostname.
func ApplyDomain(cookies []*network.CookieParam, targetURL string) {
	domain := DomainForURL(targetURL)
	if domain == "" {
		return
	}
	for _, c := range cookies {
		if c.Domain == "" {
			c.Domain = domain
		}
	}
}

// InjectResult describes the outcome of a cookie injection.
type InjectResult struct {
	Total   int `json:"total"`
	Batches int `json:"batches"`
}

// InjectWithResult is like InjectCookiesAction but also returns a result summary.
// Useful for logging and debugging.
func InjectWithResult(cookies []*network.CookieParam) (chromedp.Action, *InjectResult) {
	result := &InjectResult{
		Total:   len(cookies),
		Batches: (len(cookies) + batchSize - 1) / batchSize,
	}
	return InjectCookiesAction(cookies), result
}

var _ fmt.Stringer = (*InjectResult)(nil)

func (r *InjectResult) String() string {
	return fmt.Sprintf("%d cookies in %d batches", r.Total, r.Batches)
}
