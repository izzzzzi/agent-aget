package managedbrowser

import "testing"

func TestBundledManifestHasPinnedVersionAndPlatforms(t *testing.T) {
	manifest, err := BundledManifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version == "" {
		t.Fatal("Version is empty")
	}
	for _, platform := range []string{"darwin-arm64", "darwin-x64", "linux-x64", "win32-x64"} {
		entry, ok := manifest.Platforms[platform]
		if !ok {
			t.Fatalf("missing platform %s", platform)
		}
		if entry.URL == "" || entry.Archive == "" || entry.SHA256 == "" || entry.ExecutablePath == "" {
			t.Fatalf("incomplete manifest entry for %s: %#v", platform, entry)
		}
	}
}

func TestCurrentPlatformEntryRejectsUnknownPlatform(t *testing.T) {
	manifest := Manifest{Version: "1", Platforms: map[string]Platform{}}
	_, err := manifest.PlatformEntry("linux-arm64")
	if err == nil {
		t.Fatal("expected unsupported platform error")
	}
}
