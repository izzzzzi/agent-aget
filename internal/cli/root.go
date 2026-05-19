package cli

import (
	"errors"
	"strconv"

	"github.com/izzzzzi/agent-aget/internal/agenthelp"
	"github.com/izzzzzi/agent-aget/internal/response"
	"github.com/spf13/cobra"
)

const invalidArgsHint = "run `aget --help` for agent command map or `aget prompt` for full agent instructions"

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "aget",
		Short:         "Browser workflow helper for LLM agents",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
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
	configureAgentHelp(cmd)
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return writeInvalidArgs(cmd, err.Error())
	})
	cmd.AddCommand(newVersionCommand(), newSessionCommand(), newOpenCommand(), newPageCommand(), newBrowserCommand(), newPromptCommand(), newAgentInstructionsCommand())
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
	return writeError(cmd, "invalid_args", message, map[string]any{"hint": invalidArgsHint})
}

func writeJSON(cmd *cobra.Command, v any) error {
	body, err := response.Marshal(v)
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write(body)
	return err
}

func writeError(cmd *cobra.Command, code, message string, details map[string]any) error {
	body, marshalErr := response.MarshalError(code, message, details)
	if marshalErr != nil {
		return marshalErr
	}
	if _, writeErr := cmd.ErrOrStderr().Write(body); writeErr != nil {
		return writeErr
	}
	return errors.New(message)
}

func configureAgentHelp(cmd *cobra.Command) {
	cmd.Flags().VarP(&agentHelpFlag{cmd: cmd}, "help", "h", "help for "+cmd.Name())
	cmd.Flags().Lookup("help").NoOptDefVal = "true"
	cmd.SetHelpFunc(func(helpCmd *cobra.Command, args []string) {
		if err := writeAgentHelp(helpCmd); err != nil {
			helpCmd.PrintErr(err)
		}
	})
}

type agentHelpFlag struct {
	cmd     *cobra.Command
	enabled bool
}

func (f *agentHelpFlag) IsBoolFlag() bool {
	return true
}

func (f *agentHelpFlag) Set(value string) error {
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	if !enabled {
		f.enabled = false
		return nil
	}
	if unknown := unknownPreHelpSubcommand(f.cmd); unknown != "" {
		return errors.New("unknown command " + unknown)
	}
	f.enabled = true
	return nil
}

func (f *agentHelpFlag) String() string {
	if f.enabled {
		return "true"
	}
	return "false"
}

func (f *agentHelpFlag) Type() string {
	return "bool"
}

func unknownPreHelpSubcommand(cmd *cobra.Command) string {
	if cmd == nil || !cmd.HasSubCommands() {
		return ""
	}
	for _, arg := range cmd.Flags().Args() {
		if isFlagLike(arg) {
			continue
		}
		for _, child := range cmd.Commands() {
			if child.Name() == arg || child.HasAlias(arg) {
				return ""
			}
		}
		return arg
	}
	return ""
}

func isFlagLike(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

func writeAgentHelp(cmd *cobra.Command) error {
	if cmd == nil || cmd.CommandPath() == "aget" {
		return writeJSON(cmd, agenthelp.RootHelp())
	}
	group := agentHelpGroup(cmd)
	if payload, ok := agenthelp.GroupHelp(group); ok {
		return writeJSON(cmd, payload)
	}
	return writeJSON(cmd, agenthelp.RootHelp())
}

func agentHelpGroup(cmd *cobra.Command) string {
	if cmd == nil || cmd.CommandPath() == "aget" {
		return ""
	}
	current := cmd
	for current.Parent() != nil && current.Parent().Name() != "aget" {
		current = current.Parent()
	}
	if current.Name() == "agent-instructions" {
		return "prompt"
	}
	return current.Name()
}
