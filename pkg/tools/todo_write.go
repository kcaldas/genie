package tools

import (
	"context"
	"fmt"

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
		Description: "Create and manage the structured task list. Replaces the entire todo list with the provided items. Validates all required fields and enforces enum constraints on status/priority.",
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
			},
			Required: []string{"success", "message"},
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
			}, fmt.Errorf("missing required parameter 'todos'")
		}
		
		// Convert to array of maps
		todosArray, ok := todosParam.([]interface{})
		if !ok {
			return map[string]any{
				"success": false,
				"message": "Parameter 'todos' must be an array",
			}, fmt.Errorf("parameter 'todos' must be an array")
		}
		
		// Convert to TodoItem structs
		todos := make([]TodoItem, len(todosArray))
		for i, todoInterface := range todosArray {
			todoMap, ok := todoInterface.(map[string]interface{})
			if !ok {
				return map[string]any{
					"success": false,
					"message": fmt.Sprintf("Todo at index %d is not a valid object", i),
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
			}, err
		}
		
		return map[string]any{
			"success": true,
			"message": fmt.Sprintf("Successfully updated %d todo(s)", len(todos)),
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
	
	if success {
		return message
	} else {
		return fmt.Sprintf("Error: %s", message)
	}
}