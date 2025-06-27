package genie

import (
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/prompts"
)

// SimpleChainFactory creates a basic conversation chain without decision logic
// This is useful for testing and simpler deployments
type SimpleChainFactory struct {
	promptLoader prompts.Loader
}

// NewSimpleChainFactory creates a new simple chain factory
func NewSimpleChainFactory(promptLoader prompts.Loader) ChainFactory {
	return &SimpleChainFactory{
		promptLoader: promptLoader,
	}
}

// CreateChatChain creates a simple conversation chain without clarification logic
func (f *SimpleChainFactory) CreateChatChain() (*ai.Chain, error) {
	// Load conversation prompt
	conversationPrompt, err := f.promptLoader.LoadPrompt("conversation")
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation prompt: %w", err)
	}

	// Create simple conversation chain
	chain := &ai.Chain{
		Name: "genie-simple-chat",
		Steps: []interface{}{
			ai.ChainStep{
				Name:      "conversation",
				Prompt:    &conversationPrompt,
				// ResponseHandler: "file_generator", // Commented out - using writeFile tool instead
				ForwardAs: "response",
			},
		},
	}

	return chain, nil
}

