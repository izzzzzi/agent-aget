package browser

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type LaunchOptions struct {
	BinaryPath   string
	BrowserName  string
	URL          string
	UserDataDir  string
	Port         int
	Headless     bool
	Fingerprint  string
	PlatformName string
	WindowWidth  int
	WindowHeight int
}

type Process struct {
	PID      int
	DebugURL string
	cmd      *exec.Cmd
}

func FindFreePort() (int, error) {
	// The returned port can be claimed before the browser starts; callers must handle launch and readiness errors.
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
	configureCommand(cmd)
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
	killErr := stopCommand(p.cmd)
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
	if strings.EqualFold(options.BrowserName, "cloakbrowser") {
		args = append(args,
			"--no-sandbox",
			"--fingerprint="+cloakFingerprint(options.Fingerprint),
			"--fingerprint-platform="+cloakPlatformName(options.PlatformName),
		)
	}
	if options.Headless {
		args = append(args, "--headless=new")
	}
	if options.WindowWidth > 0 && options.WindowHeight > 0 {
		args = append(args, fmt.Sprintf("--window-size=%d,%d", options.WindowWidth, options.WindowHeight))
	}
	if options.URL != "" {
		args = append(args, options.URL)
	}
	return args
}

func cloakFingerprint(seed string) string {
	if seed != "" {
		return seed
	}
	max := big.NewInt(900000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "123456"
	}
	return fmt.Sprintf("%06d", n.Int64()+100000)
}

func cloakPlatformName(name string) string {
	if name != "" {
		return name
	}
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return "windows"
}
