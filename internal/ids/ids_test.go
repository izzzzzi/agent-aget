package ids

import "testing"

func TestNewSessionID(t *testing.T) {
	id, err := NewSessionID()
	if err != nil {
		t.Fatalf("NewSessionID() error = %v", err)
	}
	if len(id) != 8 {
		t.Fatalf("len(NewSessionID()) = %d, want 8", len(id))
	}
	for _, ch := range id {
		if ch < '0' || ch > '9' && ch < 'a' || ch > 'f' {
			t.Fatalf("NewSessionID() = %q, want lowercase hex only", id)
		}
	}
}
