package persona

import (
	"context"
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/prompts"
)

// GeneralistChainFactory creates a simple single-prompt chain with all tools available
// This is designed for better performance by avoiding multiple chain steps
type GeneralistChainFactory struct {
	eventBus     events.EventBus
	promptLoader prompts.Loader
}

// NewGeneralistChainFactory creates a new generalist chain factory
func NewGeneralistChainFactory(eventBus events.EventBus, promptLoader prompts.Loader) ChainFactory {
	return &GeneralistChainFactory{
		eventBus:     eventBus,
		promptLoader: promptLoader,
	}
}

// CreateChain creates a decision-based generalist chain
func (f *GeneralistChainFactory) CreateChain(ctx context.Context) (*ai.Chain, error) {
	// Load prompts for each stage
	chatPrompt, err := f.promptLoader.LoadPrompt("generalist-chat")
	if err != nil {
		return nil, fmt.Errorf("failed to load generalist-chat prompt: %w", err)
	}

	explorePrompt, err := f.promptLoader.LoadPrompt("generalist-explore")
	if err != nil {
		return nil, fmt.Errorf("failed to load generalist-explore prompt: %w", err)
	}

	actionPrompt, err := f.promptLoader.LoadPrompt("generalist-action")
	if err != nil {
		return nil, fmt.Errorf("failed to load generalist-action prompt: %w", err)
	}

	exitPrompt, err := f.promptLoader.LoadPrompt("generalist-exit")
	if err != nil {
		return nil, fmt.Errorf("failed to load generalist-exit prompt: %w", err)
	}

	classifyRequest := ai.DecisionStep{
		Name:    "classify_request",
		Context: `The user is working at the root level of a project. The may be asking for help with code, documentation, or general questions.`,
	}

	// Create individual chains for each decision path (no recursive decisions)
	chatChain := &ai.Chain{
		Name:        "ChatResponse",
		Description: "Simple conversation, greetings, thanks, or general questions that don't require code access. Examples: 'hi', 'hello', 'thanks', 'how are you', 'what can you do', 'explain what X is'",
		Steps: []any{
			ai.ChainStep{
				Name:      "chat_response",
				Prompt:    &chatPrompt,
				ForwardAs: "response",
			},
		},
	}

	exploreChain := &ai.Chain{
		Name:        "ExploreCode",
		Description: "Explore existing code, project structure, or understand what's already there. Examples: 'what is this project about', 'show me the main file', 'read the README', 'how does authentication work', 'find all the tests'",
		Steps: []any{
			ai.ChainStep{
				Name:      "explore_code",
				Prompt:    &explorePrompt,
				ForwardAs: "response",
			},
		},
	}

	actionChain := &ai.Chain{
		Name:        "TakeAction",
		Description: "Create, modify, execute files or run commands to make changes. Examples: 'create a new file', 'fix the bug in main.go', 'run the tests', 'add a feature', 'refactor this code', 'build the project'",
		Steps: []any{
			ai.ChainStep{
				Name:            "take_action",
				Prompt:          &actionPrompt,
				ResponseHandler: "file_generator",
				ForwardAs:       "response",
			},
		},
	}

	// Create exit chain with wrap-up response
	exitChain := &ai.Chain{
		Name:        "Exit",
		Description: "End the conversation with a friendly goodbye. Examples: 'bye', 'goodbye', 'that's all', 'exit', 'thanks, I'm done'",
		Steps: []any{
			ai.ChainStep{
				Name:      "exit_response",
				Prompt:    &exitPrompt,
				ForwardAs: "response",
			},
		},
	}

	classifyRequest.AddOption("CHAT", chatChain)
	classifyRequest.AddOption("EXPLORE", exploreChain)
	classifyRequest.AddOption("ACTION", actionChain)
	classifyRequest.AddOption("EXIT", exitChain)
	classifyRequest.AddOption("DEFAULT", chatChain) // Fallback to chat if no match

	// Create main decision-based chain
	chain := &ai.Chain{
		Name: "GeneralistChat",
		Steps: []any{
			classifyRequest,
		},
	}

	return chain, nil
}