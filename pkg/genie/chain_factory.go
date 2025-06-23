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
	
	// No longer using separate clarity analysis prompt - decision step handles everything
	
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
	
	// Create main clarification chain with single decision logic
	chain := &ai.Chain{
		Name: "genie-chat-with-clarification",
		Steps: []interface{}{
			ai.DecisionStep{
				Name:    "clarity_decision", 
				Context: `Analyze the user's question: "{{.message}}"

Questions about project understanding are CLEAR:
- "What is this project about?"
- "Explain the architecture" 
- "How does X work?"
- "Show me the code for Y"

Only choose UNCLEAR for truly vague requests:
- "Fix this" (no context)
- "Make it better" (no goal)
- "Help me" (no specifics)`,
				Options: map[string]*ai.Chain{
					"UNCLEAR": clarifyChain,
					"CLEAR":   proceedChain,
					"DEFAULT": proceedChain, // Default to proceeding - most questions are fine
				},
				SaveAs: "decision_path",
			},
		},
	}
	
	return chain, nil
}