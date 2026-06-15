package snapshot

// DiffResult describes how a snapshot changed relative to a prior one.
type DiffResult struct {
	Added   []Element `json:"added"`
	Removed []Element `json:"removed"`
	Changed []Change  `json:"changed"`
}

// Change records an element present in both snapshots whose observable state
// differs between them.
type Change struct {
	Before Element `json:"before"`
	After  Element `json:"after"`
}

// elementKey identifies an element across snapshots independently of its ref,
// which is positional and unstable between snapshots. Role/name/text/selector
// together are stable enough for diffing without false matches.
func elementKey(e Element) string {
	return e.Kind + "\x00" + e.Name + "\x00" + e.Text + "\x00" + e.Selector
}

// stateEqual reports whether two matched elements have the same observable
// interaction state. Ref is excluded because it is positional, not identity.
func stateEqual(a, b Element) bool {
	return a.Visible == b.Visible &&
		a.Enabled == b.Enabled &&
		a.Href == b.Href &&
		a.Type == b.Type
}

// Diff computes the delta from prev to curr. Elements only in curr are Added,
// elements only in prev are Removed, and elements in both whose state differs
// are Changed. Returned slices are always non-nil for stable JSON output.
func Diff(prev, curr []Element) DiffResult {
	result := DiffResult{
		Added:   []Element{},
		Removed: []Element{},
		Changed: []Change{},
	}

	prevByKey := make(map[string]Element, len(prev))
	for _, e := range prev {
		prevByKey[elementKey(e)] = e
	}
	currKeys := make(map[string]struct{}, len(curr))

	for _, c := range curr {
		key := elementKey(c)
		currKeys[key] = struct{}{}
		p, ok := prevByKey[key]
		if !ok {
			result.Added = append(result.Added, c)
			continue
		}
		if !stateEqual(p, c) {
			result.Changed = append(result.Changed, Change{Before: p, After: c})
		}
	}

	for _, p := range prev {
		if _, ok := currKeys[elementKey(p)]; !ok {
			result.Removed = append(result.Removed, p)
		}
	}

	return result
}
