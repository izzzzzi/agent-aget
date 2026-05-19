package cli

import (
	"context"
	"strings"

	"github.com/izzzzzi/agent-aget/internal/managedbrowser"
	"github.com/spf13/cobra"
)

func newBrowserCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "browser",
		Short: "Manage the bundled browser",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeInvalidArgs(cmd, "browser subcommand required")
		},
	}
	disableHelpFlag(cmd)
	cmd.AddCommand(newBrowserStatusCommand(), newBrowserPathCommand(), newBrowserInstallCommand())
	return cmd
}

func newBrowserStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show managed browser status",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, platform, _, paths, err := currentManagedBrowser()
			if err != nil {
				return writeError(cmd, browserErrorCode("browser_status_failed", err), err.Error(), nil)
			}
			status := managedbrowser.Status(paths)
			return writeJSON(cmd, map[string]any{
				"ok":         true,
				"version":    manifest.Version,
				"platform":   platform,
				"cache_dir":  paths.CacheRoot,
				"path":       paths.Executable,
				"installed":  status.Installed,
				"executable": status.Executable,
			})
		},
	}
	disableHelpFlag(cmd)
	return cmd
}

func newBrowserPathCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Print managed browser path",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, _, paths, err := currentManagedBrowser()
			if err != nil {
				return writeError(cmd, browserErrorCode("browser_path_failed", err), err.Error(), nil)
			}
			status := managedbrowser.Status(paths)
			if !status.Installed || !status.Executable {
				return writeError(cmd, "browser_not_installed", "managed browser is not installed", map[string]any{
					"recovery":  "aget browser install",
					"cache_dir": paths.CacheRoot,
				})
			}
			return writeJSON(cmd, map[string]any{"ok": true, "path": paths.Executable})
		},
	}
	disableHelpFlag(cmd)
	return cmd
}

func newBrowserInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install managed browser",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, platform, _, _, err := currentManagedBrowser()
			if err != nil {
				return writeError(cmd, browserErrorCode("browser_install_failed", err), err.Error(), nil)
			}
			result, err := managedbrowser.Install(context.Background(), manifest, platform)
			if err != nil {
				return writeError(cmd, browserErrorCode("browser_install_failed", err), err.Error(), nil)
			}
			return writeJSON(cmd, result)
		},
	}
	disableHelpFlag(cmd)
	return cmd
}

func currentManagedBrowser() (managedbrowser.Manifest, string, managedbrowser.Platform, managedbrowser.InstallPaths, error) {
	manifest, err := managedbrowser.BundledManifest()
	if err != nil {
		return managedbrowser.Manifest{}, "", managedbrowser.Platform{}, managedbrowser.InstallPaths{}, err
	}
	platform := managedbrowser.CurrentPlatformKey()
	entry, err := manifest.PlatformEntry(platform)
	if err != nil {
		return managedbrowser.Manifest{}, platform, managedbrowser.Platform{}, managedbrowser.InstallPaths{}, err
	}
	paths, err := managedbrowser.Paths(manifest.Version, platform, entry)
	if err != nil {
		return managedbrowser.Manifest{}, platform, entry, managedbrowser.InstallPaths{}, err
	}
	return manifest, platform, entry, paths, nil
}

func isUnsupportedPlatformError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unsupported managed browser platform")
}

func browserErrorCode(fallback string, err error) string {
	if isUnsupportedPlatformError(err) {
		return "browser_unsupported_platform"
	}
	return fallback
}
