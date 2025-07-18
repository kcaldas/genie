package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// SequentialThinkingTool represents the sequential thinking tool.
type SequentialThinkingTool struct {
	publisher events.Publisher
}

// SequentialThinkingParams defines the parameters for the sequentialthinking tool.
type SequentialThinkingParams struct {
	NextThoughtNeeded bool    `json:"nextThoughtNeeded"`
	Thought           string  `json:"thought"`
	ThoughtNumber     int     `json:"thoughtNumber"`
	TotalThoughts     int     `json:"totalThoughts"`
	BranchFromThought *int    `json:"branchFromThought,omitempty"`
	BranchID          *string `json:"branchId,omitempty"`
	IsRevision        *bool   `json:"isRevision,omitempty"`
	NeedsMoreThoughts *bool   `json:"needsMoreThoughts,omitempty"`
	RevisesThought    *int    `json:"revisesThought,omitempty"`
}

// SequentialThinkingResponseContent defines the content structure for the sequentialthinking tool's response.
type SequentialThinkingResponseContent struct {
	Text *string `json:"text,omitempty"`
	Type *string `json:"type,omitempty"`
}

// SequentialThinkingResponse defines the response structure for the sequentialthinking tool.
type SequentialThinkingResponse struct {
	Content []SequentialThinkingResponseContent `json:"content,omitempty"`
}

// NewSequentialThinkingTool creates a new instance of the SequentialThinkingTool.
func NewSequentialThinkingTool(publisher events.Publisher) *SequentialThinkingTool {
	return &SequentialThinkingTool{
		publisher: publisher,
	}
}

// Run executes the sequential thinking process.
func (t *SequentialThinkingTool) Run(params SequentialThinkingParams) (SequentialThinkingResponse, error) {
	// For now, simply echo the thought and thought number
	responseMessage := fmt.Sprintf("Thought %d: %s", params.ThoughtNumber, params.Thought)

	text := responseMessage
	typeStr := "text"

	return SequentialThinkingResponse{
		Content: []SequentialThinkingResponseContent{
			{
				Text: &text,
				Type: &typeStr,
			},
		},
	}, nil
}

// Declaration returns the function declaration for the sequentialthinking tool.
func (t *SequentialThinkingTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "sequentialthinking",
		Description: `A detailed tool for dynamic and reflective problem-solving through thoughts.
This tool helps analyze problems through a flexible thinking process that can adapt and evolve.
Each thought can build on, question, or revise previous insights as understanding deepens.

When to use this tool:
- Breaking down complex problems into steps
- Planning and design with room for revision
- Analysis that might need course correction
- Problems where the full scope might not be clear initially
- Problems that require a multi-step solution
- Tasks that need to maintain context over multiple steps
- Situations where irrelevant information needs to be filtered out

Key features:
- You can adjust total_thoughts up or down as you progress
- You can question or revise previous thoughts
- You can add more thoughts even after reaching what seemed like the end
- You can express uncertainty and explore alternative approaches
- Not every thought needs to build linearly - you can branch or backtrack
- Generates a solution hypothesis
- Verifies the hypothesis based on the Chain of Thought steps
- Repeats the process until satisfied
- Provides a correct answer

Parameters explained:
- thought: Your current thinking step, which can include:
* Regular analytical steps
* Revisions of previous thoughts
* Questions about previous decisions
* Realizations about needing more analysis
* Changes in approach
* Hypothesis generation
* Hypothesis verification
- next_thought_needed: True if you need more thinking, even if at what seemed like the end
- thought_number: Current number in sequence (can go beyond initial total if needed)
- total_thoughts: Current estimate of thoughts needed (can be adjusted up/down)
- is_revision: A boolean indicating if this thought revises previous thinking
- revises_thought: If is_revision is true, which thought number is being reconsidered
- branch_from_thought: If branching, which thought number is the branching point
- branch_id: Identifier for the current branch (if any)
- needs_more_thoughts: If reaching end but realizing more thoughts needed

You should:
1. Start with an initial estimate of needed thoughts, but be ready to adjust
2. Feel free to question or revise previous thoughts
3. Don't hesitate to add more thoughts if needed, even at the \"end\"
4. Express uncertainty when present
5. Mark thoughts that revise previous thinking or branch into new paths
6. Ignore information that is irrelevant to the current step
7. Generate a solution hypothesis when appropriate
8. Verify the hypothesis based on the Chain of Thought steps
9. Repeat the process until satisfied with the solution
10. Provide a single, ideally correct answer as the final output
11. Only set next_thought_needed to false when truly done and a satisfactory answer is reached`,
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"nextThoughtNeeded": {
					Type:        ai.TypeBoolean,
					Description: "Whether another thought step is needed",
				},
				"thought": {
					Type:        ai.TypeString,
					Description: "Your current thinking step",
				},
				"thoughtNumber": {
					Type:        ai.TypeInteger,
					Description: "Current thought number",
				},
				"totalThoughts": {
					Type:        ai.TypeInteger,
					Description: "Estimated total thoughts needed",
				},
				"branchFromThought": {
					Type:        ai.TypeInteger,
					Description: "Branching point thought number",
				},
				"branchId": {
					Type:        ai.TypeString,
					Description: "Branch identifier",
				},
				"isRevision": {
					Type:        ai.TypeBoolean,
					Description: "Whether this revises previous thinking",
				},
				"needsMoreThoughts": {
					Type:        ai.TypeBoolean,
					Description: "If more thoughts are needed",
				},
				"revisesThought": {
					Type:        ai.TypeInteger,
					Description: "Which thought is being reconsidered",
				},
			},
			Required: []string{"nextThoughtNeeded", "thought", "thoughtNumber", "totalThoughts"},
		},
	}
}

// Handler returns the function handler for the sequentialthinking tool.
func (t *SequentialThinkingTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, args map[string]any) (map[string]any, error) {
		var params SequentialThinkingParams
		jsonBytes, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool arguments: %w", err)
		}
		if err := json.Unmarshal(jsonBytes, &params); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool arguments: %w", err)
		}

		// Check for required display message and publish event
		if t.publisher != nil {
			notification := events.NotificationEvent{
				Message:     "Thinking... " + params.Thought,
				Role:        "system",
				ContentType: "thought",
			}
			t.publisher.Publish(notification.Topic(), notification)
		}

		resp, err := t.Run(params)
		if err != nil {
			return nil, fmt.Errorf("sequentialthinking tool failed: %w", err)
		}

		responseMap := make(map[string]any)
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool response: %w", err)
		}
		if err := json.Unmarshal(jsonResp, &responseMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool response to map: %w", err)
		}

		return responseMap, nil
	}
}

// FormatOutput formats the tool's execution result for user display.
func (t *SequentialThinkingTool) FormatOutput(result map[string]any) string {
	// This tool is a special case that we want to send the thoughts as notifications
	return "Thinking..."
}
