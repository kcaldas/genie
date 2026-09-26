package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

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
	Force     bool   `json:"force"`      // Reload even when the skill is already active (optional)
	Clear     bool   `json:"clear"`      // With a skill name: remove just that skill from the active set (optional)
}

// SkillResponse defines the response structure for the skill tool
type SkillResponse struct {
	Status      string   `json:"status"`            // "loaded", "already_active", "completed", "error"
	SkillName   string   `json:"skill_name"`        // Name of the loaded skill
	Message     string   `json:"message"`           // Human-readable message
	Description string   `json:"description"`       // Skill description
	Content     string   `json:"content,omitempty"` // SKILL.md content so the model can use it in the same turn
	Files       []string `json:"files,omitempty"`   // List of files in skill directory (if list_files=true)
	Active      []string `json:"active_skills"`     // Names of all active skills after this call, in load order
	Evicted     string   `json:"evicted,omitempty"` // Skill evicted to stay under the active-skill cap
}

// NewSkillTool creates a new instance of the SkillTool
func NewSkillTool(skillManager skills.SkillManager, publisher events.Publisher) *SkillTool {
	return &SkillTool{
		skillManager: skillManager,
		publisher:    publisher,
	}
}

// Run executes the skill invocation, file load or clear
func (t *SkillTool) Run(ctx context.Context, params SkillParams) (SkillResponse, error) {
	// Case 1: Clear all active skills (skill="" and file="")
	if params.Skill == "" && params.File == "" {
		slog.DebugContext(ctx, "Clearing active skills")
		if err := t.skillManager.ClearActiveSkills(ctx); err != nil {
			slog.ErrorContext(ctx, "Failed to clear active skills", "error", err)
			return SkillResponse{
				Status:  "error",
				Message: fmt.Sprintf("Failed to clear active skills: %v", err),
			}, err
		}

		// Publish skill cleared event
		if t.publisher != nil {
			t.publisher.Publish("skill.cleared", events.SkillClearedEvent{})
		}

		slog.InfoContext(ctx, "Active skills cleared")
		return SkillResponse{
			Status:  "completed",
			Message: "All active skills cleared",
			Active:  []string{},
		}, nil
	}

	// Case 2: Clear one skill (skill!="" and clear=true)
	if params.Skill != "" && params.Clear {
		slog.DebugContext(ctx, "Clearing one active skill", "skill", params.Skill)
		removed, err := t.skillManager.DeactivateSkill(ctx, params.Skill)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to clear skill", "skill", params.Skill, "error", err)
			return SkillResponse{
				Status:    "error",
				SkillName: params.Skill,
				Message:   fmt.Sprintf("Failed to clear skill '%s': %v", params.Skill, err),
			}, err
		}
		response := SkillResponse{
			Status:    "completed",
			SkillName: params.Skill,
			Active:    t.activeSkillNames(ctx),
		}
		if removed {
			if t.publisher != nil {
				t.publisher.Publish("skill.cleared", events.SkillClearedEvent{})
			}
			slog.InfoContext(ctx, "Skill cleared", "skill", params.Skill)
			response.Message = fmt.Sprintf("Skill '%s' cleared; %s", params.Skill, describeActive(response.Active))
		} else {
			response.Message = fmt.Sprintf("Skill '%s' was not active; %s", params.Skill, describeActive(response.Active))
		}
		return response, nil
	}

	// Case 3: Load file into the most recently loaded skill (skill="" and file!="")
	if params.Skill == "" && params.File != "" {
		slog.DebugContext(ctx, "Loading file into active skill", "file", params.File)
		if err := t.skillManager.LoadSkillFile(ctx, "", params.File); err != nil {
			slog.ErrorContext(ctx, "Failed to load file into active skill", "file", params.File, "error", err)

			errMsg := fmt.Sprintf("Failed to load file '%s': %v", params.File, err)
			if active := t.activeSkillNames(ctx); len(active) > 0 {
				errMsg += fmt.Sprintf("\nActive skills: %s (file was looked up in '%s'; pass skill=<name> to target another)", strings.Join(active, ", "), active[len(active)-1])
			}

			return SkillResponse{
				Status:  "error",
				Message: errMsg,
			}, err
		}

		slog.InfoContext(ctx, "File loaded successfully into skill context", "file", params.File)
		return SkillResponse{
			Status:  "loaded",
			Message: fmt.Sprintf("File '%s' loaded successfully into skill context", params.File),
			Active:  t.activeSkillNames(ctx),
		}, nil
	}

	// Idempotent load: the requested skill is already active, so its
	// instructions are already in context. Re-injecting them only costs
	// tokens, unless force is set.
	if !params.Force {
		if resp, handled, err := t.runAlreadyActive(ctx, params); handled {
			return resp, err
		}
	}

	// Case 4 & 5: Load and activate skill (skill!="")
	slog.DebugContext(ctx, "Loading skill", "skill", params.Skill, "file", params.File)
	skill, err := t.skillManager.LoadSkill(ctx, params.Skill)
	if err != nil {
		// Check if error message contains "not found"
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			slog.ErrorContext(ctx, "Skill not found", "skill", params.Skill, "error", err)

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
		slog.ErrorContext(ctx, "Failed to load skill", "skill", params.Skill, "error", err)
		return SkillResponse{
			Status:    "error",
			SkillName: params.Skill,
			Message:   fmt.Sprintf("Failed to load skill: %v", err),
		}, err
	}

	// Add to the active set
	evicted, err := t.skillManager.ActivateSkill(ctx, skill)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to activate skill", "skill", params.Skill, "error", err)
		return SkillResponse{
			Status:    "error",
			SkillName: params.Skill,
			Message:   fmt.Sprintf("Failed to activate skill: %v", err),
		}, err
	}

	slog.InfoContext(ctx, "Skill activated successfully", "skill", params.Skill, "base_dir", skill.BaseDir, "evicted", evicted)

	// Publish skill invoked event
	if t.publisher != nil {
		t.publisher.Publish("skill.invoked", events.SkillInvokedEvent{
			Skill: skill,
		})
	}

	response := SkillResponse{
		Status:      "loaded",
		SkillName:   skill.Name,
		Description: skill.Description,
		Content:     skill.Content,
		Evicted:     evicted,
	}

	// Case 5: If file was also specified, load it now that skill is active
	if params.File != "" {
		slog.DebugContext(ctx, "Loading additional file for activated skill", "skill", params.Skill, "file", params.File)
		if err := t.skillManager.LoadSkillFile(ctx, skill.Name, params.File); err != nil {
			slog.ErrorContext(ctx, "Skill activated but file load failed", "skill", params.Skill, "file", params.File, "error", err)
			return SkillResponse{
				Status:    "error",
				SkillName: params.Skill,
				Message:   fmt.Sprintf("Skill '%s' loaded but failed to load file '%s': %v\nSkill directory: %s", skill.Name, params.File, err, skill.BaseDir),
			}, err
		}
		slog.InfoContext(ctx, "Skill and file loaded successfully", "skill", params.Skill, "file", params.File)
		response.Message = fmt.Sprintf("Skill '%s' loaded and file '%s' loaded successfully.", skill.Name, params.File)
	} else {
		slog.InfoContext(ctx, "Skill loaded successfully", "skill", params.Skill)
		response.Message = fmt.Sprintf("Skill '%s' loaded successfully.", skill.Name)
	}
	response.Active = t.activeSkillNames(ctx)
	response.Message += " Follow the instructions in the content field; the skill stays in your context on later turns alongside the other active skills."
	if evicted != "" {
		response.Message += fmt.Sprintf(" Active skill '%s' was evicted to stay within the limit of %d active skills; reload it if you still need it.", evicted, t.skillManager.MaxActiveSkills())
	}

	t.appendFileListing(ctx, params, skill.Name, &response)
	return response, nil
}

