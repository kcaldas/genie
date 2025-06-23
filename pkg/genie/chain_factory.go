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

// CreateChatChain creates the conversation and action chain for production use
func (f *DefaultChainFactory) CreateChatChain(promptLoader prompts.Loader) (*ai.Chain, error) {
	// Load all required prompts
	conversationPrompt, err := promptLoader.LoadPrompt("conversation")
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation prompt: %w", err)
	}
	
	clarifyingPrompt, err := promptLoader.LoadPrompt("clarifying_questions")
	if err != nil {
		return nil, fmt.Errorf("failed to load clarifying questions prompt: %w", err)
	}
	
	planPrompt, err := promptLoader.LoadPrompt("plan_implementation")
	if err != nil {
		return nil, fmt.Errorf("failed to load plan_implementation prompt: %w", err)
	}
	
	executePrompt, err := promptLoader.LoadPrompt("execute_changes")
	if err != nil {
		return nil, fmt.Errorf("failed to load execute_changes prompt: %w", err)
	}
	
	verifyPrompt, err := promptLoader.LoadPrompt("verify_results")
	if err != nil {
		return nil, fmt.Errorf("failed to load verify_results prompt: %w", err)
	}
	
	// Create clarification chain
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
	
	// Create implementation chain (for when action is needed)
	implementationChain := &ai.Chain{
		Name: "implementation",
		Steps: []interface{}{
			ai.ChainStep{
				Name:      "plan_implementation",
				Prompt:    &planPrompt,
				ForwardAs: "implementation_plan",
			},
			ai.ChainStep{
				Name:            "execute_changes",
				Prompt:          &executePrompt,
				ResponseHandler: "file_generator", // Use response handler for file creation
				ForwardAs:       "execution_summary",
			},
			ai.ChainStep{
				Name:      "verify_results",
				Prompt:    &verifyPrompt,
				ForwardAs: "verification_results",
			},
			ai.DecisionStep{
				Name:    "completion_decision",
				Context: `Based on the verification results: "{{.verification_results}}"

Does the implementation need more work or is it complete?

Choose CONTINUE if:
- There were errors that need fixing
- The implementation is incomplete
- More features need to be added

Choose DONE if:
- Everything works correctly
- The user's request is fully satisfied
- No further changes are needed`,
				Options: map[string]*ai.Chain{
					"CONTINUE": &ai.Chain{
						Name: "continue-implementation",
						Steps: []interface{}{
							ai.ChainStep{
								Name:            "execute_more_changes",
								Prompt:          &executePrompt,
								ResponseHandler: "file_generator", // Use response handler for file creation
								ForwardAs:       "execution_summary",
							},
							ai.ChainStep{
								Name:      "verify_again",
								Prompt:    &verifyPrompt,
								ForwardAs: "verification_results",
							},
						},
					},
					"DONE": &ai.Chain{
						Name: "complete",
						Steps: []interface{}{
							ai.ChainStep{
								Name: "completion_message",
								Fn: func(data map[string]string, debug bool) (string, error) {
									return "Implementation complete! " + data["verification_results"], nil
								},
								ForwardAs: "response",
							},
						},
					},
					"DEFAULT": &ai.Chain{
						Name: "complete",
						Steps: []interface{}{
							ai.ChainStep{
								Name: "completion_message",
								Fn: func(data map[string]string, debug bool) (string, error) {
									return "Implementation complete! " + data["verification_results"], nil
								},
								ForwardAs: "response",
							},
						},
					},
				},
				SaveAs: "completion_decision",
			},
		},
	}
	
	// Create conversation chain that flows into action decision
	conversationWithActionChain := &ai.Chain{
		Name: "conversation-and-action",
		Steps: []interface{}{
			ai.ChainStep{
				Name:      "conversation",
				Prompt:    &conversationPrompt,
				ForwardAs: "conversation_summary",
			},
			ai.DecisionStep{
				Name:    "action_decision",
				Context: `Based on our conversation and the user's request: "{{.message}}"

Analyze if this requires IMPLEMENTATION or if it's COMPLETE as-is.

Choose IMPLEMENT if the user wants to:
- Create new files or code
- Modify existing files
- Build something new
- Add features or functionality
- Generate or write code

Choose COMPLETE if the request was:
- Asking for information or explanations
- Understanding how things work
- Exploring existing code
- Getting documentation or help`,
				Options: map[string]*ai.Chain{
					"IMPLEMENT": implementationChain,
					"COMPLETE": &ai.Chain{
						Name: "complete-conversation",
						Steps: []interface{}{
							ai.ChainStep{
								Name: "final_response",
								Fn: func(data map[string]string, debug bool) (string, error) {
									return data["conversation_summary"], nil
								},
								ForwardAs: "response",
							},
						},
					},
					"DEFAULT": &ai.Chain{
						Name: "complete-conversation",
						Steps: []interface{}{
							ai.ChainStep{
								Name: "final_response",
								Fn: func(data map[string]string, debug bool) (string, error) {
									return data["conversation_summary"], nil
								},
								ForwardAs: "response",
							},
						},
					},
				},
				SaveAs: "action_decision",
			},
		},
	}
	
	// Create main chain with clarity decision first
	chain := &ai.Chain{
		Name: "genie-chat-with-action",
		Steps: []interface{}{
			ai.DecisionStep{
				Name:    "clarity_decision", 
				Context: `Analyze the user's question: "{{.message}}"

Questions about project understanding are CLEAR:
- "What is this project about?"
- "Explain the architecture" 
- "How does X work?"
- "Show me the code for Y"

Requests for creation/modification are also CLEAR:
- "Create a hello world program"
- "Add a new feature"
- "Write a function to do X"
- "Build a simple app"

Only choose UNCLEAR for truly vague requests:
- "Fix this" (no context)
- "Make it better" (no goal)
- "Help me" (no specifics)`,
				Options: map[string]*ai.Chain{
					"UNCLEAR": clarifyChain,
					"CLEAR":   conversationWithActionChain,
					"DEFAULT": conversationWithActionChain,
				},
				SaveAs: "clarity_decision",
			},
		},
	}
	
	return chain, nil
}

// CreateTestWriteChain creates a simple chain for testing file generation functionality
func (f *DefaultChainFactory) CreateTestWriteChain(promptLoader prompts.Loader) (*ai.Chain, error) {
	// Load the file generation prompt
	generateFilesPrompt, err := promptLoader.LoadPrompt("generate_files")
	if err != nil {
		return nil, fmt.Errorf("failed to load generate_files prompt: %w", err)
	}
	
	// Create a minimal chain that uses the file generation prompt with response handler
	chain := &ai.Chain{
		Name: "file-generation-test",
		Steps: []interface{}{
			ai.ChainStep{
				Name:            "generate_files",
				Prompt:          &generateFilesPrompt,
				ResponseHandler: "file_generator", // Process response through FileGenerationHandler
				ForwardAs:       "response",
			},
		},
	}
	
	return chain, nil
}