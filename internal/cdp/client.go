package cdp

import "context"

type Element struct {
	Ref      string `json:"ref,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Selector string `json:"selector"`
	Text     string `json:"text,omitempty"`
	Href     string `json:"href,omitempty"`
	Type     string `json:"type,omitempty"`
	Name     string `json:"name,omitempty"`
	Visible  bool   `json:"visible,omitempty"`
	Enabled  bool   `json:"enabled,omitempty"`
}

type PageState struct {
	URL     string    `json:"url"`
	Title   string    `json:"title"`
	Text    string    `json:"-"`
	Links   []Element `json:"links,omitempty"`
	Buttons []Element `json:"buttons,omitempty"`
	Inputs  []Element `json:"inputs,omitempty"`
}

type SnapshotState struct {
	URL      string    `json:"url"`
	Title    string    `json:"title"`
	Elements []Element `json:"elements"`
}

type WaitOptions struct {
	Selector string
	Text     string
	URL      string
	Load     string
}

type GetOptions struct {
	Kind     string
	Selector string
}

type Driver interface {
	Read(ctx context.Context) (PageState, error)
	Snapshot(ctx context.Context) (SnapshotState, error)
	Click(ctx context.Context, selector string) error
	Type(ctx context.Context, selector, text string) error
	Fill(ctx context.Context, selector, text string) error
	Press(ctx context.Context, key string) error
	Scroll(ctx context.Context, direction string, pixels int) error
	Wait(ctx context.Context, options WaitOptions) error
	Get(ctx context.Context, options GetOptions) (string, error)
	Screenshot(ctx context.Context, path string) error
	Close(ctx context.Context) error
}
