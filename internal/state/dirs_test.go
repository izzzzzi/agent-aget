package state

import (
	"path/filepath"
	"testing"
)

func TestBaseDirUsesOverride(t *testing.T) {
	override := filepath.Join("tmp", "aget-state")
	t.Setenv("AGET_STATE_DIR", override)

	if got := BaseDir(); got != override {
		t.Fatalf("BaseDir() = %q, want %q", got, override)
	}
}

func TestDerivedDirs(t *testing.T) {
	base := filepath.Join("tmp", "aget-test")
	t.Setenv("AGET_STATE_DIR", base)

	if got, want := SessionsDir(), filepath.Join(base, "sessions"); got != want {
		t.Fatalf("SessionsDir() = %q, want %q", got, want)
	}
	if got, want := ArtifactsDir(), filepath.Join(base, "artifacts"); got != want {
		t.Fatalf("ArtifactsDir() = %q, want %q", got, want)
	}
	if got, want := ProfilesDir(), filepath.Join(base, "profiles"); got != want {
		t.Fatalf("ProfilesDir() = %q, want %q", got, want)
	}
}
