package cdp

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
)

type actionRunner func(ctx context.Context, actions ...chromedp.Action) error

type ChromeDPDriver struct {
	ctx    context.Context
	cancel context.CancelFunc
	run    actionRunner
}

func NewChromeDPDriver(parent context.Context, debugURL string) (*ChromeDPDriver, error) {
	allocatorCtx, allocatorCancel := chromedp.NewRemoteAllocator(parent, debugURL)
	ctx, cancel := chromedp.NewContext(allocatorCtx)
	return &ChromeDPDriver{
		ctx: ctx,
		cancel: func() {
			cancel()
			allocatorCancel()
		},
		run: chromedp.Run,
	}, nil
}

func (d *ChromeDPDriver) Read(ctx context.Context) (PageState, error) {
	var state PageState
	if err := d.runActions(ctx,
		chromedp.Location(&state.URL),
		chromedp.Title(&state.Title),
		chromedp.Text("body", &state.Text, chromedp.ByQuery),
	); err != nil {
		return PageState{}, err
	}
	return state, nil
}

func (d *ChromeDPDriver) Click(ctx context.Context, selector string) error {
	return d.runActions(ctx, chromedp.Click(selector, chromedp.ByQuery))
}

func (d *ChromeDPDriver) Type(ctx context.Context, selector, text string) error {
	return d.runActions(ctx, chromedp.SendKeys(selector, text, chromedp.ByQuery))
}

func (d *ChromeDPDriver) Screenshot(ctx context.Context, path string) error {
	var body []byte
	if err := d.runActions(ctx, chromedp.FullScreenshot(&body, 90)); err != nil {
		return err
	}
	return writeScreenshot(path, body)
}

func (d *ChromeDPDriver) Close(ctx context.Context) error {
	d.cancel()
	return nil
}

func (d *ChromeDPDriver) runActions(ctx context.Context, actions ...chromedp.Action) error {
	run := d.run
	if run == nil {
		run = chromedp.Run
	}
	runCtx, cancel, err := d.callContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	return run(runCtx, actions...)
}

func (d *ChromeDPDriver) callContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if err := d.ctx.Err(); err != nil {
		return nil, nil, err
	}

	runCtx, cancel := contextWithEarliestDeadline(d.ctx, ctx)
	go func() {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return
			}
			cancel()
		case <-runCtx.Done():
		}
	}()
	return runCtx, cancel, nil
}

func contextWithEarliestDeadline(parent, call context.Context) (context.Context, context.CancelFunc) {
	deadline, ok := earliestDeadline(parent, call)
	if ok {
		return context.WithDeadline(parent, deadline)
	}
	return context.WithCancel(parent)
}

func earliestDeadline(contexts ...context.Context) (time.Time, bool) {
	var earliest time.Time
	found := false
	for _, ctx := range contexts {
		deadline, ok := ctx.Deadline()
		if !ok {
			continue
		}
		if !found || deadline.Before(earliest) {
			earliest = deadline
			found = true
		}
	}
	return earliest, found
}

func writeScreenshot(path string, body []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".screenshot-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}
