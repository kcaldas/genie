package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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
	Skill     string `json:"skill"`      // Name of the skill to invoke (empty to complete)
	Task      string `json:"task"`       // Description of the task (optional)
	File      string `json:"file"`       // Additional file to load from skill directory (optional)
	ListFiles bool   `json:"list_files"` // List files in skill directory (optional)
}

// SkillResponse defines the response structure for the skill tool
type SkillResponse struct {
	Status      string   `json:"status"`       // "loaded", "completed", "error"
	SkillName   string   `json:"skill_name"`   // Name of the loaded skill
	Message     string   `json:"message"`      // Human-readable message
	Description string   `json:"description"`  // Skill description
	Files       []string `json:"files,omitempty"` // List of files in skill directory (if list_files=true)
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
	// Case 1: Clear active skill (skill="" and file="")
	if params.Skill == "" && params.File == "" {
		slog.Debug("Clearing active skill", "params", params)
		if err := t.skillManager.ClearActiveSkill(ctx); err != nil {
			slog.Error("Failed to clear active skill", "error", err)
			return SkillResponse{
				Status:  "error",
				Message: fmt.Sprintf("Failed to clear active skill: %v", err),
			}, err
		}

		// Publish skill cleared event
		if t.publisher != nil {
			t.publisher.Publish("skill.cleared", events.SkillClearedEvent{})
		}

		slog.Info("Skill cleared successfully")
		return SkillResponse{
			Status:  "completed",
			Message: "Skill completed and context cleared",
		}, nil
	}

	// Case 2: Load file without activating new skill (skill="" and file!="")
	if params.Skill == "" && params.File != "" {
		slog.Debug("Loading file into active skill", "file", params.File)
		if err := t.skillManager.LoadSkillFile(ctx, params.File); err != nil {
			slog.Error("Failed to load file into active skill", "file", params.File, "error", err)

			// Check if there's an active skill to provide context
			activeSkill, _ := t.skillManager.GetActiveSkill(ctx)
			errMsg := fmt.Sprintf("Failed to load file '%s': %v", params.File, err)
			if activeSkill != nil {
				errMsg += fmt.Sprintf("\nActive skill: %s (directory: %s)", activeSkill.Name, activeSkill.BaseDir)
			}

			return SkillResponse{
				Status:  "error",
				Message: errMsg,
			}, err
		}

		slog.Info("File loaded successfully into skill context", "file", params.File)
		return SkillResponse{
			Status:  "loaded",
			Message: fmt.Sprintf("File '%s' loaded successfully into skill context", params.File),
		}, nil
	}

	// Case 3 & 4: Load and activate skill (skill!="")
	slog.Debug("Loading skill", "skill", params.Skill, "file", params.File, "task", params.Task)
	skill, err := t.skillManager.LoadSkill(ctx, params.Skill)
	if err != nil {
		// Check if error message contains "not found"
		errMsg := err.Error()
		if stringContains(errMsg, "not found") {
			slog.Error("Skill not found", "skill", params.Skill, "error", err)

			// Get available skills to provide helpful suggestions
			availableSkills, listErr := t.skillManager.ListSkills(ctx)
			helpMsg := fmt.Sprintf("Skill '%s' not found.", params.Skill)
			if listErr == nil && len(availableSkills) > 0 {
				skillNames := make([]string, 0, len(availableSkills))
				for _, s := range availableSkills {
					skillNames = append(skillNames, s.Name)
				}
				if len(skillNames) <= 5 {
					helpMsg += fmt.Sprintf(" Available skills: %v", skillNames)
				} else {
					helpMsg += fmt.Sprintf(" %d skills are available. Check skill directory or documentation.", len(skillNames))
				}
			}

			return SkillResponse{
				Status:    "error",
				SkillName: params.Skill,
				Message:   helpMsg,
			}, err
		}
		slog.Error("Failed to load skill", "skill", params.Skill, "error", err)
		return SkillResponse{
			Status:    "error",
			SkillName: params.Skill,
			Message:   fmt.Sprintf("Failed to load skill: %v", err),
		}, err
	}

	// Set as active skill
	if err := t.skillManager.SetActiveSkill(ctx, skill); err != nil {
		slog.Error("Failed to activate skill", "skill", params.Skill, "error", err)
		return SkillResponse{
			Status:    "error",
			SkillName: params.Skill,
			Message:   fmt.Sprintf("Failed to activate skill: %v", err),
		}, err
	}

	slog.Info("Skill activated successfully", "skill", params.Skill, "base_dir", skill.BaseDir)

	// Publish skill invoked event
	if t.publisher != nil {
		t.publisher.Publish("skill.invoked", events.SkillInvokedEvent{
			Skill: skill,
		})
	}

	// Case 4: If file was also specified, load it now that skill is active
	if params.File != "" {
		slog.Debug("Loading additional file for activated skill", "skill", params.Skill, "file", params.File)
		if err := t.skillManager.LoadSkillFile(ctx, params.File); err != nil {
			slog.Error("Skill activated but file load failed", "skill", params.Skill, "file", params.File, "error", err)
			return SkillResponse{
				Status:    "error",
				SkillName: params.Skill,
				Message:   fmt.Sprintf("Skill '%s' loaded but failed to load file '%s': %v\nSkill directory: %s", skill.Name, params.File, err, skill.BaseDir),
			}, err
		}

		slog.Info("Skill and file loaded successfully", "skill", params.Skill, "file", params.File)

		response := SkillResponse{
			Status:      "loaded",
			SkillName:   skill.Name,
			Description: skill.Description,
			Message:     fmt.Sprintf("Skill '%s' loaded and file '%s' loaded successfully.", skill.Name, params.File),
		}

		// If list_files requested, list all files in skill directory
		if params.ListFiles {
			files, err := t.listSkillFiles(skill.BaseDir)
			if err != nil {
				slog.Warn("Failed to list skill files", "skill", params.Skill, "error", err)
				response.Message += fmt.Sprintf("\n\nWarning: Could not list skill files: %v", err)
			} else {
				response.Files = files
				response.Message += fmt.Sprintf("\n\nSkill directory contains %d files (see files array)", len(files))
			}
		}

		return response, nil
	}

	// Case 3: Skill loaded without additional file
	slog.Info("Skill loaded successfully", "skill", params.Skill)

	response := SkillResponse{
		Status:      "loaded",
		SkillName:   skill.Name,
		Description: skill.Description,
		Message:     fmt.Sprintf("Skill '%s' loaded successfully. The skill content is now available in your context.", skill.Name),
	}

	// If list_files requested, list all files in skill directory
	if params.ListFiles {
		files, err := t.listSkillFiles(skill.BaseDir)
		if err != nil {
			slog.Warn("Failed to list skill files", "skill", params.Skill, "error", err)
			response.Message += fmt.Sprintf("\n\nWarning: Could not list skill files: %v", err)
		} else {
			response.Files = files
			response.Message += fmt.Sprintf("\n\nSkill directory contains %d files (see files array)", len(files))
		}
	}

	return response, nil
}

