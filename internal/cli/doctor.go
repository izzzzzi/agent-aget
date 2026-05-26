package cli

import (
	"errors"
	"os"

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
