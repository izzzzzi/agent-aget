package page

import (
	"context"
	"strings"

	"github.com/izzzzzi/agent-aget/internal/cdp"
)

type Service struct {
	driver cdp.Driver
}

type ReadOptions struct {
	Limit int
}

type ReadResult struct {
	OK        bool          `json:"ok"`
	URL       string        `json:"url"`
	Title     string        `json:"title"`
	Text      []string      `json:"text"`
	Truncated bool          `json:"truncated"`
	Links     []cdp.Element `json:"links,omitempty"`
	Buttons   []cdp.Element `json:"buttons,omitempty"`
	Inputs    []cdp.Element `json:"inputs,omitempty"`
}

func NewService(driver cdp.Driver) *Service {
	return &Service{driver: driver}
}

func (s *Service) Read(ctx context.Context, options ReadOptions) (ReadResult, error) {
	state, err := s.driver.Read(ctx)
	if err != nil {
		return ReadResult{}, err
	}

	lines := compactLines(state.Text)
	limit := options.Limit
	if limit <= 0 {
		limit = 80
	}

	truncated := false
	if len(lines) > limit {
		lines = lines[:limit]
		truncated = true
	}

	return ReadResult{
		OK:        true,
		URL:       state.URL,
		Title:     state.Title,
		Text:      lines,
		Truncated: truncated,
		Links:     state.Links,
		Buttons:   state.Buttons,
		Inputs:    state.Inputs,
	}, nil
}

func (s *Service) Click(ctx context.Context, selector string) error {
	return s.driver.Click(ctx, selector)
}

func (s *Service) Type(ctx context.Context, selector, text string) error {
	return s.driver.Type(ctx, selector, text)
}

func (s *Service) Screenshot(ctx context.Context, path string) error {
	return s.driver.Screenshot(ctx, path)
}

func compactLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}
