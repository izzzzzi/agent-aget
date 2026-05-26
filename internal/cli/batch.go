package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/izzzzzi/agent-aget/internal/page"
	"github.com/izzzzzi/agent-aget/internal/snapshot"
	"github.com/spf13/cobra"
)

type batchStep struct {
	Cmd       string `json:"cmd"`
	Selector  string `json:"selector,omitempty"`
	Ref       string `json:"ref,omitempty"`
	Text      string `json:"text,omitempty"`
	Key       string `json:"key,omitempty"`
	Direction string `json:"direction,omitempty"`
	Pixels    int    `json:"pixels,omitempty"`
	Kind      string `json:"kind,omitempty"`
	URL       string `json:"url,omitempty"`
	Load      string `json:"load,omitempty"`
}

type batchError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type batchResponse struct {
	OK          bool             `json:"ok"`
	SID         string           `json:"sid"`
	Results     []map[string]any `json:"results"`
	FailedIndex *int             `json:"failed_index,omitempty"`
	Error       *batchError      `json:"error,omitempty"`
}

func newBatchCommand() *cobra.Command {
	var sid string
	var fromStdin bool
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Run page commands from a JSON stdin batch",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !fromStdin {
				return writeInvalidArgs(cmd, "stdin flag required")
			}
			record, err := lookupSession(cmd, sid)
			if err != nil {
				return err
			}
			steps, err := readBatchSteps(cmd.InOrStdin())
			if err != nil {
				return writeBatchFailure(cmd, batchResponse{
					OK:      false,
					SID:     sid,
					Results: []map[string]any{},
					Error:   &batchError{Code: "invalid_json", Message: err.Error()},
				})
			}

			ctx, cancel := pageOperationContext()
			defer cancel()
			svc, err := pageServiceForRecord(ctx, record, true)
			if err != nil {
				return writeBatchFailure(cmd, batchResponse{
					OK:      false,
					SID:     sid,
					Results: []map[string]any{},
					Error:   &batchError{Code: "page_connect_failed", Message: err.Error()},
				})
			}

			results := make([]map[string]any, 0, len(steps))
			for index, step := range steps {
				result, stepErr := runBatchStep(ctx, svc, sid, step)
				if stepErr != nil {
					code := batchErrorCode(step, stepErr)
					return writeBatchFailure(cmd, batchResponse{
						OK:          false,
						SID:         sid,
						Results:     results,
						FailedIndex: &index,
						Error:       &batchError{Code: code, Message: stepErr.Error()},
					})
				}
				results = append(results, result)
			}
			return writeJSON(cmd, batchResponse{OK: true, SID: sid, Results: results})
		},
	}
	cmd.Flags().StringVarP(&sid, "sid", "s", "", "session id")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "read JSON batch from stdin")
	configureAgentHelp(cmd)
	return cmd
}

func readBatchSteps(r io.Reader) ([]batchStep, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	var steps []batchStep
	if err := decoder.Decode(&steps); err != nil {
		return nil, err
	}
	if steps == nil {
		steps = []batchStep{}
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errors.New("stdin must contain one JSON array")
	}
	return steps, nil
}

