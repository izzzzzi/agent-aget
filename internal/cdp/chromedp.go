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

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	cdptarget "github.com/chromedp/cdproto/target"
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

func fetchFirstPageTargetID(ctx context.Context, debugURL string) (cdptarget.ID, error) {
	targets, err := fetchPageTargets(ctx, debugURL)
	if err != nil {
		return "", err
	}
	for _, candidate := range targets {
		if candidate.Type == "page" && candidate.ID != "" && candidate.URL != "about:blank" {
			return cdptarget.ID(candidate.ID), nil
		}
	}
	for _, candidate := range targets {
		if candidate.Type == "page" && candidate.ID != "" {
			return cdptarget.ID(candidate.ID), nil
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

func (d *ChromeDPDriver) Snapshot(ctx context.Context) (SnapshotState, error) {
	var state SnapshotState
	var raw string
	if err := d.runActionsWithTransientRetry(ctx,
		waitForReadableBody(),
		chromedp.Location(&state.URL),
		chromedp.Title(&state.Title),
		chromedp.Evaluate(snapshotScript(), &raw),
	); err != nil {
		return SnapshotState{}, err
	}
	if err := json.Unmarshal([]byte(raw), &state.Elements); err != nil {
		return SnapshotState{}, err
	}
	return state, nil
}

func snapshotScript() string {
	return `(() => {
	  const cssEscape = (value) => {
	    if (window.CSS && CSS.escape) return CSS.escape(value);
	    return String(value).replace(/\\/g, '\\\\').replace(/"/g, '\\"');
	  };
	  const cssStringEscape = (value) => (
	    String(value)
	      .replace(/\\/g, '\\\\')
	      .replace(/"/g, '\\"')
	      .replace(/\n/g, '\\A ')
	      .replace(/\r/g, '\\D ')
	      .replace(/\f/g, '\\C ')
	  );
	  const pointsToElement = (selector, el) => {
	    try {
	      return document.querySelector(selector) === el;
	    } catch {
	      return false;
	    }
	  };
	  const uniqueElement = (selector, el) => {
	    try {
	      const matches = document.querySelectorAll(selector);
	      return matches.length === 1 && matches[0] === el;
	    } catch {
	      return false;
	    }
	  };
	  const selectorFor = (el) => {
	    if (el.id) {
	      const selector = '#' + cssEscape(el.id);
	      if (uniqueElement(selector, el)) return selector;
	    }
	    const testid = el.getAttribute('data-testid');
	    if (testid) {
	      const selector = '[data-testid="' + cssStringEscape(testid) + '"]';
	      if (uniqueElement(selector, el)) return selector;
	    }
	    const name = el.getAttribute('name');
	    if (name) {
	      const selector = el.tagName.toLowerCase() + '[name="' + cssStringEscape(name) + '"]';
	      if (uniqueElement(selector, el)) return selector;
	    }
	    const parts = [];
	    let current = el;
	    while (current && current.nodeType === Node.ELEMENT_NODE) {
	      let part = current.tagName.toLowerCase();
	      const parent = current.parentElement;
	      if (parent) {
	        const same = Array.from(parent.children).filter((child) => child.tagName === current.tagName);
	        if (same.length > 1) part += ':nth-of-type(' + (same.indexOf(current) + 1) + ')';
	      }
	      parts.unshift(part);
	      current = parent;
	    }
	    const selector = parts.join(' > ');
	    return pointsToElement(selector, el) ? selector : '';
	  };
	  const visible = (el) => {
	    const rect = el.getBoundingClientRect();
	    const style = window.getComputedStyle(el);
	    return rect.width > 0 && rect.height > 0 && style.visibility !== 'hidden' && style.display !== 'none';
	  };
	  const enabled = (el) => !el.disabled && el.getAttribute('aria-disabled') !== 'true';
	  const kindFor = (el) => {
	    const role = (el.getAttribute('role') || '').toLowerCase();
	    const tag = (el.tagName || '').toLowerCase();
	    if (role === 'button' || tag === 'button') return 'button';
	    if (role === 'link' || tag === 'a') return 'link';
	    if (tag === 'select') return 'select';
	    if (tag === 'input' || tag === 'textarea') return 'input';
	    return role || tag;
	  };
	  const textFor = (el) => {
	    const tag = (el.tagName || '').toLowerCase();
	    if (tag === 'input' || tag === 'textarea' || tag === 'select') {
	      return (
	        el.getAttribute('aria-label') ||
	        el.getAttribute('placeholder') ||
	        el.getAttribute('name') ||
	        ''
	      ).trim().slice(0, 200);
	    }
	    return (
	      el.innerText || el.getAttribute('aria-label') || el.getAttribute('placeholder') || ''
	    ).trim().slice(0, 200);
	  };
	  const candidates = Array.from(document.querySelectorAll('a,button,input,textarea,select,[role=button],[role=link],[role=option],[role=checkbox],[role=radio],[role=switch],[role=menuitem],[role=listbox],[role=listitem],[tabindex],[onclick],div[data-group-id],span[data-id]'));
	  return JSON.stringify(candidates.slice(0, 200).map((el, index) => ({
	    ref: '@e' + (index + 1),
	    kind: kindFor(el),
	    selector: selectorFor(el),
	    text: textFor(el),
	    href: el.href || '',
	    type: el.getAttribute('type') || '',
	    name: el.getAttribute('name') || '',
	    visible: visible(el),
	    enabled: enabled(el)
	  })));
	})()`
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

func (d *ChromeDPDriver) ClickForce(ctx context.Context, selector string) error {
	script := fmt.Sprintf(`(() => {
	  const e = document.querySelector(%q);
	  if (!e) throw new Error('element not found');
	  const r = e.getBoundingClientRect();
	  if (r.width === 0 || r.height === 0) {
	    // Try to scroll into view and re-check
	    e.scrollIntoView({block:'center'});
	    const r2 = e.getBoundingClientRect();
	    return [r2.left + r2.width/2, r2.top + r2.height/2, Math.max(r2.width,1), Math.max(r2.height,1)];
	  }
	  return [r.left + r.width/2, r.top + r.height/2, r.width, r.height];
	})()`, selector)
	var result []float64
	if err := d.runActions(ctx, chromedp.Evaluate(script, &result)); err != nil {
		return err
	}
	if len(result) < 2 {
		return fmt.Errorf("click force: element %s has no dimensions", selector)
	}
	x, y := result[0], result[1]
	return d.runActions(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return mouseClickAt(ctx, x, y)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Also dispatch trusted DOM click as fallback
			script := fmt.Sprintf(`(()=>{const e=document.querySelector(%q);if(e){const ev=new MouseEvent('click',{bubbles:true,cancelable:true,view:window,clientX:%f,clientY:%f});e.dispatchEvent(ev)}})()`, selector, x, y)
			return chromedp.Evaluate(script, nil).Do(ctx)
		}),
	)
}

func mouseClickAt(ctx context.Context, x, y float64) error {
	if x == 0 {
		x = 1
	}
	if y == 0 {
		y = 1
	}
	if err := input.DispatchMouseEvent(input.MousePressed, x, y).
		WithButton(input.Left).
		WithClickCount(1).
		Do(ctx); err != nil {
		return err
	}
	if err := input.DispatchMouseEvent(input.MouseReleased, x, y).
		WithButton(input.Left).
		WithClickCount(1).
		Do(ctx); err != nil {
		return err
	}
	return nil
}

func (d *ChromeDPDriver) Type(ctx context.Context, selector, text string) error {
	return d.runActions(ctx, chromedp.SendKeys(selector, text, chromedp.ByQuery))
}

func (d *ChromeDPDriver) Fill(ctx context.Context, selector, text string) error {
	return d.runActions(ctx,
		chromedp.SetValue(selector, "", chromedp.ByQuery),
		chromedp.SendKeys(selector, text, chromedp.ByQuery),
	)
}

func (d *ChromeDPDriver) Select(ctx context.Context, selector, value string) error {
	script := fmt.Sprintf(`(()=>{const e=document.querySelector(%q);if(!e||e.tagName!=='SELECT')throw new Error('not a select element');const wanted=%q;const options=Array.from(e.options);const option=options.find(o=>o.value===wanted)||options.find(o=>o.textContent.trim()===wanted);if(!option)throw new Error('select option not found: '+wanted);e.value=option.value;e.dispatchEvent(new Event('change',{bubbles:true}));e.dispatchEvent(new Event('input',{bubbles:true}));if(window.jQuery){try{window.jQuery(e).trigger('change');window.jQuery(e).trigger('change.select2')}catch(_){}}})()`, selector, value)
	return d.runActions(ctx, chromedp.Evaluate(script, nil))
}

func (d *ChromeDPDriver) Is(ctx context.Context, selector, prop string) (bool, error) {
	checks := map[string]string{
		"visible":  fmt.Sprintf(`(()=>{const e=document.querySelector(%q);if(!e)return false;const r=e.getBoundingClientRect();const s=window.getComputedStyle(e);return r.width>0&&r.height>0&&s.visibility!=='hidden'&&s.display!=='none';})()`, selector),
		"hidden":   fmt.Sprintf(`(()=>{const e=document.querySelector(%q);if(!e)return true;const r=e.getBoundingClientRect();const s=window.getComputedStyle(e);return r.width===0||r.height===0||s.visibility==='hidden'||s.display==='none';})()`, selector),
		"enabled":  fmt.Sprintf(`(()=>{const e=document.querySelector(%q);return e!==null&&!e.disabled&&e.getAttribute('aria-disabled')!=='true';})()`, selector),
		"disabled": fmt.Sprintf(`(()=>{const e=document.querySelector(%q);return e===null||e.disabled||e.getAttribute('aria-disabled')==='true';})()`, selector),
		"checked":  fmt.Sprintf(`(()=>{const e=document.querySelector(%q);return e!==null&&(e.checked===true||e.getAttribute('aria-checked')==='true');})()`, selector),
		"editable": fmt.Sprintf(`(()=>{const e=document.querySelector(%q);if(!e)return false;if(e.readOnly||e.disabled)return false;const t=e.tagName;return t==='INPUT'||t==='TEXTAREA'||t==='SELECT'||e.getAttribute('contenteditable')==='true';})()`, selector),
		"focused":  fmt.Sprintf(`document.querySelector(%q)===document.activeElement`, selector),
	}
	script, ok := checks[prop]
	if !ok {
		return false, fmt.Errorf("unsupported is prop %q", prop)
	}
	var result bool
	return result, d.runActions(ctx, chromedp.Evaluate(script, &result))
}

func (d *ChromeDPDriver) Eval(ctx context.Context, expression string) (string, error) {
	var out json.RawMessage
	if err := d.runActions(ctx, chromedp.Evaluate(expression, &out)); err != nil {
		return "", err
	}
	return string(out), nil
}

func (d *ChromeDPDriver) Check(ctx context.Context, selector string) error {
	script := fmt.Sprintf(`(()=>{const e=document.querySelector(%q);if(e&&!e.checked){e.click()}})()`, selector)
	return d.runActions(ctx, chromedp.Evaluate(script, nil))
}

func (d *ChromeDPDriver) Uncheck(ctx context.Context, selector string) error {
	script := fmt.Sprintf(`(()=>{const e=document.querySelector(%q);if(e&&e.checked){e.click()}})()`, selector)
	return d.runActions(ctx, chromedp.Evaluate(script, nil))
}

func (d *ChromeDPDriver) Hover(ctx context.Context, selector string) error {
	script := fmt.Sprintf(`(()=>{const e=document.querySelector(%q);if(!e)throw new Error('element not found');e.focus({preventScroll:true});['mouseover','mouseenter','mousemove'].forEach(t=>e.dispatchEvent(new MouseEvent(t,{bubbles:true,cancelable:true})));})()`, selector)
	return d.runActions(ctx, chromedp.Evaluate(script, nil))
}

func (d *ChromeDPDriver) Focus(ctx context.Context, selector string) error {
	return d.runActions(ctx, chromedp.Focus(selector, chromedp.ByQuery))
}

func (d *ChromeDPDriver) Upload(ctx context.Context, selector string, files []string) error {
	return d.runActions(ctx, chromedp.SetUploadFiles(selector, files, chromedp.ByQuery))
}

func (d *ChromeDPDriver) DialogAccept(ctx context.Context, promptText string) error {
	chromedp.ListenTarget(d.ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *page.EventJavascriptDialogOpening:
			go func() {
				action := page.HandleJavaScriptDialog(true)
				if e.Type == page.DialogTypePrompt && promptText != "" {
					action.PromptText = promptText
				}
				_ = action.Do(cdp.WithExecutor(d.ctx, chromedp.FromContext(d.ctx).Browser))
			}()
		}
	})
	return nil
}

