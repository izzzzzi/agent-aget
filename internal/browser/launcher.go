package browser

import (
	"fmt"
	"net"
	"os"
	"os/exec"
)

type LaunchOptions struct {
	BinaryPath  string
	URL         string
	UserDataDir string
	Port        int
	Headless    bool
}

type Process struct {
	PID      int
	DebugURL string
	cmd      *exec.Cmd
}

func FindFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("listener address is %T, want *net.TCPAddr", listener.Addr())
	}
	return addr.Port, nil
}

func Launch(options LaunchOptions) (*Process, error) {
	if err := os.MkdirAll(options.UserDataDir, 0700); err != nil {
		return nil, err
	}

	cmd := exec.Command(options.BinaryPath, buildArgs(options)...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &Process{
		PID:      cmd.Process.Pid,
		DebugURL: fmt.Sprintf("http://127.0.0.1:%d", options.Port),
		cmd:      cmd,
	}, nil
}

func (p *Process) Stop() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	killErr := p.cmd.Process.Kill()
	_ = p.cmd.Wait()
	return killErr
}

func buildArgs(options LaunchOptions) []string {
	args := []string{
		"--remote-debugging-address=127.0.0.1",
		fmt.Sprintf("--remote-debugging-port=%d", options.Port),
		"--user-data-dir=" + options.UserDataDir,
		"--no-first-run",
		"--no-default-browser-check",
	}
	if options.Headless {
		args = append(args, "--headless=new")
	}
	if options.URL != "" {
		args = append(args, options.URL)
	}
	return args
}
