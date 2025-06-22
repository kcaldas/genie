package genie

import (
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/prompts"
)

// SimpleChainFactory creates a basic conversation chain without decision logic
// This is useful for testing and simpler deployments
type SimpleChainFactory struct{}

// NewSimpleChainFactory creates a new simple chain factory
func NewSimpleChainFactory() ChainFactory {
	return &SimpleChainFactory{}
}

// CreateChatChain creates a simple conversation chain without clarification logic
func (f *SimpleChainFactory) CreateChatChain(promptLoader prompts.Loader) (*ai.Chain, error) {
	// Load conversation prompt
	conversationPrompt, err := promptLoader.LoadPrompt("conversation")
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
				ForwardAs: "response",
			},
		},
	}
	
	return chain, nil
}