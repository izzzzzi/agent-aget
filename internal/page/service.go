package page

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/izzzzzi/agent-aget/internal/cdp"
	"github.com/izzzzzi/agent-aget/internal/snapshot"
)

var (
	ErrTargetRequired = errors.New("target selector or ref required")
	ErrRefNotFound    = snapshot.ErrRefNotFound
)

type RefResolver interface {
	Save(record snapshot.Record) error
	Resolve(sid, ref string) (snapshot.Element, error)
}

type Service struct {
	driver   cdp.Driver
	resolver RefResolver
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

type SnapshotOptions struct {
	SID string
}

type SnapshotResult struct {
	OK           bool          `json:"ok"`
	SID          string        `json:"sid"`
	URL          string        `json:"url"`
	Title        string        `json:"title"`
	Elements     []cdp.Element `json:"elements"`
	NextCommands []string      `json:"next_commands"`
}

type ActionTarget struct {
	SID      string
	Selector string
	Ref      string
}

type FillOptions struct {
	Target ActionTarget
	Text   string
}

type PressOptions struct {
	Key string
}

type WaitOptions struct {
	Target ActionTarget
	Text   string
	URL    string
	Load   string
}

type ScrollOptions struct {
	Direction string
	Pixels    int
}

type GetOptions struct {
	Kind   string
	Target ActionTarget
}

func NewService(driver cdp.Driver) *Service {
	return &Service{driver: driver}
}

func NewServiceWithRefs(driver cdp.Driver, resolver RefResolver) *Service {
	return &Service{driver: driver, resolver: resolver}
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

func (s *Service) Snapshot(ctx context.Context, options SnapshotOptions) (SnapshotResult, error) {
	state, err := s.driver.Snapshot(ctx)
	if err != nil {
		return SnapshotResult{}, err
	}

	elements := assignRefs(state.Elements)
	if s.resolver != nil {
		if err := s.resolver.Save(snapshot.Record{
			SID:       options.SID,
			URL:       state.URL,
			Title:     state.Title,
			CreatedAt: time.Now().UTC(),
			Elements:  snapshotElements(elements),
		}); err != nil {
			return SnapshotResult{}, err
		}
	}

	return SnapshotResult{
		OK:           true,
		SID:          options.SID,
		URL:          state.URL,
		Title:        state.Title,
		Elements:     elements,
		NextCommands: nextCommands(options.SID, elements),
	}, nil
}

func (s *Service) Click(ctx context.Context, selector string) error {
	return s.driver.Click(ctx, selector)
}

func (s *Service) ClickTarget(ctx context.Context, target ActionTarget) error {
	selector, err := s.resolveTarget(target)
	if err != nil {
		return err
	}
	return s.driver.Click(ctx, selector)
}

func (s *Service) ClickForceTarget(ctx context.Context, target ActionTarget) error {
	selector, err := s.resolveTarget(target)
	if err != nil {
		return err
	}
	return s.driver.ClickForce(ctx, selector)
}

func (s *Service) WaitAppearTarget(ctx context.Context, target ActionTarget) error {
	selector, err := s.resolveTarget(target)
	if err != nil {
		return err
	}
	return s.driver.WaitAppear(ctx, selector)
}

func (s *Service) Type(ctx context.Context, selector, text string) error {
	return s.driver.Type(ctx, selector, text)
}

func (s *Service) TypeTarget(ctx context.Context, target ActionTarget, text string) error {
	selector, err := s.resolveTarget(target)
	if err != nil {
		return err
	}
	return s.driver.Type(ctx, selector, text)
}

func (s *Service) Fill(ctx context.Context, options FillOptions) error {
	selector, err := s.resolveTarget(options.Target)
	if err != nil {
		return err
	}
	return s.driver.Fill(ctx, selector, options.Text)
}

func (s *Service) Select(ctx context.Context, target ActionTarget, value string) error {
	selector, err := s.resolveTarget(target)
	if err != nil {
		return err
	}
	return s.driver.Select(ctx, selector, value)
}

func (s *Service) Is(ctx context.Context, target ActionTarget, prop string) (bool, error) {
	selector, err := s.resolveTarget(target)
	if err != nil {
		return false, err
	}
	return s.driver.Is(ctx, selector, prop)
}

func (s *Service) Eval(ctx context.Context, expression string) (string, error) {
	return s.driver.Eval(ctx, expression)
}

func (s *Service) Check(ctx context.Context, target ActionTarget) error {
	selector, err := s.resolveTarget(target)
	if err != nil {
		return err
	}
	return s.driver.Check(ctx, selector)
}

func (s *Service) Uncheck(ctx context.Context, target ActionTarget) error {
	selector, err := s.resolveTarget(target)
	if err != nil {
		return err
	}
	return s.driver.Uncheck(ctx, selector)
}

func (s *Service) Hover(ctx context.Context, target ActionTarget) error {
	selector, err := s.resolveTarget(target)
	if err != nil {
		return err
	}
	return s.driver.Hover(ctx, selector)
}

func (s *Service) Focus(ctx context.Context, target ActionTarget) error {
	selector, err := s.resolveTarget(target)
	if err != nil {
		return err
	}
	return s.driver.Focus(ctx, selector)
}

func (s *Service) Upload(ctx context.Context, target ActionTarget, files []string) error {
	selector, err := s.resolveTarget(target)
	if err != nil {
		return err
	}
	return s.driver.Upload(ctx, selector, files)
}

func (s *Service) DialogAccept(ctx context.Context, promptText string) error {
	return s.driver.DialogAccept(ctx, promptText)
}

func (s *Service) DialogDismiss(ctx context.Context) error {
	return s.driver.DialogDismiss(ctx)
}

func (s *Service) Press(ctx context.Context, options PressOptions) error {
	return s.driver.Press(ctx, options.Key)
}

func (s *Service) Wait(ctx context.Context, options WaitOptions) error {
	selector, err := s.resolveOptionalTarget(options.Target)
	if err != nil {
		return err
	}
	return s.driver.Wait(ctx, cdp.WaitOptions{
		Selector: selector,
		Text:     options.Text,
		URL:      options.URL,
		Load:     options.Load,
	})
}

func (s *Service) Scroll(ctx context.Context, options ScrollOptions) error {
	return s.driver.Scroll(ctx, options.Direction, options.Pixels)
}

func (s *Service) Get(ctx context.Context, options GetOptions) (string, error) {
	selector, err := s.getSelector(options)
	if err != nil {
		return "", err
	}
	return s.driver.Get(ctx, cdp.GetOptions{Kind: options.Kind, Selector: selector})
}

func (s *Service) Screenshot(ctx context.Context, path string) error {
	return s.driver.Screenshot(ctx, path)
}

func (s *Service) getSelector(options GetOptions) (string, error) {
	switch options.Kind {
	case "url", "title":
		return "", nil
	case "text", "html", "value":
		return s.resolveTarget(options.Target)
	default:
		selector, err := s.resolveOptionalTarget(options.Target)
		if err != nil {
			return "", err
		}
		return selector, nil
	}
}

func (s *Service) resolveTarget(target ActionTarget) (string, error) {
	selector, err := s.resolveOptionalTarget(target)
	if err != nil {
		return "", err
	}
	if selector == "" {
		return "", ErrTargetRequired
	}
	return selector, nil
}

func (s *Service) resolveOptionalTarget(target ActionTarget) (string, error) {
	if target.Selector != "" {
		return target.Selector, nil
	}
	if target.Ref == "" {
		return "", nil
	}
	if s.resolver == nil {
		return "", ErrRefNotFound
	}
	element, err := s.resolver.Resolve(target.SID, target.Ref)
	if err != nil {
		return "", err
	}
	return element.Selector, nil
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

func assignRefs(elements []cdp.Element) []cdp.Element {
	out := make([]cdp.Element, 0, len(elements))
	elementCount := 0
	inputCount := 0
	for _, element := range elements {
		if isInputKind(element.Kind) {
			inputCount++
			element.Ref = "@i" + intString(inputCount)
		} else {
			elementCount++
			element.Ref = "@e" + intString(elementCount)
		}
		out = append(out, element)
	}
	return out
}

func isInputKind(kind string) bool {
	switch strings.ToLower(kind) {
	case "input", "textarea", "select":
		return true
	default:
		return false
	}
}

func snapshotElements(elements []cdp.Element) []snapshot.Element {
	out := make([]snapshot.Element, 0, len(elements))
	for _, element := range elements {
		out = append(out, snapshot.Element{
			Ref:      element.Ref,
			Kind:     element.Kind,
			Text:     element.Text,
			Selector: element.Selector,
			Href:     element.Href,
			Type:     element.Type,
			Name:     element.Name,
			Visible:  element.Visible,
			Enabled:  element.Enabled,
		})
	}
	return out
}

func nextCommands(sid string, elements []cdp.Element) []string {
	commands := []string{
		"aget page get -s " + sid + " url",
		"aget page get -s " + sid + " title",
	}
	for _, element := range elements {
		switch {
		case element.Kind == "select":
			commands = append(commands, "aget page select -s "+sid+" --ref "+element.Ref+" --value VALUE")
		case element.Kind == "input" && (element.Type == "checkbox" || element.Type == "radio"):
			commands = append(commands, "aget page check -s "+sid+" --ref "+element.Ref)
		case isInputKind(element.Kind):
			commands = append(commands, "aget page fill -s "+sid+" --ref "+element.Ref+" --text TEXT")
		default:
			commands = append(commands, "aget page click -s "+sid+" --ref "+element.Ref)
		}
	}
	return commands
}

func intString(value int) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = digits[value%10]
		value /= 10
	}
	return string(buf[i:])
}
