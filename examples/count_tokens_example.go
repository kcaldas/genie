package main

import (
	"context"
	"fmt"
	"log"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/llm/genai"
)

func main() {
	eventBus := events.NewEventBus()
	// Create a new genai client
	client, err := genai.NewClient(eventBus)
	if err != nil {
		log.Fatalf("Failed to create genai client: %v", err)
	}

	// Create a prompt
	prompt := ai.Prompt{
		Text:      "The quick brown fox jumps over the lazy dog.",
		ModelName: "gemini-2.0-flash",
	}

	// Count tokens
	ctx := context.Background()
	tokenCount, err := client.CountTokens(ctx, prompt, false)
	if err != nil {
		log.Fatalf("Failed to count tokens: %v", err)
	}

	fmt.Printf("Token count for prompt: '%s'\n", prompt.Text)
	fmt.Printf("Total tokens: %d\n", tokenCount.TotalTokens)
	fmt.Printf("Input tokens: %d\n", tokenCount.InputTokens)
	fmt.Printf("Output tokens: %d\n", tokenCount.OutputTokens)

	// Example with system instruction
	promptWithInstruction := ai.Prompt{
		Instruction: "You are a helpful assistant that answers questions concisely.",
		Text:        "What is the capital of France?",
		ModelName:   "gemini-2.0-flash",
	}

	tokenCount2, err := client.CountTokens(ctx, promptWithInstruction, false)
	if err != nil {
		log.Fatalf("Failed to count tokens with instruction: %v", err)
	}

	fmt.Printf("\nToken count for prompt with instruction:\n")
	fmt.Printf("Instruction: '%s'\n", promptWithInstruction.Instruction)
	fmt.Printf("Text: '%s'\n", promptWithInstruction.Text)
	fmt.Printf("Total tokens: %d\n", tokenCount2.TotalTokens)
}

