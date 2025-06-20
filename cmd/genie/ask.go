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
			gen, err := di.InitializeGen()
			if err != nil {
				return err
			}
			promptLoader, err := di.InitializePromptLoader()
			if err != nil {
				return err
			}
			return runAskCommand(cmd, args, gen, promptLoader)
		},
	}
}

func NewAskCommandWithLLM(llmClient ai.Gen) *cobra.Command {
	return &cobra.Command{
		Use:   "ask",
		Short: "Ask the AI a question",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use a no-op publisher for the ask command (events don't need to be shown)
			publisher := &events.NoOpPublisher{}
			promptLoader := prompts.NewPromptLoader(publisher)
			return runAskCommand(cmd, args, llmClient, promptLoader)
		},
	}
}

func runAskCommand(cmd *cobra.Command, args []string, gen ai.Gen, promptLoader prompts.Loader) error {
	message := strings.Join(args, " ")

	// Load the conversation prompt (already enhanced with tools)
	conversationPrompt, err := promptLoader.LoadPrompt("conversation")
	if err != nil {
		return fmt.Errorf("failed to load prompt: %w", err)
	}
	
	// Create chain with loaded prompt
	chain := &ai.Chain{
		Name: "ask-chain",
		Steps: []ai.ChainStep{
			{
				Name:      "conversation",
				Prompt:    &conversationPrompt,
				ForwardAs: "response",
			},
		},
	}
	
	// Execute chain using Gen directly
	ctx := context.Background()
	chainCtx := ai.NewChainContext(map[string]string{
		"context": "", // No conversation context for standalone ask command
		"message": message,
	})
	
	err = chain.Run(ctx, gen, chainCtx, false)
	if err != nil {
		return fmt.Errorf("failed to generate response: %w", err)
	}
	
	// Extract and print response
	response := chainCtx.Data["response"]
	cmd.Println(response)
	return nil
}
