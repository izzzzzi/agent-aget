package browser

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

const browserNotFoundMessage = "CloakBrowser-compatible binary not found; set --browser-path or AGET_BROWSER_PATH"

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
