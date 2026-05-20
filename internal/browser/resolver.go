package browser

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/izzzzzi/agent-aget/internal/managedbrowser"
)

const browserNotFoundMessage = "Chromium-compatible browser not found; run `aget browser install`, set --browser-path, or set AGET_BROWSER_PATH"

type ResolvedBinary struct {
	Path    string
	Browser string
}

var primaryManagedBrowserPath = func() (ResolvedBinary, bool) {
	return managedBrowserPathFor(managedbrowser.BundledManifest)
}

var fallbackManagedBrowserPath = func() (ResolvedBinary, bool) {
	return managedBrowserPathFor(managedbrowser.ChromeManifest)
}

var managedBrowserPath = func() (ResolvedBinary, bool) {
	return primaryManagedBrowserPath()
}

func managedBrowserPathFor(loadManifest func() (managedbrowser.Manifest, error)) (ResolvedBinary, bool) {
	manifest, err := loadManifest()
	if err != nil {
		return ResolvedBinary{}, false
	}
	entry, err := manifest.PlatformEntry(managedbrowser.CurrentPlatformKey())
	if err != nil {
		return ResolvedBinary{}, false
	}
	paths, err := managedbrowser.PathsForManifest(manifest, managedbrowser.CurrentPlatformKey(), entry)
	if err != nil {
		return ResolvedBinary{}, false
	}
	status := managedbrowser.Status(paths)
	return ResolvedBinary{Path: paths.Executable, Browser: manifest.BrowserName()}, status.Installed && status.Executable
}

func ResolveBinary(explicit string) (string, error) {
	resolved, err := Resolve(explicit)
	if err != nil {
		return "", err
	}
	return resolved.Path, nil
}

func Resolve(explicit string) (ResolvedBinary, error) {
	if explicit != "" {
		path, err := requireExecutable(explicit)
		if err != nil {
			return ResolvedBinary{}, fmt.Errorf("%s: %w", browserNotFoundMessage, err)
		}
		return ResolvedBinary{Path: path, Browser: browserNameFromPath(path)}, nil
	}

	if env := os.Getenv("AGET_BROWSER_PATH"); env != "" {
		path, err := requireExecutable(env)
		if err != nil {
			return ResolvedBinary{}, fmt.Errorf("%s: %w", browserNotFoundMessage, err)
		}
		return ResolvedBinary{Path: path, Browser: browserNameFromPath(path)}, nil
	}

	if resolved, ok := managedBrowserPath(); ok {
		return resolved, nil
	}
	if resolved, ok := fallbackManagedBrowserPath(); ok {
		return resolved, nil
	}

	for _, name := range candidateNames() {
		path, err := exec.LookPath(name)
		if err == nil {
			return ResolvedBinary{Path: path, Browser: browserNameFromPath(path)}, nil
		}
	}

	return ResolvedBinary{}, errors.New(browserNotFoundMessage)
}

func requireExecutable(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", path)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		return "", fmt.Errorf("%s is not executable", path)
	}
	return path, nil
}

func candidateNames() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"cloakbrowser.exe", "chrome.exe", "chromium.exe"}
	case "darwin":
		return []string{"cloakbrowser", "chromium", "google-chrome"}
	default:
		return []string{"cloakbrowser", "chromium-browser", "chromium", "google-chrome"}
	}
}

func browserNameFromPath(path string) string {
	cleaned := filepath.ToSlash(path)
	base := strings.ToLower(filepath.Base(path))
	if strings.Contains(cleaned, "/agent-aget/cloakbrowser/") || base == "cloakbrowser" || base == "cloakbrowser.exe" {
		return "cloakbrowser"
	}
	return "chromium"
}
