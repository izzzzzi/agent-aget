//go:build !unix && !windows

package browser

import (
	"os"
	"os/exec"
)

func configureCommand(cmd *exec.Cmd) {}

func stopCommand(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}

func stopPID(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}
