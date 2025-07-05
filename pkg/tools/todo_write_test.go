package tools

import (
	"context"
	"testing"
)

func TestTodoWriteTool_Declaration(t *testing.T) {
	tool := NewTodoWriteTool(nil)
	decl := tool.Declaration()
	
	if decl.Name != "TodoWrite" {
		t.Errorf("Expected name 'TodoWrite', got '%s'", decl.Name)
	}
	
	if decl.Description == "" {
		t.Error("Expected non-empty description")
	}
	
	// Should require todos parameter
	if decl.Parameters == nil || len(decl.Parameters.Required) == 0 {
		t.Error("Expected required parameters")
	}
	
	// Should have response schema
	if decl.Response == nil {
		t.Error("Expected response schema")
	}
}

func TestTodoWriteTool_Handler_ValidTodos(t *testing.T) {
	manager := NewTodoManager()
	tool := NewTodoWriteTool(manager)
	
	params := map[string]any{
		"todos": []interface{}{
			map[string]interface{}{
				"id":       "1",
				"content":  "Test task 1",
				"status":   "pending",
				"priority": "high",
			},
			map[string]interface{}{
				"id":       "2",
				"content":  "Test task 2",
				"status":   "in_progress",
				"priority": "medium",
			},
		},
	}
	
	handler := tool.Handler()
	result, err := handler(context.Background(), params)
	
	if err != nil {
		t.Errorf("Handler failed: %v", err)
	}
	
	// Check response structure
	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("Expected success=true, got %v", result["success"])
	}
	
	if message, ok := result["message"].(string); !ok || message == "" {
		t.Errorf("Expected non-empty message, got %v", result["message"])
	}
	
	// Verify todos were actually written to the manager
	todos := manager.Read()
	if len(todos) != 2 {
		t.Errorf("Expected 2 todos in manager, got %d", len(todos))
	}
}

func TestTodoWriteTool_Handler_InvalidTodos(t *testing.T) {
	manager := NewTodoManager()
	tool := NewTodoWriteTool(manager)
	
	tests := []struct {
		name   string
		params map[string]any
	}{
		{
			name: "missing todos parameter",
			params: map[string]any{},
		},
		{
			name: "invalid todos type",
			params: map[string]any{
				"todos": "not an array",
			},
		},
		{
			name: "empty content in todo",
			params: map[string]any{
				"todos": []interface{}{
					map[string]interface{}{
						"id":       "1",
						"content":  "",
						"status":   "pending",
						"priority": "high",
					},
				},
			},
		},
		{
			name: "invalid status",
			params: map[string]any{
				"todos": []interface{}{
					map[string]interface{}{
						"id":       "1",
						"content":  "Test task",
						"status":   "invalid",
						"priority": "high",
					},
				},
			},
		},
		{
			name: "duplicate IDs",
			params: map[string]any{
				"todos": []interface{}{
					map[string]interface{}{
						"id":       "1",
						"content":  "Test task 1",
						"status":   "pending",
						"priority": "high",
					},
					map[string]interface{}{
						"id":       "1",
						"content":  "Test task 2",
						"status":   "pending",
						"priority": "medium",
					},
				},
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := tool.Handler()
			result, err := handler(context.Background(), tt.params)
			
			if err == nil {
				t.Errorf("Expected error for %s", tt.name)
			}
			
			// Check that result indicates failure
			if result != nil {
				if success, ok := result["success"].(bool); ok && success {
					t.Errorf("Expected success=false for %s", tt.name)
				}
			}
		})
	}
}

func TestTodoWriteTool_Handler_ReplacesExistingList(t *testing.T) {
	manager := NewTodoManager()
	
	// Pre-populate with initial todos
	initial := []TodoItem{
		{
			ID:       "old1",
			Content:  "Old task",
			Status:   StatusPending,
			Priority: PriorityLow,
		},
	}
	err := manager.Write(initial)
	if err != nil {
		t.Fatalf("Failed to setup initial data: %v", err)
	}
	
	tool := NewTodoWriteTool(manager)
	
	// Write new todos that should replace the old ones
	params := map[string]any{
		"todos": []interface{}{
			map[string]interface{}{
				"id":       "new1",
				"content":  "New task 1",
				"status":   "pending",
				"priority": "high",
			},
			map[string]interface{}{
				"id":       "new2",
				"content":  "New task 2",
				"status":   "completed",
				"priority": "medium",
			},
		},
	}
	
	handler := tool.Handler()
	_, err = handler(context.Background(), params)
	
	if err != nil {
		t.Errorf("Handler failed: %v", err)
	}
	
	// Verify the list was completely replaced
	todos := manager.Read()
	if len(todos) != 2 {
		t.Errorf("Expected 2 todos after replacement, got %d", len(todos))
	}
	
	// Verify old todo is gone
	for _, todo := range todos {
		if todo.ID == "old1" {
			t.Errorf("Old todo should have been replaced")
		}
	}
	
	// Verify new todos are present
	foundNew1, foundNew2 := false, false
	for _, todo := range todos {
		if todo.ID == "new1" {
			foundNew1 = true
		}
		if todo.ID == "new2" {
			foundNew2 = true
		}
	}
	
	if !foundNew1 || !foundNew2 {
		t.Errorf("New todos not found after replacement")
	}
}

func TestTodoWriteTool_FormatOutput(t *testing.T) {
	tool := NewTodoWriteTool(nil)
	
	tests := []struct {
		name   string
		result map[string]interface{}
		want   string
	}{
		{
			name: "success",
			result: map[string]interface{}{
				"success": true,
				"message": "Todos updated successfully",
			},
			want: "Todos updated successfully",
		},
		{
			name: "failure", 
			result: map[string]interface{}{
				"success": false,
				"message": "Validation failed",
			},
			want: "Error: Validation failed",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tool.FormatOutput(tt.result)
			if got != tt.want {
				t.Errorf("FormatOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}