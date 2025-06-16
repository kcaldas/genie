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
			llmClient, err := di.InitializeGen()
			if err != nil {
				return err
			}
			return runAskCommand(cmd, args, llmClient)
		},
	}
}

func NewAskCommandWithLLM(llmClient ai.Gen) *cobra.Command {
	return &cobra.Command{
		Use:   "ask",
		Short: "Ask the AI a question",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAskCommand(cmd, args, llmClient)
		},
	}
}

func runAskCommand(cmd *cobra.Command, args []string, llmClient ai.Gen) error {
	prompt := strings.Join(args, " ")
	
	// Create a simple prompt for the LLM
	aiPrompt := ai.Prompt{
		Name:        "ask-command",
		Text:        prompt,
		Instruction: "You are a helpful AI assistant. Answer the user's question directly and concisely.",
		ModelName:   "gemini-1.5-flash",
		MaxTokens:   1000,
		Temperature: 0.7,
		TopP:        0.9,
	}
	
	response, err := llmClient.GenerateContent(aiPrompt, false)
	if err != nil {
		return fmt.Errorf("failed to generate response: %w", err)
	}
	
	cmd.Println(response)
	return nil
}