// listSkillFiles recursively lists all files in a skill directory
func (t *SkillTool) listSkillFiles(baseDir string) ([]string, error) {
	var files []string

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories, only include files
		if !info.IsDir() {
			// Get relative path from baseDir
			relPath, err := filepath.Rel(baseDir, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk skill directory: %w", err)
	}

	return files, nil
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
		Description: `Load specialized skill instructions to handle complex, domain-specific tasks.

CRITICAL: This tool ONLY loads instructions - it does NOT execute scripts or perform actions.
- Skills provide HOW-TO guidance, procedures, and reference documentation
- To execute scripts mentioned in skills, you MUST use the Bash tool
- To load additional skill files for inspection, use the file parameter

Skills are modular capability packages that provide focused expertise for specific types of work.
Each skill contains detailed instructions, best practices, and context for a particular domain.

When to use this tool:
- When the task requires specialized domain knowledge
- When you need specific procedures or workflows
- When a skill's description matches the user's request
- When you need focused context for a particular task type

How it works:
1. Invoke with skill name to load its instructions into your context
2. The skill's content (instructions, examples, procedures) and base directory path will be available
3. FOLLOW the skill's instructions - if it tells you to run scripts, use the Bash tool
4. Load additional files using the file parameter to inspect scripts or references
5. When done, invoke with empty skill name to clear the skill context

Parameters:
- skill: The name of the skill to invoke (e.g., "pdf", "xlsx", "invoice-generator")
         Use empty string "" to load file into active skill or clear context
         Provide skill name with file parameter to reload skill and load file together
- file: Optional file path relative to skill directory to load into context
        Use this to inspect scripts, load reference docs, or view examples
        Works with skill="" (load into active) or skill="name" (reload + load)
- task: Brief description of what you need to accomplish (helps with context)
- list_files: Set to true to list all files in the skill directory (default: false)
              Useful for discovering available scripts, references, and examples

Skill execution workflow:
1. Load skill: Skill(skill="invoice-generator") - Gets instructions in context
2. Follow instructions: If skill says "run python3 script.py", use Bash tool
3. Load additional files (two ways):
   - Skill(skill="invoice-generator", file="examples/sample.json") - Reload skill + file
   - Skill(skill="", file="examples/sample.json") - Load file into active skill
4. Execute scripts: Use Bash tool with paths from skill's Environment section
5. Complete: Skill(skill="") - Clear skill from context

Available skills are listed in your system prompt with their descriptions.

Example usage:
1. Load a skill: Skill(skill="pdf", task="extract text from PDF")
2. Explore skill files: Skill(skill="pdf", list_files=true) to see available resources
3. Load specific file: Skill(skill="pdf", file="extract_text.py") to inspect script
4. Execute script: Use Bash tool with the base path from skill context
5. Load reference: Skill(skill="pdf", file="references/guide.md") for more details
6. Clear when done: Skill(skill="") to clear the skill and all loaded files`,
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
				"list_files": {
					Type:        ai.TypeBoolean,
					Description: "List all files in the skill directory (optional, useful for exploring skill resources)",
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

		// Convert response to map
		responseMap := make(map[string]any)
		jsonResp, marshalErr := json.Marshal(resp)
		if marshalErr != nil {
			slog.Error("Failed to marshal skill response", "error", marshalErr)
			return nil, fmt.Errorf("failed to marshal tool response: %w", marshalErr)
		}
		if unmarshalErr := json.Unmarshal(jsonResp, &responseMap); unmarshalErr != nil {
			slog.Error("Failed to unmarshal skill response to map", "error", unmarshalErr)
			return nil, fmt.Errorf("failed to unmarshal tool response to map: %w", unmarshalErr)
		}

		// Handle errors from Run()
		if err != nil {
			// Check if this is an operational error (skill not found, file not found, etc.)
			// These should be communicated to the LLM, not terminate the generation
			if resp.Status == "error" {
				// Log the error but return the response to LLM with nil error
				slog.Warn("Skill operation failed", "status", resp.Status, "message", resp.Message, "error", err)

				// Publish error event for UI
				if t.publisher != nil {
					errorEvent := events.NotificationEvent{
						Message:     fmt.Sprintf("Skill error: %s", resp.Message),
						Role:        "system",
						ContentType: "error",
					}
					t.publisher.Publish(errorEvent.Topic(), errorEvent)
				}

				// Return the error response to LLM (with nil error so generation continues)
				return responseMap, nil
			}

			// For unexpected errors, log and propagate
			slog.Error("Unexpected skill tool error", "error", err)
			return nil, err
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
