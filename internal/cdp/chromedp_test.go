package cdp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestClickReturnsCanceledContextWithoutRunningAction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	driver := &ChromeDPDriver{
		ctx: context.Background(),
		run: func(ctx context.Context, actions ...chromedp.Action) error {
			t.Fatal("runner should not be called for an already canceled context")
			return nil
		},
	}

	err := driver.Click(ctx, "#login")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Click error = %v, want context.Canceled", err)
	}
}

func TestRunActionsCancelsWhenCallContextIsCanceled(t *testing.T) {
	callCtx, cancelCall := context.WithCancel(context.Background())
	started := make(chan context.Context, 1)
	done := make(chan error, 1)
	driver := &ChromeDPDriver{
		ctx: context.Background(),
		run: func(ctx context.Context, actions ...chromedp.Action) error {
			started <- ctx
			<-ctx.Done()
			return ctx.Err()
		},
	}

	go func() {
		done <- driver.Click(callCtx, "#login")
	}()

	var runCtx context.Context
	select {
	case runCtx = <-started:
	case <-time.After(time.Second):
		t.Fatal("runner was not called")
	}

	cancelCall()

	select {
	case <-runCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("runner context was not canceled")
	}

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Click error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Click did not return after call context cancellation")
	}
}

func TestCallContextPreservesCallDeadline(t *testing.T) {
	callDeadline := time.Now().Add(50 * time.Millisecond)
	callCtx, cancelCall := context.WithDeadline(context.Background(), callDeadline)
	defer cancelCall()
	driver := &ChromeDPDriver{ctx: context.Background()}

	runCtx, cancelRun, err := driver.callContext(callCtx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelRun()

	gotDeadline, ok := runCtx.Deadline()
	if !ok {
		t.Fatal("run context has no deadline")
	}
	if !gotDeadline.Equal(callDeadline) {
		t.Fatalf("deadline = %v, want %v", gotDeadline, callDeadline)
	}

	<-runCtx.Done()
	if !errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		t.Fatalf("run context error = %v, want context.DeadlineExceeded", runCtx.Err())
	}
}

func TestCallContextUsesEarliestDeadline(t *testing.T) {
	driverCtx, cancelDriver := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancelDriver()
	callDeadline := time.Now().Add(100 * time.Millisecond)
	callCtx, cancelCall := context.WithDeadline(context.Background(), callDeadline)
	defer cancelCall()
	driver := &ChromeDPDriver{ctx: driverCtx}

	runCtx, cancelRun, err := driver.callContext(callCtx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelRun()

	gotDeadline, ok := runCtx.Deadline()
	if !ok {
		t.Fatal("run context has no deadline")
	}
	if !gotDeadline.Equal(callDeadline) {
		t.Fatalf("deadline = %v, want earliest %v", gotDeadline, callDeadline)
	}
}

func TestWriteScreenshotOverwritesWithPrivateMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "screenshot.png")
	if err := os.WriteFile(path, []byte("old"), 0o666); err != nil {
		t.Fatal(err)
	}

	if err := writeScreenshot(path, []byte("new")); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "new" {
		t.Fatalf("body = %q, want %q", string(body), "new")
	}

	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %v, want 0600", got)
	}
}
