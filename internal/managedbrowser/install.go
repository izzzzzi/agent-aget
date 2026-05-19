package managedbrowser

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type InstallResult struct {
	OK               bool   `json:"ok"`
	Version          string `json:"version"`
	Platform         string `json:"platform"`
	Path             string `json:"path"`
	CacheDir         string `json:"cache_dir"`
	AlreadyInstalled bool   `json:"already_installed"`
}

func Install(ctx context.Context, manifest Manifest, platform string) (InstallResult, error) {
	entry, err := manifest.PlatformEntry(platform)
	if err != nil {
		return InstallResult{}, err
	}
	paths, err := Paths(manifest.Version, platform, entry)
	if err != nil {
		return InstallResult{}, err
	}
	if status := Status(paths); status.Installed && status.Executable {
		return InstallResult{OK: true, Version: manifest.Version, Platform: platform, Path: paths.Executable, CacheDir: paths.CacheRoot, AlreadyInstalled: true}, nil
	}
	tmp, err := os.MkdirTemp(paths.CacheRoot, "agent-aget-browser-")
	if err != nil {
		return InstallResult{}, err
	}
	defer os.RemoveAll(tmp)

	archivePath := filepath.Join(tmp, entry.Archive)
	if err := download(ctx, entry.URL, archivePath); err != nil {
		return InstallResult{}, fmt.Errorf("download browser archive: %w", err)
	}
	if err := verifySHA256(archivePath, entry.SHA256); err != nil {
		return InstallResult{}, err
	}
	extractDir := filepath.Join(tmp, "extract")
	if err := extractZip(archivePath, extractDir); err != nil {
		return InstallResult{}, fmt.Errorf("extract browser archive: %w", err)
	}
	staged := filepath.Join(tmp, "staged")
	if err := os.Rename(extractDir, staged); err != nil {
		return InstallResult{}, err
	}
	if err := os.RemoveAll(paths.InstallDir); err != nil {
		return InstallResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(paths.InstallDir), 0o700); err != nil {
		return InstallResult{}, err
	}
	if err := os.Rename(staged, paths.InstallDir); err != nil {
		return InstallResult{}, err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(paths.Executable, 0o755)
	}
	if status := Status(paths); !status.Installed || !status.Executable {
		return InstallResult{}, fmt.Errorf("browser executable validation failed: %s", paths.Executable)
	}
	return InstallResult{OK: true, Version: manifest.Version, Platform: platform, Path: paths.Executable, CacheDir: paths.CacheRoot}, nil
}

func download(ctx context.Context, url, destination string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status: %s", response.Status)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, response.Body)
	return err
}

func verifySHA256(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != strings.ToLower(expected) {
		return fmt.Errorf("sha256 mismatch for %s: got %s, want %s", path, actual, strings.ToLower(expected))
	}
	return nil
}

func extractZip(archivePath, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		target, err := safeZipPath(destination, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, file.Mode().Perm()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		if err := writeZipFile(target, source, file.Mode().Perm()); err != nil {
			source.Close()
			return err
		}
		if err := source.Close(); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(path string, source io.Reader, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, source)
	return err
}

func safeZipPath(destination, name string) (string, error) {
	cleanDestination := filepath.Clean(destination)
	target := filepath.Clean(filepath.Join(cleanDestination, filepath.FromSlash(name)))
	prefix := cleanDestination + string(filepath.Separator)
	if target != cleanDestination && !strings.HasPrefix(target, prefix) {
		return "", fmt.Errorf("zip entry escapes destination: %s", name)
	}
	return target, nil
}
