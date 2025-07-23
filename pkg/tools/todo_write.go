package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
)

// TodoWriteTool implements the TodoWrite tool as specified
type TodoWriteTool struct {
	manager TodoManager
}

// NewTodoWriteTool creates a new TodoWrite tool
func NewTodoWriteTool(manager TodoManager) Tool {
	return &TodoWriteTool{
		manager: manager,
	}
}

// Declaration returns the function declaration for the TodoWrite tool
func (t *TodoWriteTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "TodoWrite",
		Description: `Manage a structured task list for tracking work progress. Replaces the entire todo list each time (not incremental).

⚠️ IMPORTANT: Use this for ACTIONABLE TASKS, not for simple communication or observations!

WHEN TO USE - Required for:
• Complex multi-step tasks (3+ distinct steps)
• User provides multiple tasks (numbered/bulleted list)
• Multi-file changes or complex refactoring
• Debugging requiring systematic investigation
• When user explicitly requests todo/task tracking
• IMMEDIATELY when receiving a complex request (be proactive)

WHEN TO USE - Recommended for:
• Breaking down complex problems into manageable steps
• Planning your approach before implementation
• Tasks needing coordination across components
• Work that spans multiple tool calls
• When progress visibility helps the user
• Complex analysis with multiple phases
• Organizing your thinking for systematic execution

DO NOT USE for:
• Single, straightforward tasks
• Trivial operations (< 3 steps)
• Pure Q&A or explanations
• Tasks you'll complete immediately
• Micro-managing individual function calls
• Simple communication ("Ask user for clarification", "Greet the user")
• Non-actionable observations ("User seems happy", "This looks complex")
• Immediate conversational responses ("Tell user about X", "Explain Y")
• Tasks without clear implementation steps

CRITICAL WORKFLOW RULES:
1. Always check current todos first (todos preserved in context)
2. Include ALL todos when updating (replaces entire list)
3. Only ONE task "in_progress" at a time
4. Mark "completed" IMMEDIATELY after finishing
5. If blocked, keep as "in_progress" and add blocker tasks

HANDLING BLOCKERS:
When blocked on a task:
1. Keep the blocked task as "in_progress"
2. Add new high-priority tasks to resolve blockers
3. Work on blocker tasks first
4. Return to original task once unblocked
Example: "Implement auth" blocked → Add "Fix database connection" task

STATUS RULES:
• pending → Not started yet
• in_progress → Actively working (max 1)
• completed → FULLY done with no errors

NEVER mark completed if:
• Tests failing
• Implementation incomplete  
• Unresolved errors exist
• Missing dependencies
• Blocked by external factors

EXAMPLE:
User: "Add dark mode to the app"
Todos:
1. Analyze current theme system (high)
2. Design dark color palette (high)
3. Implement theme switching (medium)
4. Update components for dark mode (medium)
5. Test theme switching (low)

BEST PRACTICES:
• Action-oriented descriptions ("Fix auth bug in login()")
• Include enough context to resume later
• 3-10 meaningful tasks (not 50 micro-steps)
• Update in real-time as you work
• Add new tasks when discovering work

ANTI-PATTERNS TO AVOID:
✗ Batch updating multiple completions at once
✗ Creating todos for every tiny step
✗ Forgetting to mark in_progress when starting
✗ Having multiple tasks in_progress simultaneously
✗ Marking tasks complete when they have errors
✗ Creating vague tasks like "Fix stuff"
✗ Using todos for simple 1-2 step tasks
✗ Creating todos for simple communication: "Ask user about X", "Greet the user"
✗ Creating todos for observations: "User seems confused", "This might be wrong"
✗ Creating todos for immediate responses: "Say hello", "Thank the user"

IMPORTANT REMINDERS:
• Todos persist across the conversation in context
• Check existing todos at conversation start
• IDs must be unique and consistent
• Empty todo list is valid (all tasks complete)
• Add discovered tasks as you find them
• Remove irrelevant tasks to keep list clean

SPECIAL CASES:
• User asks "what's left?" → Check todos before answering
• Discovering prerequisites → Add them as high priority
• Tests fail after "complete" → Revert to "in_progress"
• User cancels/changes direction → Update or clear todos
• Resuming after break → Always check todos first`,
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Parameters for updating the todo list",
			Properties: map[string]*ai.Schema{
				"todos": {
					Type:        ai.TypeArray,
					Description: "Complete array of todo items (replaces existing list)",
					Items: &ai.Schema{
						Type:        ai.TypeObject,
						Description: "A single todo item",
						Properties: map[string]*ai.Schema{
							"id": {
								Type:        ai.TypeString,
								Description: "Unique identifier for the todo item (required)",
							},
							"content": {
								Type:        ai.TypeString,
								Description: "Task description (required, non-empty)",
							},
							"status": {
								Type:        ai.TypeString,
								Description: "Task state: pending, in_progress, or completed (required)",
							},
							"priority": {
								Type:        ai.TypeString,
								Description: "Task importance: high, medium, or low (required)",
							},
						},
						Required: []string{"id", "content", "status", "priority"},
					},
				},
			},
			Required: []string{"todos"},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Result of the todo update operation",
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether the operation was successful",
				},
				"message": {
					Type:        ai.TypeString,
					Description: "Success message or error description",
				},
				"todos": {
					Type:        ai.TypeArray,
					Description: "The complete list of todo items after the update",
					Items: &ai.Schema{
						Type:        ai.TypeObject,
						Properties: map[string]*ai.Schema{
							"id": {
								Type: ai.TypeString,
							},
							"content": {
								Type: ai.TypeString,
							},
							"status": {
								Type: ai.TypeString,
							},
							"priority": {
								Type: ai.TypeString,
							},
						},
						Required: []string{"id", "content", "status", "priority"},
					},
				},
			},
			Required: []string{"success", "message", "todos"},
		},
	}
}

