package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNoPolicyAllowsAll(t *testing.T) {
	p, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if p.Check("click", "https://example.com") != Allow {
		t.Fatal("no policy should allow all")
	}
}

func TestNonexistentPolicyAllowsAll(t *testing.T) {
	p, err := Load("/nonexistent/policy.json")
	if err != nil {
		t.Fatal(err)
	}
	if p.Check("js", "https://example.com") != Allow {
		t.Fatal("missing policy file should allow all")
	}
}

func TestClearDeny(t *testing.T) {
	p := &Policy{
		Rules:  []Rule{{Action: "js", Allow: false}},
		loaded: true,
	}
	if p.Check("js", "https://example.com") != Deny {
		t.Fatal("js should be denied")
	}
	// non-js actions not in any rule → deny (fail-closed)
	if p.Check("click", "https://example.com") != Deny {
		t.Fatal("unlisted action should be deny by default")
	}
}

func TestAllowSpecific(t *testing.T) {
	p := &Policy{
		Rules: []Rule{
			{Action: "click", Allow: true},
		},
		loaded: true,
	}
	if p.Check("click", "https://example.com") != Allow {
		t.Fatal("click should be allowed")
	}
	if p.Check("fill", "https://example.com") != Deny {
		t.Fatal("fill should be denied (not listed)")
	}
}

func TestWildcardAllow(t *testing.T) {
	p := &Policy{
		Rules: []Rule{
			{Action: "js", Allow: false},
			{Action: "*", Allow: true},
		},
		loaded: true,
	}
	if p.Check("click", "https://example.com") != Allow {
		t.Fatal("click via wildcard should be allowed")
	}
	if p.Check("js", "https://example.com") != Deny {
		t.Fatal("js explicit deny (listed before wildcard) should be denied")
	}
}

func TestConfirmRequired(t *testing.T) {
	p := &Policy{
		Rules: []Rule{
			{Action: "click", Allow: true},
			{Action: "fill", RequireConfirm: true},
		},
		loaded: true,
	}
	if p.Check("click", "https://example.com") != Allow {
		t.Fatal("click should be allowed")
	}
	if p.Check("fill", "https://example.com") != ConfirmRequired {
		t.Fatal("fill should require confirm")
	}
	if p.CheckWithConfirm("fill", "https://example.com", false) != ConfirmRequired {
		t.Fatal("fill without confirm flag should still require confirm")
	}
	if p.CheckWithConfirm("fill", "https://example.com", true) != Allow {
		t.Fatal("fill with --confirm should be allowed")
	}
}

func TestURLPattern(t *testing.T) {
	p := &Policy{
		Rules: []Rule{
			{Action: "click", URLPattern: "admin", RequireConfirm: true},
			{Action: "click", Allow: true},
		},
		loaded: true,
	}
	if p.Check("click", "https://example.com") != Allow {
		t.Fatal("click on non-admin should be allowed")
	}
	if p.Check("click", "https://admin.example.com") != ConfirmRequired {
		t.Fatal("click on admin should require confirm")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(fp, []byte(`{"rules":[{"action":"js","allow":false}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(fp)
	if err != nil {
		t.Fatal(err)
	}
	if p.Check("js", "http://x.com") != Deny {
		t.Fatal("js should be denied from file policy")
	}
	if p.Check("click", "http://x.com") != Deny {
		t.Fatal("unlisted action should be deny (fail-closed)")
	}
}

func TestResultString(t *testing.T) {
	if Allow.String() != "allow" {
		t.Fatalf("Allow.String() = %q", Allow.String())
	}
}
