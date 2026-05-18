package cli

import (
	"errors"

	"github.com/izzzzzi/agent-aget/internal/response"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "aget",
		Short:         "Browser workflow helper for LLM agents",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return writeInvalidArgs(cmd, "unknown command "+args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeInvalidArgs(cmd, "command required")
		},
	}
	cmd.PersistentFlags().Bool("json", true, "emit JSON output")
	_ = cmd.PersistentFlags().MarkHidden("json")
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return writeInvalidArgs(cmd, err.Error())
	})
	cmd.AddCommand(newVersionCommand())
	return cmd
}

func Execute() error {
	return NewRootCommand().Execute()
}

func noPositionalArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return writeInvalidArgs(cmd, "unexpected positional arguments")
	}
	return nil
}

func writeInvalidArgs(cmd *cobra.Command, message string) error {
	return writeError(cmd, "invalid_args", message, map[string]any{"hint": "run aget --help"})
}

func writeJSON(cmd *cobra.Command, v any) error {
	body, err := response.Marshal(v)
	if err != nil {
		return err
	}
	_, _ = cmd.OutOrStdout().Write(body)
	return nil
}

func writeError(cmd *cobra.Command, code, message string, details map[string]any) error {
	body, marshalErr := response.MarshalError(code, message, details)
	if marshalErr != nil {
		return marshalErr
	}
	_, _ = cmd.ErrOrStderr().Write(body)
	return errors.New(message)
}
