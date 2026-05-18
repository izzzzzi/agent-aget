//go:build !unix && !windows

package browser

import "os/exec"

func configureCommand(cmd *exec.Cmd) {}

func stopCommand(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
