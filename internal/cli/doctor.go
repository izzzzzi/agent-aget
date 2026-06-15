package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/izzzzzi/agent-aget/internal/browser"
	"github.com/izzzzzi/agent-aget/internal/doctor"
	"github.com/izzzzzi/agent-aget/internal/state"
	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check aget installation and runtime readiness",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result := doctor.Runner{Checks: []doctor.Check{
				{Name: "state_dir", Run: checkWritableDir(state.BaseDir())},
				{Name: "sessions_dir", Run: checkWritableDir(state.SessionsDir())},
				{Name: "artifacts_dir", Run: checkWritableDir(state.ArtifactsDir())},
				{Name: "snapshots_dir", Run: checkWritableDir(state.SnapshotsDir())},
				{Name: "browser", Run: checkBrowserResolution},
				{Name: "state_permissions", Run: checkStatePermissions},
				{Name: "encryption_key_unused", Run: checkUnusedEncryptionKey},
			}}.Run()
			if err := writeJSON(cmd, result); err != nil {
				return err
			}
			if !result.OK {
				return errors.New("doctor checks failed")
			}
			return nil
		},
	}
	configureAgentHelp(cmd)
	return cmd
}

func checkWritableDir(dir string) func() doctor.Detail {
	return func() doctor.Detail {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return doctor.DetailFromError(err, "check directory permissions")
		}
		probe, err := os.CreateTemp(dir, ".doctor-write-test-*")
		if err != nil {
			return doctor.DetailFromError(err, "check directory permissions")
		}
		probePath := probe.Name()
		if _, err := probe.Write([]byte("ok")); err != nil {
			_ = probe.Close()
			_ = os.Remove(probePath)
			return doctor.DetailFromError(err, "check directory permissions")
		}
		if err := probe.Close(); err != nil {
			_ = os.Remove(probePath)
			return doctor.DetailFromError(err, "check directory permissions")
		}
		if err := os.Remove(probePath); err != nil {
			return doctor.DetailFromError(err, "check directory permissions")
		}
		return doctor.Detail{OK: true, Message: "writable"}
	}
}

func checkBrowserResolution() doctor.Detail {
	resolved, err := browser.Resolve("")
	if err != nil {
		return doctor.DetailFromError(err, "run `aget browser install`, set AGET_BROWSER_PATH, or pass --browser-path to open")
	}
	return doctor.Detail{OK: true, Message: resolved.Browser + " at " + resolved.Path}
}

// checkStatePermissions verifies that aget's state directories and their
// files are not world-readable. Existing code creates them with 0700/0600;
// this check detects drift caused by umask or manual changes.
func checkStatePermissions() doctor.Detail {
	dirs := []string{
		state.BaseDir(),
		state.SessionsDir(),
		state.SnapshotsDir(),
		state.ProfilesDir(),
		state.ArtifactsDir(),
	}
	sampleFiles := []string{}
	for _, dir := range dirs {
		fi, err := os.Stat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return doctor.Detail{OK: false, Message: fmt.Sprintf("cannot stat %s: %v", dir, err), Remediation: "check directory permissions"}
		}
		perm := fi.Mode().Perm()
		if perm&0o077 != 0 {
			return doctor.Detail{OK: false, Message: fmt.Sprintf("%s: world-readable (0%o), want 0700", dir, perm), Remediation: "run: chmod 0700 " + dir}
		}
		// Collect a sample file from each directory to check.
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !e.IsDir() && len(sampleFiles) < 5 {
				sampleFiles = append(sampleFiles, filepath.Join(dir, e.Name()))
			}
		}
	}
	for _, path := range sampleFiles {
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		perm := fi.Mode().Perm()
		if perm&0o077 != 0 {
			return doctor.Detail{OK: false, Message: fmt.Sprintf("%s: world-readable (0%o), want 0600", path, perm), Remediation: "run: chmod 0600 " + path}
		}
	}
	return doctor.Detail{OK: true, Message: "state files are private (0700/0600)"}
}

// checkUnusedEncryptionKey warns about AGET_ENCRYPTION_KEY being set but
// unused. It's informational-only: the env exists as a placeholder for future
// encryption-at-rest support.
func checkUnusedEncryptionKey() doctor.Detail {
	if os.Getenv("AGET_ENCRYPTION_KEY") != "" {
		return doctor.Detail{
			OK:          true,
			Message:     "AGET_ENCRYPTION_KEY is set but unused (no encryption-at-rest yet)",
			Remediation: "unset it to avoid confusion, or keep as a placeholder",
		}
	}
	return doctor.Detail{OK: true, Message: "no encryption key configured (fine — state files are 0600)"}
}