func (d *ChromeDPDriver) DialogDismiss(ctx context.Context) error {
	chromedp.ListenTarget(d.ctx, func(ev interface{}) {
		switch ev.(type) {
		case *page.EventJavascriptDialogOpening:
			go func() {
				_ = page.HandleJavaScriptDialog(false).Do(cdp.WithExecutor(d.ctx, chromedp.FromContext(d.ctx).Browser))
			}()
		}
	})
	return nil
}

func (d *ChromeDPDriver) Press(ctx context.Context, key string) error {
	return d.runActions(ctx, chromedp.KeyEvent(key))
}

func (d *ChromeDPDriver) Scroll(ctx context.Context, direction string, pixels int) error {
	if pixels <= 0 {
		pixels = 800
	}
	x, y := 0, 0
	switch direction {
	case "up":
		y = -pixels
	case "down":
		y = pixels
	case "left":
		x = -pixels
	case "right":
		x = pixels
	default:
		return fmt.Errorf("unsupported scroll direction %q", direction)
	}
	return d.runActions(ctx, chromedp.Evaluate(fmt.Sprintf(`window.scrollBy(%d, %d)`, x, y), nil))
}

func (d *ChromeDPDriver) WaitAppear(ctx context.Context, selector string) error {
	expr := fmt.Sprintf(`document.querySelector(%q) !== null`, selector)
	return d.runActions(ctx, chromedp.Poll(expr, nil, chromedp.WithPollingInterval(100*time.Millisecond)))
}

