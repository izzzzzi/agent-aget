package cdp

import "context"

type Element struct {
	Selector string `json:"selector"`
	Text     string `json:"text,omitempty"`
	Href     string `json:"href,omitempty"`
	Type     string `json:"type,omitempty"`
	Name     string `json:"name,omitempty"`
}

type PageState struct {
	URL     string    `json:"url"`
	Title   string    `json:"title"`
	Text    string    `json:"-"`
	Links   []Element `json:"links,omitempty"`
	Buttons []Element `json:"buttons,omitempty"`
	Inputs  []Element `json:"inputs,omitempty"`
}

type Driver interface {
	Read(ctx context.Context) (PageState, error)
	Click(ctx context.Context, selector string) error
	Type(ctx context.Context, selector, text string) error
	Screenshot(ctx context.Context, path string) error
	Close(ctx context.Context) error
}
