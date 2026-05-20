package browser

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestBuildArgsHeadless(t *testing.T) {
	args := buildArgs(LaunchOptions{
		URL:         "https://example.com",
		UserDataDir: "/tmp/profile",
		Port:        9333,
		Headless:    true,
	})

	assertContains(t, args, "--remote-debugging-address=127.0.0.1")
	assertContains(t, args, "--remote-debugging-port=9333")
	assertContains(t, args, "--user-data-dir=/tmp/profile")
	assertContains(t, args, "--headless=new")
	assertContains(t, args, "https://example.com")
}

func TestBuildArgsCloakBrowserAddsStealthDefaults(t *testing.T) {
	args := buildArgs(LaunchOptions{
		BrowserName:  "cloakbrowser",
		URL:          "https://example.com",
		UserDataDir:  "/tmp/profile",
		Port:         9333,
		Headless:     true,
		Fingerprint:  "12345",
		PlatformName: "macos",
	})

	assertContains(t, args, "--no-sandbox")
	assertContains(t, args, "--fingerprint=12345")
	assertContains(t, args, "--fingerprint-platform=macos")
	assertContains(t, args, "--headless=new")
}

func TestBuildArgsCloakBrowserDefaultsFingerprintPlatform(t *testing.T) {
	args := buildArgs(LaunchOptions{
		BrowserName: "cloakbrowser",
		UserDataDir: "/tmp/profile",
		Port:        9333,
	})

	assertContainsPrefix(t, args, "--fingerprint=")
	if runtime.GOOS == "darwin" {
		assertContains(t, args, "--fingerprint-platform=macos")
	} else {
		assertContains(t, args, "--fingerprint-platform=windows")
	}
}

func TestFindFreePort(t *testing.T) {
	port, err := FindFreePort()
	if err != nil {
		t.Fatal(err)
	}
	if port <= 0 {
		t.Fatalf("FindFreePort() = %d, want > 0", port)
	}
}

func TestLaunchWithFakeExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake executable is not portable to windows")
	}

	dir := t.TempDir()
	exe := filepath.Join(dir, "fake-browser")
	logPath := filepath.Join(dir, "args.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + logPath + "\nsleep 5\n"
	if err := os.WriteFile(exe, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	process, err := Launch(LaunchOptions{
		BinaryPath:  exe,
		URL:         "https://example.com",
		UserDataDir: filepath.Join(dir, "profile"),
		Port:        9334,
		Headless:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := process.Stop(); err != nil {
			t.Fatal(err)
		}
	}()

	if process.PID <= 0 {
		t.Fatalf("PID = %d, want > 0", process.PID)
	}
}

func assertContains(t *testing.T, args []string, want string) {
	t.Helper()
	if !slices.Contains(args, want) {
		t.Fatalf("args = %v, want %q", args, want)
	}
}

func assertContainsPrefix(t *testing.T, args []string, prefix string) {
	t.Helper()
	for _, arg := range args {
		if len(arg) >= len(prefix) && arg[:len(prefix)] == prefix {
			return
		}
	}
	t.Fatalf("args = %v, want prefix %q", args, prefix)
}
