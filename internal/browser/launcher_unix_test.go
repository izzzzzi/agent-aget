//go:build unix

package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStopKillsSpawnedChildProcess(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "fake-browser")
	childPIDPath := filepath.Join(dir, "child.pid")
	helperLogPath := filepath.Join(dir, "helper.log")
	testExe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nAGET_BROWSER_HELPER=1 AGET_CHILD_PID_PATH=" + strconv.Quote(childPIDPath) +
		" exec " + strconv.Quote(testExe) + " -test.run=TestBrowserChildHelper -- > " + strconv.Quote(helperLogPath) + " 2>&1\n"
	if err := os.WriteFile(exe, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	process, err := Launch(LaunchOptions{
		BinaryPath:  exe,
		UserDataDir: filepath.Join(dir, "profile"),
		Port:        9335,
	})
	if err != nil {
		t.Fatal(err)
	}

	childPID := readChildPID(t, childPIDPath, helperLogPath)
	t.Cleanup(func() {
		_ = syscall.Kill(childPID, syscall.SIGKILL)
	})

	if err := process.Stop(); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !processExists(childPID) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("child process %d still exists after Stop", childPID)
}

func TestBrowserChildHelper(t *testing.T) {
	if os.Getenv("AGET_BROWSER_HELPER") != "1" {
		return
	}

	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("AGET_CHILD_PID_PATH"), []byte(strconv.Itoa(cmd.Process.Pid)), 0600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	_ = cmd.Wait()
	os.Exit(0)
}

func readChildPID(t *testing.T, path, logPath string) int {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		body, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.Atoi(strings.TrimSpace(string(body)))
			if err != nil {
				t.Fatal(err)
			}
			return pid
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if body, err := os.ReadFile(logPath); err == nil && len(body) > 0 {
		t.Logf("helper log:\n%s", body)
	}
	t.Fatalf("timed out waiting for child pid file %s", path)
	return 0
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
