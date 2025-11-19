package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/skills"
)

// SkillTool allows the AI to invoke specialized skills
type SkillTool struct {
	skillManager skills.SkillManager
	publisher    events.Publisher
}

// SkillParams defines the parameters for the skill tool
type SkillParams struct {
	Skill string `json:"skill"` // Name of the skill to invoke (empty to complete)
	Task  string `json:"task"`  // Description of the task (optional)
	File  string `json:"file"`  // Additional file to load from skill directory (optional)
}

// SkillResponse defines the response structure for the skill tool
type SkillResponse struct {
	Status      string `json:"status"`       // "loaded", "completed", "error"
	SkillName   string `json:"skill_name"`   // Name of the loaded skill
	Message     string `json:"message"`      // Human-readable message
	Description string `json:"description"`  // Skill description
}

// NewSkillTool creates a new instance of the SkillTool
func NewSkillTool(skillManager skills.SkillManager, publisher events.Publisher) *SkillTool {
	return &SkillTool{
		skillManager: skillManager,
		publisher:    publisher,
	}
}

// Run executes the skill invocation or completion
func (t *SkillTool) Run(ctx context.Context, params SkillParams) (SkillResponse, error) {
	// If skill name is empty, complete the active skill
	if params.Skill == "" {
		if err := t.skillManager.ClearActiveSkill(ctx); err != nil {
			return SkillResponse{
				Status:  "error",
				Message: fmt.Sprintf("Failed to clear active skill: %v", err),
			}, err
		}

		// Publish skill cleared event
		if t.publisher != nil {
			t.publisher.Publish("skill.cleared", events.SkillClearedEvent{})
		}

		return SkillResponse{
			Status:  "completed",
			Message: "Skill completed and context cleared",
		}, nil
	}

	// If file is specified, load additional file into active skill
	if params.File != "" {
		if err := t.skillManager.LoadSkillFile(ctx, params.File); err != nil {
			return SkillResponse{
				Status:  "error",
				Message: fmt.Sprintf("Failed to load file %s: %v", params.File, err),
			}, err
		}

		return SkillResponse{
			Status:  "loaded",
			Message: fmt.Sprintf("File '%s' loaded successfully into skill context", params.File),
		}, nil
	}

	// Load the skill
	skill, err := t.skillManager.LoadSkill(ctx, params.Skill)
	if err != nil {
		// Check if error message contains "not found"
		errMsg := err.Error()
		if stringContains(errMsg, "not found") {
			return SkillResponse{
				Status:    "error",
				SkillName: params.Skill,
				Message:   fmt.Sprintf("Skill not found: %s", params.Skill),
			}, err
		}
		return SkillResponse{
			Status:    "error",
			SkillName: params.Skill,
			Message:   fmt.Sprintf("Failed to load skill: %v", err),
		}, err
	}

	// Set as active skill
	if err := t.skillManager.SetActiveSkill(ctx, skill); err != nil {
		return SkillResponse{
			Status:    "error",
			SkillName: params.Skill,
			Message:   fmt.Sprintf("Failed to activate skill: %v", err),
		}, err
	}

	// Publish skill invoked event
	if t.publisher != nil {
		t.publisher.Publish("skill.invoked", events.SkillInvokedEvent{
			Skill: skill,
		})
	}

	// Extract skill information directly from the skill struct
	return SkillResponse{
		Status:      "loaded",
		SkillName:   skill.Name,
		Description: skill.Description,
		Message:     fmt.Sprintf("Skill '%s' loaded successfully. The skill content is now available in your context.", skill.Name),
	}, nil
}

// Helper function to check if a string contains a substring
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && stringIndexOf(s, substr) >= 0)
}

func stringIndexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Declaration returns the function declaration for the skill tool
func (t *SkillTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "Skill",
		Description: `Invoke specialized skills to handle complex, domain-specific tasks.

Skills are modular capability packages that provide focused expertise for specific types of work.
Each skill contains detailed instructions, best practices, and context for a particular domain.

When to use this tool:
- When the task requires specialized domain knowledge
- When you need specific procedures or workflows
- When a skill's description matches the user's request
- When you need focused context for a particular task type

How it works:
1. Invoke with a skill name to load its content into your context
2. The skill's full content (instructions, examples, procedures) and base directory path will be available
3. You can load additional files from the skill directory using the file parameter
4. Use the skill's guidance to complete the task
5. When done, invoke with empty skill name to clear the skill context

Parameters:
- skill: The name of the skill to invoke (e.g., "codebase-search", "test-helper")
         Use empty string "" to signal skill completion and clear context
- file: Optional file path relative to skill directory to load into context
        Use this to load reference documentation, examples, or scripts you need to inspect
- task: Brief description of what you need to accomplish (helps with context)

The skill's base directory path is provided in context, enabling you to:
- Execute scripts from the skill directory using Bash tool
- Load reference files using the file parameter
- Access any skill resources as needed

Available skills are listed in your system prompt with their descriptions.

Example usage:
1. Load a skill: Skill(skill="pdf", task="extract text from PDF")
2. Load additional file: Skill(skill="pdf", file="extract_text.py") to inspect script
3. Execute script: Use Bash tool with the base path from skill context
4. Load reference: Skill(skill="pdf", file="references/guide.md") for more details
5. Clear when done: Skill(skill="") to clear the skill and all loaded files`,
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"skill": {
					Type:        ai.TypeString,
					Description: "Name of the skill to invoke (empty string to complete and clear)",
				},
				"file": {
					Type:        ai.TypeString,
					Description: "Optional file path relative to skill directory to load (e.g., 'extract_text.py' or 'references/guide.md')",
				},
				"task": {
					Type:        ai.TypeString,
					Description: "Brief description of the task (optional, helps with context)",
				},
			},
			Required: []string{"skill"},
		},
	}
}

// Handler returns the function handler for the skill tool
func (t *SkillTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, args map[string]any) (map[string]any, error) {
		var params SkillParams
		jsonBytes, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool arguments: %w", err)
		}
		if err := json.Unmarshal(jsonBytes, &params); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool arguments: %w", err)
		}

		// Publish notification
		if t.publisher != nil {
			var message string
			if params.Skill == "" {
				message = "Completing skill and clearing context..."
			} else {
				message = fmt.Sprintf("Loading skill: %s", params.Skill)
			}

			notification := events.NotificationEvent{
				Message:     message,
				Role:        "system",
				ContentType: "info",
			}
			t.publisher.Publish(notification.Topic(), notification)
		}

		resp, err := t.Run(ctx, params)
		if err != nil {
			return nil, err
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

// FormatOutput formats the tool's execution result for user display
func (t *SkillTool) FormatOutput(result map[string]any) string {
	status, _ := result["status"].(string)
	skillName, _ := result["skill_name"].(string)
	message, _ := result["message"].(string)
	description, _ := result["description"].(string)

	switch status {
	case "loaded":
		if description != "" {
			return fmt.Sprintf("✓ Skill '%s' loaded\n  %s", skillName, description)
		}
		return fmt.Sprintf("✓ Skill '%s' loaded", skillName)
	case "completed":
		return "✓ Skill completed"
	case "error":
		return fmt.Sprintf("✗ Error: %s", message)
	default:
		return message
	}
}
