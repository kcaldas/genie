package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
)

// TodoReadTool implements the TodoRead tool as specified
type TodoReadTool struct {
	manager TodoManager
}

// NewTodoReadTool creates a new TodoRead tool
func NewTodoReadTool(manager TodoManager) Tool {
	return &TodoReadTool{
		manager: manager,
	}
}

// Declaration returns the function declaration for the TodoRead tool
func (t *TodoReadTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "TodoRead",
		Description: "Read the current session todo list. Shows major milestones and their progress. No parameters needed.",
		Parameters: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "No parameters required - leave empty",
			Properties:  map[string]*ai.Schema{},
			Required:    []string{},
		},
		Response: &ai.Schema{
			Type:        ai.TypeObject,
			Description: "Current todo list with all items",
			Properties: map[string]*ai.Schema{
				"success": {
					Type:        ai.TypeBoolean,
					Description: "Whether the operation was successful",
				},
				"todos": {
					Type:        ai.TypeArray,
					Description: "Array of current todo items",
					Items: &ai.Schema{
						Type:        ai.TypeObject,
						Description: "A single todo item",
						Properties: map[string]*ai.Schema{
							"id": {
								Type:        ai.TypeString,
								Description: "Unique identifier for the todo item",
							},
							"content": {
								Type:        ai.TypeString,
								Description: "Task description",
							},
							"status": {
								Type:        ai.TypeString,
								Description: "Current state: pending, in_progress, or completed",
							},
							"priority": {
								Type:        ai.TypeString,
								Description: "Importance level: high, medium, or low",
							},
						},
						Required: []string{"id", "content", "status", "priority"},
					},
				},
			},
			Required: []string{"success", "todos"},
		},
	}
}

// Handler returns the function handler for the TodoRead tool
func (t *TodoReadTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		// TodoRead takes no parameters, so we simply read from the manager
		todos := t.manager.Read()
		
		// Convert todos to generic maps for consistent JSON handling
		var todoMaps []map[string]interface{}
		for _, todo := range todos {
			todoMaps = append(todoMaps, map[string]interface{}{
				"id":       todo.ID,
				"content":  todo.Content,
				"status":   string(todo.Status),
				"priority": string(todo.Priority),
			})
		}
		
		return map[string]any{
			"success": true,
			"todos":   todoMaps,
		}, nil
	}
}

// FormatOutput formats the tool's execution result for user display
func (t *TodoReadTool) FormatOutput(result map[string]interface{}) string {
	todos, ok := result["todos"].([]TodoItem)
	if !ok {
		return "Error: Invalid todo list format"
	}
	
	if len(todos) == 0 {
		return "No todos found."
	}
	
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d todo", len(todos)))
	if len(todos) > 1 {
		output.WriteString("s")
	}
	output.WriteString(":\n")
	
	for _, todo := range todos {
		output.WriteString(fmt.Sprintf("- [%s] %s (%s, %s)\n", 
			todo.ID, todo.Content, todo.Status, todo.Priority))
	}
	
	// Remove trailing newline
	formatted := output.String()
	return strings.TrimSuffix(formatted, "\n")
}