// runAlreadyActive handles Skill(skill=X) when X is already in the active
// set. It reports handled=false when a full load is needed: X is not
// active, or the manager cannot tell.
func (t *SkillTool) runAlreadyActive(ctx context.Context, params SkillParams) (SkillResponse, bool, error) {
	activeSkills, err := t.skillManager.GetActiveSkills(ctx)
	if err != nil {
		return SkillResponse{}, false, nil
	}
	var active *skills.Skill
	for _, candidate := range activeSkills {
		if candidate.Name == params.Skill {
			active = candidate
			break
		}
	}
	if active == nil {
		return SkillResponse{}, false, nil
	}

	response := SkillResponse{
		Status:      "already_active",
		SkillName:   active.Name,
		Description: active.Description,
	}

	if params.File != "" {
		if _, loaded := active.LoadedFiles[filepath.ToSlash(filepath.Clean(params.File))]; !loaded {
			slog.DebugContext(ctx, "Loading file into already active skill", "skill", active.Name, "file", params.File)
			if err := t.skillManager.LoadSkillFile(ctx, active.Name, params.File); err != nil {
				slog.ErrorContext(ctx, "Failed to load file into active skill", "skill", active.Name, "file", params.File, "error", err)
				return SkillResponse{
					Status:    "error",
					SkillName: active.Name,
					Message:   fmt.Sprintf("Skill '%s' is already active but failed to load file '%s': %v\nSkill directory: %s", active.Name, params.File, err, active.BaseDir),
				}, true, err
			}
			response.Status = "loaded"
			response.Message = fmt.Sprintf("Skill '%s' is already active; file '%s' loaded into skill context", active.Name, params.File)
		} else {
			response.Message = fmt.Sprintf("Skill '%s' is already active and file '%s' is already loaded; instructions are in your context", active.Name, params.File)
		}
	} else {
		response.Message = fmt.Sprintf("Skill '%s' is already active; instructions are in your context", active.Name)
	}
	response.Active = t.activeSkillNames(ctx)

	slog.InfoContext(ctx, "Skill already active", "skill", active.Name, "status", response.Status)

	t.appendFileListing(ctx, params, active.Name, &response)
	return response, true, nil
}

