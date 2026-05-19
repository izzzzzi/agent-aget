package browser

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/izzzzzi/agent-aget/internal/managedbrowser"
)

const browserNotFoundMessage = "Chromium-compatible browser not found; run `aget browser install`, set --browser-path, or set AGET_BROWSER_PATH"

var managedBrowserPath = func() (string, bool) {
	manifest, err := managedbrowser.BundledManifest()
	if err != nil {
		return "", false
	}
	entry, err := manifest.PlatformEntry(managedbrowser.CurrentPlatformKey())
	if err != nil {
		return "", false
	}
	paths, err := managedbrowser.Paths(manifest.Version, managedbrowser.CurrentPlatformKey(), entry)
	if err != nil {
		return "", false
	}
	status := managedbrowser.Status(paths)
	return paths.Executable, status.Installed && status.Executable
}

func ResolveBinary(explicit string) (string, error) {
	if explicit != "" {
		path, err := requireExecutable(explicit)
		if err != nil {
			return "", fmt.Errorf("%s: %w", browserNotFoundMessage, err)
		}
		return path, nil
	}

	if env := os.Getenv("AGET_BROWSER_PATH"); env != "" {
		path, err := requireExecutable(env)
		if err != nil {
			return "", fmt.Errorf("%s: %w", browserNotFoundMessage, err)
		}
		return path, nil
	}

	if path, ok := managedBrowserPath(); ok {
		return path, nil
	}

	for _, name := range candidateNames() {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, nil
		}
	}

	return "", errors.New(browserNotFoundMessage)
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
