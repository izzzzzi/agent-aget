package cdp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

type actionRunner func(ctx context.Context, actions ...chromedp.Action) error

type ChromeDPDriver struct {
	ctx    context.Context
	cancel context.CancelFunc
	run    actionRunner
}

func NewChromeDPDriver(parent context.Context, debugURL string) (*ChromeDPDriver, error) {
	if parent == nil {
		parent = context.Background()
	}
	targetID, err := fetchFirstPageTargetID(parent, debugURL)
	if err != nil {
		return nil, err
	}

	allocatorCtx, allocatorCancel := chromedp.NewRemoteAllocator(context.Background(), debugURL)
	ctx, cancel := chromedp.NewContext(
		allocatorCtx,
		chromedp.WithTargetID(targetID),
		chromedp.WithLogf(func(string, ...any) {}),
	)
	return &ChromeDPDriver{
		ctx: ctx,
		cancel: func() {
			cancel()
			allocatorCancel()
		},
		run: chromedp.Run,
	}, nil
}

type pageTarget struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

func fetchFirstPageTargetID(ctx context.Context, debugURL string) (target.ID, error) {
	targets, err := fetchPageTargets(ctx, debugURL)
	if err != nil {
		return "", err
	}
	for _, candidate := range targets {
		if candidate.Type == "page" && candidate.ID != "" && candidate.URL != "about:blank" {
			return target.ID(candidate.ID), nil
		}
	}
	for _, candidate := range targets {
		if candidate.Type == "page" && candidate.ID != "" {
			return target.ID(candidate.ID), nil
		}
	}
	return "", fmt.Errorf("no page target found at %s", strings.TrimRight(debugURL, "/")+"/json/list")
}

func WaitForPageURL(ctx context.Context, debugURL, pageURL string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for {
		targets, err := fetchPageTargets(ctx, debugURL)
		if err == nil {
			hasReadyPage := false
			for _, candidate := range targets {
				if candidate.Type != "page" || candidate.ID == "" || candidate.URL == "" || candidate.URL == "about:blank" {
					continue
				}
				if pageURL == "" || candidate.URL == pageURL {
					return nil
				}
				hasReadyPage = true
			}
			if hasReadyPage {
				return nil
			}
			lastErr = fmt.Errorf("page target for %s not ready", pageURL)
		} else {
			lastErr = err
		}

		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			if lastErr != nil {
				return lastErr
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func fetchPageTargets(ctx context.Context, debugURL string) ([]pageTarget, error) {
	url := strings.TrimRight(debugURL, "/") + "/json/list"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("devtools target list returned %s", resp.Status)
	}
	var targets []pageTarget
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, err
	}
	return targets, nil
}

func (d *ChromeDPDriver) Read(ctx context.Context) (PageState, error) {
	var state PageState
	if err := d.runActionsWithTransientRetry(ctx,
		waitForReadableBody(),
		chromedp.Location(&state.URL),
		chromedp.Title(&state.Title),
		chromedp.Text("body", &state.Text, chromedp.ByQuery),
	); err != nil {
		return PageState{}, err
	}
	return state, nil
}

func waitForReadableBody() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		err := chromedp.Poll(
			`document.body && document.body.innerText && document.body.innerText.trim().length > 0`,
			nil,
			chromedp.WithPollingInterval(100*time.Millisecond),
			chromedp.WithPollingTimeout(5*time.Second),
		).Do(ctx)
		if errors.Is(err, chromedp.ErrPollingTimeout) {
			return nil
		}
		return err
	})
}

func (d *ChromeDPDriver) Click(ctx context.Context, selector string) error {
	return d.runActions(ctx, chromedp.Click(selector, chromedp.ByQuery))
}

func (d *ChromeDPDriver) Type(ctx context.Context, selector, text string) error {
	return d.runActions(ctx, chromedp.SendKeys(selector, text, chromedp.ByQuery))
}

func (d *ChromeDPDriver) Screenshot(ctx context.Context, path string) error {
	var body []byte
	if err := d.runActionsWithTransientRetry(ctx, chromedp.FullScreenshot(&body, 90)); err != nil {
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
	runCtx, _, err := d.callContext(ctx)
	if err != nil {
		return err
	}
	return run(runCtx, actions...)
}

func (d *ChromeDPDriver) runActionsWithTransientRetry(ctx context.Context, actions ...chromedp.Action) error {
	waitCtx := ctx
	if waitCtx == nil {
		waitCtx = context.Background()
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := d.runActions(ctx, actions...)
		if err == nil || !isTransientPageReadinessError(err) {
			return err
		}
		if !time.Now().Before(deadline) {
			return err
		}

		timer := time.NewTimer(150 * time.Millisecond)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			return waitCtx.Err()
		case <-timer.C:
		}
	}
}

func isTransientPageReadinessError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "Execution context was destroyed") ||
		strings.Contains(message, "Cannot take screenshot with 0 width") ||
		strings.Contains(message, "Cannot take screenshot with 0 height")
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
