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

func TestValidSessionID(t *testing.T) {
	valid := []string{"abc12345", "00000000", "ffffffff"}
	for _, id := range valid {
		if !ValidSessionID(id) {
			t.Fatalf("ValidSessionID(%q) = false, want true", id)
		}
	}

	invalid := []string{"", "a", "abc1234", "abc123456", "ABC12345", "../bad", "abc;rm", "abc def", "abc$(x)"}
	for _, id := range invalid {
		if ValidSessionID(id) {
			t.Fatalf("ValidSessionID(%q) = true, want false", id)
		}
	}
}
