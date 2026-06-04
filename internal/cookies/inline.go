package cookies

import (
	"strings"

	"github.com/chromedp/cdproto/network"
)

// ParseInline parses a cookie string in the format "name=value[; name2=value2]".
// Domain, path, secure, httpOnly are left empty — the caller must set them
// (typically from the target URL) before injection.
func ParseInline(s string) []*network.CookieParam {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	var cookies []*network.CookieParam
	pairs := strings.Split(s, ";")

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		eq := strings.IndexByte(pair, '=')
		if eq < 1 {
			// No '=' or empty name — skip
			continue
		}

		name := strings.TrimSpace(pair[:eq])
		value := strings.TrimSpace(pair[eq+1:])

		if name == "" {
			continue
		}

		cookies = append(cookies, &network.CookieParam{
			Name:     name,
			Value:    value,
			SameSite: network.CookieSameSite("Lax"),
		})
	}

	return cookies
}
