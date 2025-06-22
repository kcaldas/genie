package genie

import (
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/prompts"
)

// DefaultChainFactory creates the default clarification-based conversation chain
type DefaultChainFactory struct{}

// NewDefaultChainFactory creates a new default chain factory
func NewDefaultChainFactory() ChainFactory {
	return &DefaultChainFactory{}
}

// CreateChatChain creates the default clarification chain for production use
func (f *DefaultChainFactory) CreateChatChain(promptLoader prompts.Loader) (*ai.Chain, error) {
	// Load prompts for clarification chain
	conversationPrompt, err := promptLoader.LoadPrompt("conversation")
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation prompt: %w", err)
	}
	
	clarityPrompt, err := promptLoader.LoadPrompt("clarity_analysis")
	if err != nil {
		return nil, fmt.Errorf("failed to load clarity analysis prompt: %w", err)
	}
	
	clarifyingPrompt, err := promptLoader.LoadPrompt("clarifying_questions")
	if err != nil {
		return nil, fmt.Errorf("failed to load clarifying questions prompt: %w", err)
	}
	
	// Create sub-chains for decision paths
	clarifyChain := &ai.Chain{
		Name: "clarify-request",
		Steps: []interface{}{
			ai.ChainStep{
				Name:      "ask_clarifying_questions",
				Prompt:    &clarifyingPrompt,
				ForwardAs: "response",
			},
		},
	}
	
	proceedChain := &ai.Chain{
		Name: "proceed-with-conversation",
		Steps: []interface{}{
			ai.ChainStep{
				Name:      "conversation",
				Prompt:    &conversationPrompt,
				ForwardAs: "response",
			},
		},
	}
	
	// Create main clarification chain with decision logic
	chain := &ai.Chain{
		Name: "genie-chat-with-clarification",
		Steps: []interface{}{
			ai.ChainStep{
				Name:      "analyze_clarity",
				Prompt:    &clarityPrompt,
				ForwardAs: "clarity_analysis",
			},
			ai.DecisionStep{
				Name:    "clarity_decision",
				Context: "Based on the clarity analysis of the user's message",
				Options: map[string]*ai.Chain{
					"UNCLEAR": clarifyChain,
					"CLEAR":   proceedChain,
				},
				SaveAs: "decision_path",
			},
		},
	}
	
	return chain, nil
}