// appendFileListing honours list_files on a load or already-active response.
func (t *SkillTool) appendFileListing(ctx context.Context, params SkillParams, skillName string, response *SkillResponse) {
	if !params.ListFiles {
		return
	}
	files, err := t.skillManager.ListSkillFiles(ctx, skillName)
	if err != nil {
		slog.WarnContext(ctx, "Failed to list skill files", "skill", skillName, "error", err)
		response.Message += fmt.Sprintf("\n\nWarning: Could not list skill files: %v", err)
		return
	}
	response.Files = files
	response.Message += fmt.Sprintf("\n\nSkill directory contains %d files (see files array)", len(files))
}

// activeSkillNames lists the session's active skills in load order; an
// empty, non-nil slice when there are none or the catalog is unavailable.
func (t *SkillTool) activeSkillNames(ctx context.Context) []string {
	names := []string{}
	activeSkills, err := t.skillManager.GetActiveSkills(ctx)
	if err != nil {
		return names
	}
	for _, skill := range activeSkills {
		names = append(names, skill.Name)
	}
	return names
}

func describeActive(names []string) string {
	if len(names) == 0 {
		return "no skills remain active"
	}
	return fmt.Sprintf("active skills: %s", strings.Join(names, ", "))
}

// Declaration returns the function declaration for the skill tool
func (t *SkillTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name: "Skill",
		Description: fmt.Sprintf(`Load specialized skill instructions to handle complex, domain-specific tasks.

Several skills can be active at once. Loading a skill ADDS it to the active set; each active skill
appears in your context under its own "Active Skill: <name>" section containing detailed
instructions, environment paths, and the skill's full content. Read and follow those instructions.
Active skills stay in your context on later turns. Loading a skill that is already active is a
no-op that returns status "already_active" (its instructions are already in your context), so do
not reload a skill you have already loaded.

CRITICAL: This tool ONLY loads instructions - it does NOT execute scripts or code.
- Skills tell you what to do (write code, prepare data, run scripts, etc.)
- To execute any scripts or code, you MUST use the Bash tool
- Use list_files=true to explore what resources a skill provides

Parameters:
- skill: Name of skill to load (e.g., "pdf", "xlsx", "invoice-generator")
         Empty string "" clears ALL active skills (or, with file, loads a file into the most recently loaded skill)
- file: Optional file path relative to skill directory (e.g., "examples/sample.json")
        Load examples, scripts, or reference docs from the skill; a file already loaded is not loaded again
- clear: With a skill name, remove just that skill from the active set: Skill(skill="xlsx", clear=true)
- task: Brief description of what you need to accomplish (optional)
- list_files: List all files in skill directory (optional, useful for exploring)
- force: Reload the skill even when it is already active (optional, default false)

Basic workflow:
1. Load skill: Skill(skill="xlsx") - Instructions appear in your context
2. Read and follow the loaded instructions step-by-step
3. Use Bash tool to execute any scripts or code as instructed
Load another skill the same way when the task needs it; both stay active.

Clearing is optional. Skill(skill="xlsx", clear=true) removes one skill and Skill(skill="") removes
all; use them only when switching away from a skill mid-turn or when its instructions should no
longer apply. There is no need to clear skills when you are done with them. As a guard against
context growth, at most %d skills stay active per session; beyond that the oldest is evicted and
the result says which.

Available skills are listed in your system prompt with their descriptions.`, skills.DefaultMaxActiveSkills),
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"skill": {
					Type:        ai.TypeString,
					Description: "Name of the skill to load (empty string clears all active skills; loading an already active skill is a no-op)",
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
				"force": {
					Type:        ai.TypeBoolean,
					Description: "Reload the skill even when it is already active (optional, default false)",
				},
				"clear": {
					Type:        ai.TypeBoolean,
					Description: "With a skill name, remove just that skill from the active set (optional, default false)",
				},
			},
			Required: []string{"skill"},
		},
	}
}

