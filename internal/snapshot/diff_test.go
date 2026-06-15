package snapshot

import "testing"

func TestDiffAddedRemovedChanged(t *testing.T) {
	prev := []Element{
		{Ref: "@e1", Kind: "button", Name: "Submit", Selector: "#go", Visible: true, Enabled: false},
		{Ref: "@e2", Kind: "link", Text: "Home", Selector: "a#home", Visible: true},
	}
	curr := []Element{
		// same identity as prev[0] but now enabled -> Changed
		{Ref: "@e1", Kind: "button", Name: "Submit", Selector: "#go", Visible: true, Enabled: true},
		// brand new -> Added
		{Ref: "@e2", Kind: "link", Text: "About", Selector: "a#about", Visible: true},
		// prev[1] (Home) is gone -> Removed
	}

	d := Diff(prev, curr)

	if len(d.Added) != 1 || d.Added[0].Text != "About" {
		t.Fatalf("added = %#v", d.Added)
	}
	if len(d.Removed) != 1 || d.Removed[0].Text != "Home" {
		t.Fatalf("removed = %#v", d.Removed)
	}
	if len(d.Changed) != 1 {
		t.Fatalf("changed = %#v", d.Changed)
	}
	if d.Changed[0].Before.Enabled || !d.Changed[0].After.Enabled {
		t.Fatalf("changed state wrong: %#v", d.Changed[0])
	}
}

func TestDiffIdenticalSnapshots(t *testing.T) {
	els := []Element{
		{Kind: "button", Name: "Go", Selector: "#go", Visible: true, Enabled: true},
	}
	d := Diff(els, els)
	if len(d.Added) != 0 || len(d.Removed) != 0 || len(d.Changed) != 0 {
		t.Fatalf("identical snapshots should produce no diff: %#v", d)
	}
}

func TestDiffEmptyInputs(t *testing.T) {
	d := Diff(nil, nil)
	if d.Added == nil || d.Removed == nil || d.Changed == nil {
		t.Fatal("diff slices must be non-nil for stable JSON")
	}
	if len(d.Added) != 0 || len(d.Removed) != 0 || len(d.Changed) != 0 {
		t.Fatalf("empty diff should be empty: %#v", d)
	}
}

func TestDiffAllAddedFromEmpty(t *testing.T) {
	curr := []Element{{Kind: "button", Name: "Go", Selector: "#go"}}
	d := Diff(nil, curr)
	if len(d.Added) != 1 || len(d.Removed) != 0 {
		t.Fatalf("diff from empty = %#v", d)
	}
}
