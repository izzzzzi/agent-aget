package doctor

import (
	"errors"
	"testing"
)

func TestRunAggregatesChecks(t *testing.T) {
	runner := Runner{
		Checks: []Check{
			{Name: "state_dir", Run: func() Detail { return Detail{OK: true, Message: "writable"} }},
			{Name: "browser", Run: func() Detail {
				return Detail{OK: false, Message: "missing", Remediation: "run `aget browser install`"}
			}},
		},
	}

	got := runner.Run()

	if got.OK {
		t.Fatal("overall OK = true, want false")
	}
	if len(got.Checks) != 2 {
		t.Fatalf("len(checks) = %d, want 2", len(got.Checks))
	}
	if got.Checks[0].Name != "state_dir" || !got.Checks[0].OK {
		t.Fatalf("first check = %#v", got.Checks[0])
	}
	if got.Checks[1].Name != "browser" || got.Checks[1].Remediation == "" {
		t.Fatalf("second check = %#v", got.Checks[1])
	}
}

func TestRunRecoversPanickingCheck(t *testing.T) {
	runner := Runner{
		Checks: []Check{
			{Name: "state_dir", Run: func() Detail { return Detail{OK: true, Message: "writable"} }},
			{Name: "browser", Run: func() Detail { panic("boom") }},
			{Name: "sessions_dir", Run: func() Detail { return Detail{OK: true, Message: "writable"} }},
		},
	}

	got := runner.Run()

	if got.OK {
		t.Fatal("overall OK = true, want false")
	}
	if len(got.Checks) != 3 {
		t.Fatalf("checks = %#v", got.Checks)
	}
	if got.Checks[1].Name != "browser" || got.Checks[1].OK || got.Checks[1].Message != "panic: boom" {
		t.Fatalf("panic check = %#v", got.Checks[1])
	}
	if !got.Checks[2].OK {
		t.Fatalf("third check did not run: %#v", got.Checks[2])
	}
}

func TestDetailFromError(t *testing.T) {
	got := DetailFromError(errors.New("boom"), "repair")
	if got.OK || got.Message != "boom" || got.Remediation != "repair" {
		t.Fatalf("detail = %#v", got)
	}
}

func TestDetailFromNilError(t *testing.T) {
	got := DetailFromError(nil, "repair")
	if !got.OK || got.Message != "ok" || got.Remediation != "" {
		t.Fatalf("detail = %#v", got)
	}
}
