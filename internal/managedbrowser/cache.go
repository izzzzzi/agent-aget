package managedbrowser

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	root, err := CacheRoot()
	if err != nil {
		return InstallPaths{}, err
	}
	if version == "" || platform == "" || entry.ExecutablePath == "" {
		return InstallPaths{}, fmt.Errorf("browser install path requires version, platform, and executable path")
	}
	installDir := filepath.Join(root, "agent-aget", "chrome-for-testing", version, platform)
	return InstallPaths{
		CacheRoot:  root,
		InstallDir: installDir,
		Executable: filepath.Join(installDir, filepath.FromSlash(entry.ExecutablePath)),
	}, nil
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