// Handler returns the function handler for the TodoWrite tool
func (t *TodoWriteTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		// Extract todos parameter
		todosParam, exists := params["todos"]
		if !exists {
			return map[string]any{
				"success": false,
				"message": "Missing required parameter 'todos'",
				"todos":   []interface{}{},
			}, fmt.Errorf("missing required parameter 'todos'")
		}
		
		// Convert to array of any
		todosAnyArray, ok := todosParam.([]any)
		if !ok {
			return map[string]any{
				"success": false,
				"message": "Parameter 'todos' must be an array",
				"todos":   []interface{}{},
			}, fmt.Errorf("parameter 'todos' must be an array")
		}
		
		// Convert to TodoItem structs
		todos := make([]TodoItem, len(todosAnyArray))
		for i, todoInterface := range todosAnyArray {
			todoMap, ok := todoInterface.(map[string]interface{})
			if !ok {
				return map[string]any{
					"success": false,
					"message": fmt.Sprintf("Todo at index %d is not a valid object", i),
					"todos":   []interface{}{},
				}, fmt.Errorf("todo at index %d is not a valid object", i)
			}
			
			// Extract required fields
			id, idOk := todoMap["id"].(string)
			content, contentOk := todoMap["content"].(string)
			status, statusOk := todoMap["status"].(string)
			priority, priorityOk := todoMap["priority"].(string)
			
			if !idOk || !contentOk || !statusOk || !priorityOk {
				return map[string]any{
					"success": false,
					"message": fmt.Sprintf("Todo at index %d is missing required fields (id, content, status, priority)", i),
					"todos":   []interface{}{},
				}, fmt.Errorf("todo at index %d is missing required fields", i)
			}
			
			todos[i] = TodoItem{
				ID:       id,
				Content:  content,
				Status:   Status(status),
				Priority: Priority(priority),
			}
		}
		
		// Write to manager (includes validation)
		err := t.manager.Write(todos)
		if err != nil {
			return map[string]any{
				"success": false,
				"message": fmt.Sprintf("Validation failed: %v", err),
				"todos":   []interface{}{},
			}, err
		}
		
		// Read the current state of todos from the manager to return the canonical list
		currentTodos := t.manager.Read()
		var responseTodos []map[string]interface{}
		for _, item := range currentTodos {
			responseTodos = append(responseTodos, map[string]interface{}{
				"id":       item.ID,
				"content":  item.Content,
				"status":   string(item.Status),
				"priority": string(item.Priority),
			})
		}

		return map[string]any{
			"success": true,
			"message": fmt.Sprintf("Successfully updated %d todo(s)", len(todos)),
			"todos":   responseTodos,
		}, nil
	}
}

// FormatOutput formats the tool's execution result for user display
func (t *TodoWriteTool) FormatOutput(result map[string]interface{}) string {
	success, ok := result["success"].(bool)
	if !ok {
		return "Error: Invalid response format"
	}
	
	message, ok := result["message"].(string)
	if !ok {
		message = "Unknown result"
	}
	
	todosInterface, todosOk := result["todos"].([]interface{})
	
	output := message

	if success && todosOk {
		if len(todosInterface) > 0 {
			output += ".\nCurrent Todos:"
			for _, todoItem := range todosInterface {
				if todoMap, ok := todoItem.(map[string]interface{}); ok {
					content, _ := todoMap["content"].(string)
					status, _ := todoMap["status"].(string)
					priority, _ := todoMap["priority"].(string)
					
					checkbox := "[ ]"
					if status == string(StatusCompleted) {
						checkbox = "[x]"
					}
					output += fmt.Sprintf("\n- %s %s (%s)", checkbox, content, capitalizeFirstLetter(priority))
				}
			}
		}
	} else if !success {
		output = fmt.Sprintf("Error: %s", message)
	}
	
	return output
}

func capitalizeFirstLetter(s string) string {
    if len(s) == 0 {
        return ""
    }
    return strings.ToUpper(s[:1]) + s[1:]
}
