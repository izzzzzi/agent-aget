package managedbrowser

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const CacheEnv = "AGET_BROWSER_CACHE_DIR"

type InstallPaths struct {
	CacheRoot  string `json:"cache_dir"`
	InstallDir string `json:"install_dir"`
	Executable string `json:"path"`
}

type InstallStatus struct {
	Installed  bool `json:"installed"`
	Executable bool `json:"executable"`
}

func CacheRoot() (string, error) {
	if override := os.Getenv(CacheEnv); override != "" {
		return override, nil
	}
	root, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return root, nil
}

func Paths(version, platform string, entry Platform) (InstallPaths, error) {
	if version == "" || platform == "" || entry.ExecutablePath == "" {
		return InstallPaths{}, fmt.Errorf("browser install path requires version, platform, and executable path")
	}
	if !safePathName(version) {
		return InstallPaths{}, fmt.Errorf("browser install path version must be a relative path name")
	}
	if !safePathName(platform) {
		return InstallPaths{}, fmt.Errorf("browser install path platform must be a relative path name")
	}
	executable, err := safeExecutablePath(entry.ExecutablePath)
	if err != nil {
		return InstallPaths{}, err
	}

	root, err := CacheRoot()
	if err != nil {
		return InstallPaths{}, err
	}
	installDir := filepath.Join(root, "agent-aget", "chrome-for-testing", version, platform)
	executablePath := filepath.Join(installDir, executable)
	if !pathInside(installDir, executablePath) {
		return InstallPaths{}, fmt.Errorf("browser executable path must stay within install dir")
	}
	return InstallPaths{
		CacheRoot:  root,
		InstallDir: installDir,
		Executable: executablePath,
	}, nil
}

func safePathName(name string) bool {
	return name != "" &&
		name != "." &&
		name != ".." &&
		!strings.ContainsAny(name, `/\`)
}

func safeExecutablePath(path string) (string, error) {
	executable := filepath.FromSlash(path)
	if filepath.IsAbs(executable) {
		return "", fmt.Errorf("browser executable path must be relative")
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." || strings.Contains(segment, `\`) {
			return "", fmt.Errorf("browser executable path contains unsafe path segment")
		}
	}
	return executable, nil
}

func pathInside(parent, child string) bool {
	rel, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	if err != nil || rel == "." || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func Status(paths InstallPaths) InstallStatus {
	info, err := os.Stat(paths.Executable)
	if err != nil || info.IsDir() {
		return InstallStatus{}
	}
	executable := true
	if runtime.GOOS != "windows" {
		executable = info.Mode().Perm()&0o111 != 0
	}
	return InstallStatus{Installed: true, Executable: executable}
}
