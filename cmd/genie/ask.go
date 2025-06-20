package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/internal/di"
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/prompts"
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
			// Create a PromptExecutor with embedded prompts for testing
			// Use a no-op publisher for the ask command (events don't need to be shown)
			publisher := &events.NoOpPublisher{}
			promptExecutor := prompts.NewExecutor(llmClient, publisher)
			return runAskCommand(cmd, args, promptExecutor)
		},
	}
}

func runAskCommand(cmd *cobra.Command, args []string, promptExecutor prompts.Executor) error {
	message := strings.Join(args, " ")

	// Use the same prompt as the REPL for consistency
	ctx := context.Background()
	response, err := promptExecutor.Execute(ctx, "conversation", false, ai.Attr{
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
