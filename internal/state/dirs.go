package state

import (
	"os"
	"path/filepath"
	"runtime"
)

func BaseDir() string {
	if override := os.Getenv("AGET_STATE_DIR"); override != "" {
		return override
	}

	switch runtime.GOOS {
	case "windows":
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "aget")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, "Library", "Application Support", "aget")
		}
	default:
		if xdgStateHome := os.Getenv("XDG_STATE_HOME"); xdgStateHome != "" {
			return filepath.Join(xdgStateHome, "aget")
		}
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, ".local", "state", "aget")
		}
	}

	return "aget"
}

func SessionsDir() string {
	return filepath.Join(BaseDir(), "sessions")
}

func ArtifactsDir() string {
	return filepath.Join(BaseDir(), "artifacts")
}

func SnapshotsDir() string {
	return filepath.Join(BaseDir(), "snapshots")
}

func ProfilesDir() string {
	return filepath.Join(BaseDir(), "profiles")
}

func ProfileMetaPath() string {
	return filepath.Join(ProfilesDir(), "profiles.json")
}

func ProfileUserDataDir(name string) string {
	return filepath.Join(ProfilesDir(), name)
}
