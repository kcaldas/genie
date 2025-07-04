// Package chain_assembly contains examples of complex chain structures
// for reference and learning purposes.
//
// This file demonstrates how to build sophisticated decision-based chains
// that can route user requests to different specialized sub-chains based
// on classification logic.
package chain_assembly

import (
	"context"
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/prompts"
)

// ExampleComplexChainFactory demonstrates building a decision-based chain
// that classifies user requests and routes them to specialized sub-chains.
// 
// This example shows:
// - Decision steps with multiple routing options
// - Sub-chains with different purposes (chat, exploration, action, exit)
// - How to load and use multiple prompts in a single chain
// - Fallback handling for unmatched requests
//
// NOTE: This is an EXAMPLE only - it does not implement any required interfaces
// and is provided purely for reference when building complex chains.
type ExampleComplexChainFactory struct {
	eventBus     events.EventBus
	promptLoader prompts.Loader
}

// NewExampleComplexChainFactory creates a new example chain factory
func NewExampleComplexChainFactory(eventBus events.EventBus, promptLoader prompts.Loader) *ExampleComplexChainFactory {
	return &ExampleComplexChainFactory{
		eventBus:     eventBus,
		promptLoader: promptLoader,
	}
}

// CreateDecisionBasedChain demonstrates how to build a complex decision-based chain
// that routes user requests to different specialized sub-chains.
//
// Chain Structure:
// 1. Decision Step: Classifies the user's request type
// 2. Multiple Sub-chains: Each handling a specific type of request
//    - Chat: Simple conversations and questions
//    - Explore: Code exploration and understanding
//    - Action: File modification and system commands
//    - Exit: Conversation termination
// 3. Fallback: Default handling for unclassified requests
func (f *ExampleComplexChainFactory) CreateDecisionBasedChain(ctx context.Context) (*ai.Chain, error) {
	// Load prompts for each specialized sub-chain
	// NOTE: In this example, we reference prompts that don't exist as files.
	// This is intentional - this example demonstrates the pattern but is not meant to be functional.
	// In a real implementation, you would either:
	// 1. Load from persona prompt files using LoadPromptFromFile()
	// 2. Create the prompts programmatically 
	// 3. Load from your own embedded prompts
	chatPrompt, err := f.promptLoader.LoadPromptFromFile("/path/to/example-chat.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load example-chat prompt: %w", err)
	}

	explorePrompt, err := f.promptLoader.LoadPromptFromFile("/path/to/example-explore.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load example-explore prompt: %w", err)
	}

	actionPrompt, err := f.promptLoader.LoadPromptFromFile("/path/to/example-action.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load example-action prompt: %w", err)
	}

	exitPrompt, err := f.promptLoader.LoadPromptFromFile("/path/to/example-exit.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load example-exit prompt: %w", err)
	}

	// Create the main decision step that classifies user requests
	classifyRequest := ai.DecisionStep{
		Name:    "classify_request",
		Context: `The user is working at the root level of a project. They may be asking for help with code, documentation, or general questions.`,
	}

	// Sub-chain 1: Handle simple conversations and general questions
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

	// Sub-chain 2: Handle code exploration and understanding requests
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

	// Sub-chain 3: Handle action-oriented requests (file modifications, commands)
	actionChain := &ai.Chain{
		Name:        "TakeAction",
		Description: "Create, modify, execute files or run commands to make changes. Examples: 'create a new file', 'fix the bug in main.go', 'run the tests', 'add a feature', 'refactor this code', 'build the project'",
		Steps: []any{
			ai.ChainStep{
				Name:            "take_action",
				Prompt:          &actionPrompt,
				ResponseHandler: "file_generator", // Example of using response handlers
				ForwardAs:       "response",
			},
		},
	}

	// Sub-chain 4: Handle conversation termination
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

	// Wire up the decision step with all the sub-chains
	// The LLM will be given these options and choose the most appropriate one
	classifyRequest.AddOption("CHAT", chatChain)
	classifyRequest.AddOption("EXPLORE", exploreChain)
	classifyRequest.AddOption("ACTION", actionChain)
	classifyRequest.AddOption("EXIT", exitChain)
	classifyRequest.AddOption("DEFAULT", chatChain) // Fallback for unmatched requests

	// Create the main chain that starts with the decision step
	mainChain := &ai.Chain{
		Name: "ComplexDecisionBasedChain",
		Steps: []any{
			classifyRequest, // Single decision step that routes to sub-chains
		},
	}

	return mainChain, nil
}

// Key Learning Points from this Example:
//
// 1. **Decision Steps**: Use ai.DecisionStep to create branching logic
//    - Give it a name and context to help the LLM make decisions
//    - Add multiple options with AddOption(key, subChain)
//    - Always include a DEFAULT fallback option
//
// 2. **Sub-chains**: Each routing option is its own complete chain
//    - Can have different prompts, steps, and response handlers
//    - Keep descriptions clear to help with classification
//    - Can be as simple or complex as needed
//
// 3. **Prompt Management**: Complex chains often need multiple prompts
//    - Load each prompt separately with descriptive names
//    - Handle loading errors gracefully
//    - Consider prompt caching for performance
//
// 4. **Response Handlers**: Different sub-chains can use different handlers
//    - "file_generator" for actions that modify files
//    - Custom handlers for specialized processing
//    - Leave empty for simple text responses
//
// 5. **Chain Composition**: Chains can be nested arbitrarily deep
//    - Sub-chains can themselves contain decision steps
//    - Balance complexity with maintainability
//    - Consider performance implications of deep nesting
//
// 6. **Error Handling**: Always handle prompt loading and chain creation errors
//    - Provide meaningful error messages
//    - Consider fallback chains for error cases
//    - Log important events for debugging