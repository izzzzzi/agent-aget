package cli

import (
	"github.com/izzzzzi/agent-aget/internal/agenthelp"
	"github.com/spf13/cobra"
)

func newPromptCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Print LLM agent instructions",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeJSON(cmd, agenthelp.Prompt())
		},
	}
	configureAgentHelp(cmd)
	return cmd
}

func newAgentInstructionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent-instructions",
		Short: "Alias for prompt",
		Args:  noPositionalArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeJSON(cmd, agenthelp.Prompt())
		},
	}
	configureAgentHelp(cmd)
	return cmd
}
