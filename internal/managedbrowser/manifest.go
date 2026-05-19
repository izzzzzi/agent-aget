package managedbrowser

import (
	"embed"
	"encoding/json"
	"fmt"
	"runtime"
)

//go:embed browser-manifest.json
var manifestFS embed.FS

type Manifest struct {
	Version   string              `json:"version"`
	Platforms map[string]Platform `json:"platforms"`
}

type Platform struct {
	Archive        string `json:"archive"`
	URL            string `json:"url"`
	SHA256         string `json:"sha256"`
	ExecutablePath string `json:"executable_path"`
}

func BundledManifest() (Manifest, error) {
	body, err := manifestFS.ReadFile("browser-manifest.json")
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.Version == "" {
		return Manifest{}, fmt.Errorf("browser manifest version is empty")
	}
	if len(manifest.Platforms) == 0 {
		return Manifest{}, fmt.Errorf("browser manifest has no platforms")
	}
	return manifest, nil
}

func CurrentPlatformKey() string {
	return PlatformKey(runtime.GOOS, runtime.GOARCH)
}

func PlatformKey(goos, goarch string) string {
	switch goos + "/" + goarch {
	case "darwin/arm64":
		return "darwin-arm64"
	case "darwin/amd64":
		return "darwin-x64"
	case "linux/amd64":
		return "linux-x64"
	case "windows/amd64":
		return "win32-x64"
	default:
		return goos + "-" + goarch
	}
}

func (m Manifest) PlatformEntry(platform string) (Platform, error) {
	entry, ok := m.Platforms[platform]
	if !ok {
		return Platform{}, fmt.Errorf("unsupported managed browser platform: %s", platform)
	}
	return entry, nil
}
