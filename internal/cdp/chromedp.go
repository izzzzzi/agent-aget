package cdp

import (
	"context"
	"os"

	"github.com/chromedp/chromedp"
)

type ChromeDPDriver struct {
	ctx    context.Context
	cancel context.CancelFunc
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
	}, nil
}

func (d *ChromeDPDriver) Read(ctx context.Context) (PageState, error) {
	var state PageState
	if err := chromedp.Run(d.ctx,
		chromedp.Location(&state.URL),
		chromedp.Title(&state.Title),
		chromedp.Text("body", &state.Text, chromedp.ByQuery),
	); err != nil {
		return PageState{}, err
	}
	return state, nil
}

func (d *ChromeDPDriver) Click(ctx context.Context, selector string) error {
	return chromedp.Run(d.ctx, chromedp.Click(selector, chromedp.ByQuery))
}

func (d *ChromeDPDriver) Type(ctx context.Context, selector, text string) error {
	return chromedp.Run(d.ctx, chromedp.SendKeys(selector, text, chromedp.ByQuery))
}

func (d *ChromeDPDriver) Screenshot(ctx context.Context, path string) error {
	var body []byte
	if err := chromedp.Run(d.ctx, chromedp.FullScreenshot(&body, 90)); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o600)
}

func (d *ChromeDPDriver) Close(ctx context.Context) error {
	d.cancel()
	return nil
}
