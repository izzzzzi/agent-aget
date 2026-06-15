// Package policy implements an allow/deny/confirm action policy for aget. It
// is loaded from a JSON file path supplied via AGET_POLICY env or CLI flag.
// When no policy file exists all actions are permitted (backward compatible).
// With a policy loaded, every action is checked against the rules before
// execution: first match wins, default is deny.
package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrDenied is returned when the policy explicitly denies an action.
var ErrDenied = errors.New("action denied by policy")

// Rule defines a single policy rule.
type Rule struct {
	// Action is the action name to match: "click", "fill", "type", "js",
	// "navigate", "eval", "screenshot", "snapshot", "read", or "*" for all.
	Action string `json:"action"`
	// URLPattern is an optional substring match against the page URL.
	// The rule only applies when the URL contains this substring.
	URLPattern string `json:"url_pattern,omitempty"`
	// Allow explicitly permits the action.
	Allow bool `json:"allow,omitempty"`
	// RequireConfirm makes the action fail with a confirmable error instead
	// of executing it silently. The agent can retry with --confirm.
	RequireConfirm bool `json:"require_confirm,omitempty"`
}

// Policy is a loaded set of rules.
type Policy struct {
	Rules  []Rule `json:"rules"`
	loaded bool
}

// Result describes the outcome of a policy check.
type Result int

const (
	// Allow — the action may proceed.
	Allow Result = iota
	// Deny — the action is forbidden.
	Deny
	// ConfirmRequired — the action requires explicit agent confirmation.
	ConfirmRequired
)

func (r Result) String() string {
	switch r {
	case Allow:
		return "allow"
	case Deny:
		return "deny"
	case ConfirmRequired:
		return "confirm_required"
	default:
		return "unknown"
	}
}

// Load reads a policy from the given JSON file path. If the path is empty or
// the file does not exist, it returns a nil (allow-all) policy without error.
func Load(path string) (*Policy, error) {
	if path == "" {
		return &Policy{}, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Policy{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("policy load: %w", err)
	}
	var p Policy
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("policy parse: %w", err)
	}
	p.loaded = true
	return &p, nil
}

// IsLoaded reports whether a policy file was actually loaded (as opposed to
// the nil allow-all default).
func (p *Policy) IsLoaded() bool {
	return p != nil && p.loaded
}

// Check evaluates the policy for the given action and page URL against every
// rule in order. The first matching rule determines the result. If no rule
// matches the default is Deny (fail-closed) when a policy is loaded, or Allow
// when no policy was loaded.
func (p *Policy) Check(action, pageURL string) Result {
	if p == nil || !p.loaded {
		return Allow
	}
	action = strings.ToLower(strings.TrimSpace(action))
	for _, rule := range p.Rules {
		if rule.Action != "*" && strings.ToLower(strings.TrimSpace(rule.Action)) != action {
			continue
		}
		if rule.URLPattern != "" && !strings.Contains(pageURL, rule.URLPattern) {
			continue
		}
		if rule.RequireConfirm {
			return ConfirmRequired
		}
		if rule.Allow {
			return Allow
		}
		return Deny
	}
	// fail-closed: no matching rule means deny
	return Deny
}

// CheckWithConfirm is like Check but includes user-provided confirmation.
// If the user passed --confirm, ConfirmRequired is promoted to Allow.
func (p *Policy) CheckWithConfirm(action, pageURL string, confirmed bool) Result {
	r := p.Check(action, pageURL)
	if r == ConfirmRequired && confirmed {
		return Allow
	}
	return r
}
