package cdp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/chromedp/chromedp"
)

// FindCriteria selects an element by semantic attributes rather than a raw CSS
// selector. Fields are AND-combined; empty fields are ignored. Nth is 1-based
// and selects among the matches that satisfy the other criteria (0 means "no
// nth filter, require a unique match").
type FindCriteria struct {
	Role        string
	Name        string
	Text        string
	Placeholder string
	TestID      string
	Nth         int
}

// IsEmpty reports whether no selection criteria were provided.
func (c FindCriteria) IsEmpty() bool {
	return c.Role == "" && c.Name == "" && c.Text == "" &&
		c.Placeholder == "" && c.TestID == "" && c.Nth == 0
}

var (
	// ErrNoMatch is returned when no element satisfies the criteria.
	ErrNoMatch = errors.New("no element matched the locator")
	// ErrAmbiguousMatch is returned when multiple elements match and no Nth
	// was given to disambiguate.
	ErrAmbiguousMatch = errors.New("locator matched multiple elements")
)

type findResult struct {
	Selector string `json:"selector"`
	Matches  int    `json:"matches"`
}

// Find resolves a semantic locator to a unique CSS selector by reading the
// page's accessibility-relevant attributes. It performs no mutation, only
// read-only DOM queries, so it does not affect the browser's stealth profile.
func (d *ChromeDPDriver) Find(ctx context.Context, criteria FindCriteria) (string, error) {
	if criteria.IsEmpty() {
		return "", errors.New("find criteria required")
	}
	spec, err := json.Marshal(map[string]any{
		"role":        criteria.Role,
		"name":        criteria.Name,
		"text":        criteria.Text,
		"placeholder": criteria.Placeholder,
		"testid":      criteria.TestID,
		"nth":         criteria.Nth,
	})
	if err != nil {
		return "", err
	}

	var raw string
	if err := d.runActionsWithTransientRetry(ctx,
		chromedp.Evaluate(findScript(string(spec)), &raw),
	); err != nil {
		return "", err
	}

	var result findResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return "", err
	}
	switch {
	case result.Matches == 0:
		return "", ErrNoMatch
	case result.Selector == "":
		// Matched but the chosen element has no resolvable unique selector.
		return "", fmt.Errorf("%w: %d candidates but none uniquely addressable", ErrAmbiguousMatch, result.Matches)
	case result.Matches > 1 && criteria.Nth == 0:
		return "", fmt.Errorf("%w: %d candidates (use --nth to pick one)", ErrAmbiguousMatch, result.Matches)
	}
	return result.Selector, nil
}

// findScript builds the page-side resolver. It walks candidate elements,
// filters by the supplied criteria, and returns the unique CSS selector of the
// chosen match along with the total match count.
func findScript(specJSON string) string {
	return `(() => {
	  const spec = ` + specJSON + `;
	  const cssEscape = (value) => (window.CSS && CSS.escape) ? CSS.escape(value) : String(value).replace(/["\\]/g, '\\$&');
	  const cssStringEscape = (value) => String(value).replace(/\\/g, '\\\\').replace(/"/g, '\\"');
	  const uniqueElement = (selector, el) => {
	    try {
	      const matches = document.querySelectorAll(selector);
	      return matches.length === 1 && matches[0] === el;
	    } catch { return false; }
	  };
	  const pointsToElement = (selector, el) => {
	    try { return document.querySelector(selector) === el; } catch { return false; }
	  };
	  const selectorFor = (el) => {
	    if (el.id) {
	      const s = '#' + cssEscape(el.id);
	      if (uniqueElement(s, el)) return s;
	    }
	    const testid = el.getAttribute('data-testid');
	    if (testid) {
	      const s = '[data-testid="' + cssStringEscape(testid) + '"]';
	      if (uniqueElement(s, el)) return s;
	    }
	    const nm = el.getAttribute('name');
	    if (nm) {
	      const s = el.tagName.toLowerCase() + '[name="' + cssStringEscape(nm) + '"]';
	      if (uniqueElement(s, el)) return s;
	    }
	    const parts = [];
	    let current = el;
	    while (current && current.nodeType === Node.ELEMENT_NODE) {
	      let part = current.tagName.toLowerCase();
	      const parent = current.parentElement;
	      if (parent) {
	        const same = Array.from(parent.children).filter((c) => c.tagName === current.tagName);
	        if (same.length > 1) part += ':nth-of-type(' + (same.indexOf(current) + 1) + ')';
	      }
	      parts.unshift(part);
	      current = parent;
	    }
	    const s = parts.join(' > ');
	    return pointsToElement(s, el) ? s : '';
	  };
	  const roleFor = (el) => {
	    const explicit = (el.getAttribute('role') || '').toLowerCase();
	    if (explicit) return explicit;
	    const tag = (el.tagName || '').toLowerCase();
	    if (tag === 'button') return 'button';
	    if (tag === 'a' && el.hasAttribute('href')) return 'link';
	    if (tag === 'input') {
	      const t = (el.getAttribute('type') || 'text').toLowerCase();
	      if (t === 'checkbox') return 'checkbox';
	      if (t === 'radio') return 'radio';
	      if (t === 'submit' || t === 'button') return 'button';
	      return 'textbox';
	    }
	    if (tag === 'textarea') return 'textbox';
	    if (tag === 'select') return 'combobox';
	    return tag;
	  };
	  const accName = (el) => {
	    const aria = el.getAttribute('aria-label');
	    if (aria) return aria.trim();
	    const labelledby = el.getAttribute('aria-labelledby');
	    if (labelledby) {
	      const ref = document.getElementById(labelledby);
	      if (ref) return (ref.innerText || ref.textContent || '').trim();
	    }
	    if (el.id) {
	      const lbl = document.querySelector('label[for="' + cssStringEscape(el.id) + '"]');
	      if (lbl) return (lbl.innerText || lbl.textContent || '').trim();
	    }
	    const wrapLabel = el.closest && el.closest('label');
	    if (wrapLabel) return (wrapLabel.innerText || wrapLabel.textContent || '').trim();
	    return (el.innerText || el.textContent || '').trim();
	  };
	  const norm = (s) => (s || '').trim().toLowerCase().replace(/\s+/g, ' ');
	  const matchesText = (haystack, needle) => norm(haystack).includes(norm(needle));

	  const all = Array.from(document.querySelectorAll('*'));
	  const matched = all.filter((el) => {
	    if (spec.role && roleFor(el) !== norm(spec.role)) return false;
	    if (spec.testid && el.getAttribute('data-testid') !== spec.testid) return false;
	    if (spec.placeholder && !matchesText(el.getAttribute('placeholder') || '', spec.placeholder)) return false;
	    if (spec.name && !matchesText(accName(el), spec.name)) return false;
	    if (spec.text && !matchesText(el.innerText || el.textContent || '', spec.text)) return false;
	    return true;
	  });

	  // When matching by text/name without a role, prefer leaf-most elements so
	  // we don't select a wrapping container that also "contains" the text.
	  let candidates = matched;
	  if ((spec.text || spec.name) && !spec.role && !spec.testid) {
	    candidates = matched.filter((el) => !matched.some((other) => other !== el && el.contains(other)));
	  }

	  const total = candidates.length;
	  let chosen = null;
	  if (spec.nth && spec.nth >= 1 && spec.nth <= total) {
	    chosen = candidates[spec.nth - 1];
	  } else if (total === 1) {
	    chosen = candidates[0];
	  }
	  return JSON.stringify({ matches: total, selector: chosen ? selectorFor(chosen) : '' });
	})()`
}