// Handler returns the function handler for the skill tool
func (t *SkillTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, args map[string]any) (ai.ToolOutput, error) {
		var params SkillParams
		jsonBytes, err := json.Marshal(args)
		if err != nil {
			return ai.ToolOutput{}, fmt.Errorf("failed to marshal tool arguments: %w", err)
		}
		if err := json.Unmarshal(jsonBytes, &params); err != nil {
			return ai.ToolOutput{}, fmt.Errorf("failed to unmarshal tool arguments: %w", err)
		}

		// Publish notification
		if t.publisher != nil {
			var message string
			switch {
			case params.Skill == "" && params.File == "":
				message = "Clearing active skills..."
			case params.Skill == "" && params.File != "":
				message = fmt.Sprintf("Loading skill file: %s", params.File)
			case params.Clear:
				message = fmt.Sprintf("Clearing skill: %s", params.Skill)
			default:
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
			slog.ErrorContext(ctx, "Failed to marshal skill response", "error", marshalErr)
			return ai.ToolOutput{}, fmt.Errorf("failed to marshal tool response: %w", marshalErr)
		}
		if unmarshalErr := json.Unmarshal(jsonResp, &responseMap); unmarshalErr != nil {
			slog.ErrorContext(ctx, "Failed to unmarshal skill response to map", "error", unmarshalErr)
			return ai.ToolOutput{}, fmt.Errorf("failed to unmarshal tool response to map: %w", unmarshalErr)
		}

		// Handle errors from Run()
		if err != nil {
			// Check if this is an operational error (skill not found, file not found, etc.)
			// These should be communicated to the LLM, not terminate the generation
			if resp.Status == "error" {
				// Log the error but return the response to LLM with nil error
				slog.WarnContext(ctx, "Skill operation failed", "status", resp.Status, "message", resp.Message, "error", err)

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
				return failedOutput(responseMap), nil
			}

			// For unexpected errors, log and propagate
			slog.ErrorContext(ctx, "Unexpected skill tool error", "error", err)
			return ai.ToolOutput{}, err
		}

		return resultOutput(responseMap), nil
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
	case "already_active":
		return fmt.Sprintf("✓ Skill '%s' already active", skillName)
	case "completed":
		if skillName != "" {
			return fmt.Sprintf("✓ Skill '%s' cleared", skillName)
		}
		return "✓ Active skills cleared"
	case "error":
		return fmt.Sprintf("✗ Error: %s", message)
	default:
		return message
	}
}
