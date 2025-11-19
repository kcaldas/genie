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

Skills provide specialized capabilities and domain knowledge. When you invoke a skill, detailed
instructions will be loaded into your context telling you exactly how to complete the task.

CRITICAL UNDERSTANDING:
- This tool ONLY loads instructions - it does NOT execute anything
- After loading a skill, READ the instructions carefully and follow them step-by-step
- The skill will tell you what to do: write Python code, execute scripts, prepare data, etc.
- To execute any scripts or code, you MUST use the Bash tool
- Skills never execute automatically - you are always in control

How skills work:
1. Load skill: When you invoke a skill, an "Active Skill" section is added to your context containing:
   - Environment: Working Directory and Skill Directory (absolute paths)
   - File Storage Rules: Where to save output files (use tmp/ directory)
   - The skill's full SKILL.md content with instructions, examples, and procedures
   - Any additional files you load via the file parameter
2. Read carefully: The loaded skill content tells you exactly what to do next
3. Follow instructions: If it says "write Python code", write it. If it says "run script.py", use Bash
4. Explore resources: Use list_files=true to see what scripts, examples, or docs are available
5. Load additional files: Use file parameter to inspect scripts, examples, or reference docs
6. Complete: Call with skill="" to clear the skill context when done

What skills may provide:
- Pre-built scripts you can execute via Bash tool
- Library documentation and code examples you use to write your own code
- Utility scripts to run after your work (e.g., recalc.py for xlsx)
- Example input files and templates
- Reference documentation and best practices
- Any combination of the above

Parameters:
- skill: Name of the skill to invoke (e.g., "pdf", "xlsx", "invoice-generator")
         Empty string "" to clear active skill or load file into active skill
- file: Optional file path relative to skill directory to load
        Used to inspect scripts, load examples, or view reference docs
        Works with skill="" (load into active) or skill="name" (reload + load)
- task: Brief description of what you need to accomplish (optional, helps with context)
- list_files: Set to true to list all files in skill directory (default: false)
              Use this first to explore what resources the skill provides

Available skills are listed in your system prompt with their descriptions.

Common workflow examples:

Example 1 - Skill with pre-built script (invoice-generator):
  1. Skill(skill="invoice-generator", task="generate invoice")
  2. Read the loaded instructions
  3. Skill(skill="", list_files=true) to see available resources
  4. Prepare input data as instructed (usually JSON file)
  5. Use Bash tool to run: python3 <skill_dir>/scripts/generate_invoice.py input.json

Example 2 - Skill with library docs (xlsx):
  1. Skill(skill="xlsx", task="create financial model")
  2. Read the loaded instructions and code examples
  3. Write your own Python code using openpyxl based on examples
  4. Use Bash tool to run your code: python3 your_script.py
  5. If you used formulas, use Bash to run: python3 <skill_dir>/recalc.py output.xlsx

Example 3 - Loading additional resources:
  1. Skill(skill="invoice-generator", file="examples/electrical_quote.json")
  2. Study the example structure
  3. Create your own data following the same pattern
  4. Use Bash tool to execute the script

Remember: Skills load instructions. You follow those instructions. Execution happens via Bash tool.`,
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