func (d *ChromeDPDriver) Wait(ctx context.Context, options WaitOptions) error {
	switch {
	case options.Selector != "":
		return d.runActions(ctx, chromedp.WaitVisible(options.Selector, chromedp.ByQuery))
	case options.Text != "":
		expr := fmt.Sprintf(`document.body && document.body.innerText.includes(%q)`, options.Text)
		return d.runActions(ctx, chromedp.Poll(expr, nil, chromedp.WithPollingInterval(100*time.Millisecond)))
	case options.URL != "":
		expr := fmt.Sprintf(`location.href.includes(%q)`, strings.Trim(options.URL, "*"))
		return d.runActions(ctx, chromedp.Poll(expr, nil, chromedp.WithPollingInterval(100*time.Millisecond)))
	case options.Load != "":
		return d.runActions(ctx, chromedp.WaitReady("body", chromedp.ByQuery))
	default:
		return errors.New("wait condition required")
	}
}

func (d *ChromeDPDriver) Get(ctx context.Context, options GetOptions) (string, error) {
	var out string
	switch options.Kind {
	case "url":
		err := d.runActions(ctx, chromedp.Location(&out))
		return out, err
	case "title":
		err := d.runActions(ctx, chromedp.Title(&out))
		return out, err
	case "text":
		err := d.runActions(ctx, chromedp.Text(options.Selector, &out, chromedp.ByQuery))
		return out, err
	case "html":
		err := d.runActions(ctx, chromedp.InnerHTML(options.Selector, &out, chromedp.ByQuery))
		return out, err
	case "value":
		script := fmt.Sprintf(`document.querySelector(%q)?.value ?? ""`, options.Selector)
		err := d.runActions(ctx, chromedp.Evaluate(script, &out))
		return out, err
	default:
		return "", fmt.Errorf("unsupported get kind %q", options.Kind)
	}
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
