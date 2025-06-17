package main

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/internal/di"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/spf13/cobra"
)

func NewAskCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ask",
		Short: "Ask the AI a question",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			promptExecutor, err := di.InitializePromptExecutor()
			if err != nil {
				return err
			}
			return runAskCommand(cmd, args, promptExecutor)
		},
	}
}

func NewAskCommandWithLLM(llmClient ai.Gen) *cobra.Command {
	return &cobra.Command{
		Use:   "ask",
		Short: "Ask the AI a question",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create a PromptExecutor with the provided LLM client for testing
			promptExecutor := ai.NewDefaultPromptExecutor(llmClient, "prompts")
			return runAskCommand(cmd, args, promptExecutor)
		},
	}
}

func runAskCommand(cmd *cobra.Command, args []string, promptExecutor ai.PromptExecutor) error {
	message := strings.Join(args, " ")

	// Use the same prompt as the REPL for consistency
	response, err := promptExecutor.Execute("conversation", false, ai.Attr{
		Key:   "message",
		Value: message,
	}, ai.Attr{
		Key:   "context",
		Value: "", // No conversation context for standalone ask command
	})
	if err != nil {
		return fmt.Errorf("failed to generate response: %w", err)
	}

	cmd.Println(response)
	return nil
}
