// Package clean trims boilerplate noise from captured page text.
//
// It operates purely on the already-captured []string lines produced by the
// page reader (plain innerText, never HTML), so it performs zero interaction
// with the live page and preserves the browser's stealth profile. The
// heuristic is intentionally conservative: it only removes exact-duplicate
// lines and lines that strongly match a small set of known boilerplate
// patterns (cookie banners, skip-links, scroll-to-top affordances). Primary
// content is never dropped.
package clean

import (
	"regexp"
	"strings"
)

// boilerplatePatterns matches lines that are page chrome rather than content.
// Each pattern is anchored to the whole line (after trimming) so that a line
// merely containing one of these words as part of a sentence is preserved.
var boilerplatePatterns = []*regexp.Regexp{
	// Cookie / consent banners.
	regexp.MustCompile(`(?i)^we use cookies\b.*$`),
	regexp.MustCompile(`(?i)^(this (web)?site|our (web)?site) uses cookies\b.*$`),
	regexp.MustCompile(`(?i)^accept( all)?( cookies)?$`),
	regexp.MustCompile(`(?i)^reject( all)?( cookies)?$`),
	regexp.MustCompile(`(?i)^manage( your)? cookies$`),
	regexp.MustCompile(`(?i)^cookie (policy|preferences|settings)$`),
	regexp.MustCompile(`(?i)^(allow|deny) all$`),
	// Navigation affordances.
	regexp.MustCompile(`(?i)^skip to (main )?content$`),
	regexp.MustCompile(`(?i)^back to top$`),
	regexp.MustCompile(`(?i)^scroll to top$`),
	regexp.MustCompile(`(?i)^jump to navigation$`),
}

// isBoilerplate reports whether a trimmed line matches a known boilerplate
// pattern.
func isBoilerplate(line string) bool {
	for _, pattern := range boilerplatePatterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

// Extract removes boilerplate noise from page text lines.
//
// It drops exact-duplicate lines (keeping the first occurrence) and lines that
// match a known boilerplate pattern. It returns the kept lines and the count
// of dropped lines. The function is idempotent: Extract(Extract(x)) equals
// Extract(x). A nil or empty input yields an empty (non-nil) slice and a zero
// dropped count.
func Extract(lines []string) (kept []string, dropped int) {
	kept = make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isBoilerplate(trimmed) {
			dropped++
			continue
		}
		if _, ok := seen[trimmed]; ok {
			dropped++
			continue
		}
		seen[trimmed] = struct{}{}
		kept = append(kept, line)
	}
	return kept, dropped
}
