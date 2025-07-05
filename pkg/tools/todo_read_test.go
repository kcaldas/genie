package tools

import (
	"context"
	"testing"
)

func TestTodoReadTool_Declaration(t *testing.T) {
	tool := NewTodoReadTool(nil)
	decl := tool.Declaration()
	
	if decl.Name != "TodoRead" {
		t.Errorf("Expected name 'TodoRead', got '%s'", decl.Name)
	}
	
	if decl.Description == "" {
		t.Error("Expected non-empty description")
	}
	
	// Should have no required parameters
	if decl.Parameters != nil {
		if len(decl.Parameters.Required) != 0 {
			t.Errorf("Expected no required parameters, got %v", decl.Parameters.Required)
		}
	}
	
	// Should have response schema with array of todos
	if decl.Response == nil {
		t.Error("Expected response schema")
	}
}

func TestTodoReadTool_Handler_EmptyList(t *testing.T) {
	manager := NewTodoManager()
	tool := NewTodoReadTool(manager)
	
	handler := tool.Handler()
	
	result, err := handler(context.Background(), map[string]any{})
	if err != nil {
		t.Errorf("Handler failed: %v", err)
	}
	
	// Check response structure
	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("Expected success=true, got %v", result["success"])
	}
	
	todosInterface, ok := result["todos"].([]map[string]interface{})
	if !ok {
		t.Errorf("Expected todos array of maps, got %T", result["todos"])
	}
	
	if len(todosInterface) != 0 {
		t.Errorf("Expected empty todos array, got %d items", len(todosInterface))
	}
}

func TestTodoReadTool_Handler_WithTodos(t *testing.T) {
	manager := NewTodoManager()
	
	// Pre-populate the manager with todos
	testTodos := []TodoItem{
		{
			ID:       "1",
			Content:  "Test task 1",
			Status:   StatusPending,
			Priority: PriorityHigh,
		},
		{
			ID:       "2",
			Content:  "Test task 2", 
			Status:   StatusInProgress,
			Priority: PriorityMedium,
		},
	}
	
	err := manager.Write(testTodos)
	if err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}
	
	tool := NewTodoReadTool(manager)
	handler := tool.Handler()
	
	result, err := handler(context.Background(), map[string]any{})
	if err != nil {
		t.Errorf("Handler failed: %v", err)
	}
	
	// Check response structure
	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("Expected success=true, got %v", result["success"])
	}
	
	todosInterface, ok := result["todos"].([]map[string]interface{})
	if !ok {
		t.Errorf("Expected todos array of maps, got %T", result["todos"])
	}
	
	if len(todosInterface) != 2 {
		t.Errorf("Expected 2 todos, got %d", len(todosInterface))
	}
	
	// Verify the todos match what we stored
	if todosInterface[0]["id"] != "1" || todosInterface[0]["content"] != "Test task 1" {
		t.Errorf("First todo incorrect: %+v", todosInterface[0])
	}
	
	if todosInterface[1]["id"] != "2" || todosInterface[1]["content"] != "Test task 2" {
		t.Errorf("Second todo incorrect: %+v", todosInterface[1])
	}
}

func TestTodoReadTool_FormatOutput(t *testing.T) {
	tool := NewTodoReadTool(nil)
	
	tests := []struct {
		name   string
		result map[string]interface{}
		want   string
	}{
		{
			name: "empty list",
			result: map[string]interface{}{
				"success": true,
				"todos":   []TodoItem{},
			},
			want: "No todos found.",
		},
		{
			name: "with todos",
			result: map[string]interface{}{
				"success": true,
				"todos": []TodoItem{
					{
						ID:       "1",
						Content:  "Test task",
						Status:   StatusPending,
						Priority: PriorityHigh,
					},
				},
			},
			want: "Found 1 todo:\n- [1] Test task (pending, high)",
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