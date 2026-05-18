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
	t.Setenv("AGET_STATE_DIR", "/tmp/aget-test")

	if got, want := SessionsDir(), "/tmp/aget-test/sessions"; got != want {
		t.Fatalf("SessionsDir() = %q, want %q", got, want)
	}
	if got, want := ArtifactsDir(), "/tmp/aget-test/artifacts"; got != want {
		t.Fatalf("ArtifactsDir() = %q, want %q", got, want)
	}
	if got, want := ProfilesDir(), "/tmp/aget-test/profiles"; got != want {
		t.Fatalf("ProfilesDir() = %q, want %q", got, want)
	}
}
