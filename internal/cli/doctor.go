package cli

import (
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
			}}.Run()
			return writeJSON(cmd, result)
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
		probe := filepath.Join(dir, ".doctor-write-test")
		if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
			return doctor.DetailFromError(err, "check directory permissions")
		}
		if err := os.Remove(probe); err != nil {
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
