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
	archiveName, err := safeArchiveName(entry.Archive)
	if err != nil {
		return InstallResult{}, err
	}
	if err := os.MkdirAll(paths.CacheRoot, 0o700); err != nil {
		return InstallResult{}, err
	}
	tmp, err := os.MkdirTemp(paths.CacheRoot, "agent-aget-browser-")
	if err != nil {
		return InstallResult{}, err
	}
	defer os.RemoveAll(tmp)

	archivePath := filepath.Join(tmp, archiveName)
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
	stagedExecutable, err := stagedExecutablePath(paths, staged)
	if err != nil {
		return InstallResult{}, err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(stagedExecutable, 0o755)
	}
	if err := validateExecutable(stagedExecutable); err != nil {
		return InstallResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(paths.InstallDir), 0o700); err != nil {
		return InstallResult{}, err
	}
	backup := filepath.Join(tmp, "previous")
	installed := false
	if err := os.Rename(paths.InstallDir, backup); err != nil {
		if !os.IsNotExist(err) {
			return InstallResult{}, err
		}
	} else {
		installed = true
	}
	if err := os.Rename(staged, paths.InstallDir); err != nil {
		if installed {
			_ = os.Rename(backup, paths.InstallDir)
		}
		return InstallResult{}, err
	}
	if status := Status(paths); !status.Installed || !status.Executable {
		_ = os.RemoveAll(paths.InstallDir)
		if installed {
			_ = os.Rename(backup, paths.InstallDir)
		}
		return InstallResult{}, fmt.Errorf("browser executable validation failed: %s", paths.Executable)
	}
	if installed {
		if err := os.RemoveAll(backup); err != nil {
			return InstallResult{}, err
		}
	}
	return InstallResult{OK: true, Version: manifest.Version, Platform: platform, Path: paths.Executable, CacheDir: paths.CacheRoot}, nil
}

func safeArchiveName(name string) (string, error) {
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("browser archive name must be a safe basename")
	}
	return name, nil
}

func stagedExecutablePath(paths InstallPaths, stagedDir string) (string, error) {
	relative, err := filepath.Rel(paths.InstallDir, paths.Executable)
	if err != nil || relative == "." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return "", fmt.Errorf("browser executable path must stay within install dir")
	}
	return filepath.Join(stagedDir, relative), nil
}

func validateExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return fmt.Errorf("browser executable validation failed: %s", path)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("browser executable validation failed: %s", path)
	}
	return nil
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