func runBatchStep(ctx context.Context, svc *page.Service, sid string, step batchStep) (map[string]any, error) {
	switch step.Cmd {
	case "snapshot":
		result, err := svc.Snapshot(ctx, page.SnapshotOptions{SID: sid})
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"cmd":           "snapshot",
			"ok":            true,
			"sid":           result.SID,
			"url":           result.URL,
			"title":         result.Title,
			"elements":      result.Elements,
			"next_commands": result.NextCommands,
		}, nil
	case "click":
		if err := validateBatchTarget(step); err != nil {
			return nil, err
		}
		if err := svc.ClickTarget(ctx, batchTarget(sid, step)); err != nil {
			return nil, err
		}
		return batchTargetResult("click", step), nil
	case "fill":
		if err := validateBatchTarget(step); err != nil {
			return nil, err
		}
		if err := svc.Fill(ctx, page.FillOptions{Target: batchTarget(sid, step), Text: step.Text}); err != nil {
			return nil, err
		}
		result := batchTargetResult("fill", step)
		result["text_len"] = len(step.Text)
		return result, nil
	case "press":
		if step.Key == "" {
			return nil, errors.New("key required")
		}
		if err := svc.Press(ctx, page.PressOptions{Key: step.Key}); err != nil {
			return nil, err
		}
		return map[string]any{"cmd": "press", "ok": true, "key": step.Key}, nil
	case "wait":
		if countNonEmpty(step.Selector, step.Text, step.URL, step.Load) != 1 {
			return nil, errors.New("exactly one wait condition required")
		}
		if step.Ref != "" {
			return nil, errors.New("ref is not supported for wait")
		}
		if err := svc.Wait(ctx, page.WaitOptions{Target: page.ActionTarget{Selector: step.Selector}, Text: step.Text, URL: step.URL, Load: step.Load}); err != nil {
			return nil, err
		}
		return map[string]any{"cmd": "wait", "ok": true}, nil
	case "scroll":
		if step.Direction == "" {
			return nil, errors.New("direction required")
		}
		pixels := step.Pixels
		if pixels == 0 {
			pixels = 800
		}
		if err := svc.Scroll(ctx, page.ScrollOptions{Direction: step.Direction, Pixels: pixels}); err != nil {
			return nil, err
		}
		return map[string]any{"cmd": "scroll", "ok": true, "direction": step.Direction, "pixels": pixels}, nil
	case "get":
		if !validGetKind(step.Kind) {
			return nil, fmt.Errorf("unsupported get kind %s", step.Kind)
		}
		needsTarget := step.Kind == "text" || step.Kind == "html" || step.Kind == "value"
		if needsTarget {
			if err := validateBatchTarget(step); err != nil {
				return nil, err
			}
		} else if step.Selector != "" || step.Ref != "" {
			return nil, errors.New("selector/ref only valid for text, html, or value")
		}
		value, err := svc.Get(ctx, page.GetOptions{Kind: step.Kind, Target: batchTarget(sid, step)})
		if err != nil {
			return nil, err
		}
		return map[string]any{"cmd": "get", "ok": true, "kind": step.Kind, "value": value}, nil
	default:
		if step.Cmd == "" {
			return nil, errors.New("cmd required")
		}
		return nil, fmt.Errorf("unsupported command %s", step.Cmd)
	}
}

func validateBatchTarget(step batchStep) error {
	if step.Selector == "" && step.Ref == "" {
		return errors.New("selector or ref required")
	}
	if step.Selector != "" && step.Ref != "" {
		return errors.New("selector and ref are mutually exclusive")
	}
	return nil
}

func batchTarget(sid string, step batchStep) page.ActionTarget {
	return page.ActionTarget{SID: sid, Selector: step.Selector, Ref: step.Ref}
}

func batchTargetResult(command string, step batchStep) map[string]any {
	result := map[string]any{"cmd": command, "ok": true}
	if step.Selector != "" {
		result["selector"] = step.Selector
	}
	if step.Ref != "" {
		result["ref"] = step.Ref
	}
	return result
}

func batchErrorCode(step batchStep, err error) string {
	if errors.Is(err, snapshot.ErrRefNotFound) || errors.Is(err, snapshot.ErrNotFound) || errors.Is(err, page.ErrRefNotFound) {
		return "ref_not_found"
	}
	if step.Cmd == "wait" {
		switch err.Error() {
		case "exactly one wait condition required", "ref is not supported for wait":
			return "invalid_args"
		default:
			return "page_wait_timeout"
		}
	}
	switch step.Cmd {
	case "", "click", "fill", "press", "scroll", "get", "snapshot":
		if isBatchValidationError(err) {
			return "invalid_args"
		}
		return "page_action_failed"
	default:
		return "invalid_args"
	}
}

func isBatchValidationError(err error) bool {
	switch err.Error() {
	case "selector or ref required",
		"selector and ref are mutually exclusive",
		"key required",
		"direction required",
		"cmd required",
		"selector/ref only valid for text, html, or value":
		return true
	default:
		return false
	}
}

func writeBatchFailure(cmd *cobra.Command, response batchResponse) error {
	if response.Results == nil {
		response.Results = []map[string]any{}
	}
	if err := writeJSON(cmd, response); err != nil {
		return err
	}
	if response.Error == nil {
		return errors.New("batch failed")
	}
	return errors.New(response.Error.Message)
}
