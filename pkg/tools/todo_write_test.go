package tools

import (
	"context"
	"testing"
	"reflect"

	"github.com/kcaldas/genie/pkg/ai"
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

	// New checks for 'todos' in response
	todosProp, ok := decl.Response.Properties["todos"]
	if !ok {
		t.Error("Expected 'todos' property in response schema")
	}
	if todosProp.Type != ai.TypeArray {
		t.Errorf("Expected 'todos' to be of type 'array', got '%s'", string(todosProp.Type))
	}
	if todosProp.Items == nil {
		t.Error("Expected 'items' schema for 'todos' array")
	}
	if todosProp.Items.Type != ai.TypeObject {
		t.Errorf("Expected 'todos' items to be of type 'object', got '%s'", string(todosProp.Items.Type))
	}
	
	expectedItemProps := map[string]bool{
		"id": true, "content": true, "status": true, "priority": true,
	}
	if todosProp.Items.Properties == nil {
		t.Error("Expected properties for 'todos' items")
	}
	for propName := range expectedItemProps {
		if _, exists := todosProp.Items.Properties[propName]; !exists {
			t.Errorf("Expected property '%s' in 'todos' item schema", propName)
		}
	}

	expectedRequiredItemFields := map[string]bool{
		"id": true, "content": true, "status": true, "priority": true,
	}
	if todosProp.Items.Required == nil || len(todosProp.Items.Required) != len(expectedRequiredItemFields) {
		t.Errorf("Expected %d required fields for 'todos' items, got %d", len(expectedRequiredItemFields), len(todosProp.Items.Required))
	}
	for _, requiredField := range decl.Response.Required {
		if _, exists := expectedRequiredItemFields[requiredField]; !exists && requiredField != "success" && requiredField != "message" && requiredField != "todos" {
			t.Errorf("Unexpected required field '%s' in 'todos' item schema", requiredField)
		}
	}

	// Ensure success and message are still required
	if !contains(decl.Response.Required, "success") {
		t.Error("Expected 'success' to be a required response field")
	}
	if !contains(decl.Response.Required, "message") {
		t.Error("Expected 'message' to be a required response field")
	}
	if !contains(decl.Response.Required, "todos") {
		t.Error("Expected 'todos' to be a required response field")
	}
}

func TestTodoWriteTool_Handler_ValidTodos(t *testing.T) {
	manager := NewTodoManager()
	tool := NewTodoWriteTool(manager)
	
	expectedTodos := []map[string]interface{}{
		{
			"id":       "1",
			"content":  "Test task 1",
			"status":   "pending",
			"priority": "high",
		},
		{
			"id":       "2",	
			"content":  "Test task 2",
			"status":   "in_progress",
			"priority": "medium",
		},
	}

	// Convert expectedTodos to []any for the handler
	var todosForHandler []any
	for _, todo := range expectedTodos {
		todosForHandler = append(todosForHandler, todo)
	}

	params := map[string]any{
		"todos": todosForHandler,
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
	
	// Verify returned todos
	returnedTodos, ok := result["todos"].([]map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'todos' in response to be an array of map[string]interface{}, got %T", result["todos"])
	}
	
	if len(returnedTodos) != len(expectedTodos) {
		t.Errorf("Expected %d returned todos, got %d", len(expectedTodos), len(returnedTodos))
	}

	if !reflect.DeepEqual(returnedTodos, expectedTodos) {
		t.Errorf("Returned todos do not match expected.\nExpected: %+v\nGot: %+v", expectedTodos, returnedTodos)
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
				// Ensure 'todos' is not present on failure or is an empty array
				if _, ok := result["todos"]; ok {
					if todosArr, isArr := result["todos"].([]interface{}); !isArr || len(todosArr) != 0 {
						t.Errorf("Expected 'todos' to be absent or empty array on failure for %s, got %v", tt.name, result["todos"])
					}
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
	
	expectedTodos := []map[string]interface{}{
		{
			"id":       "new1",
			"content":  "New task 1",
			"status":   "pending",
			"priority": "high",
		},
		{
			"id":       "new2",
			"content":  "New task 2",
			"status":   "completed",
			"priority": "medium",
		},
	}

	// Convert expectedTodos to []any for the handler
	var todosForHandler []any
	for _, todo := range expectedTodos {
		todosForHandler = append(todosForHandler, todo)
	}

	params := map[string]any{
		"todos": todosForHandler,
	}
	
	handler := tool.Handler()
	result, err := handler(context.Background(), params)
	
	if err != nil {
		t.Errorf("Handler failed: %v", err)
	}
	
	// Verify the list was completely replaced in the response
	returnedTodos, ok := result["todos"].([]map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'todos' in response to be an array of map[string]interface{}, got %T", result["todos"])
	}

	if len(returnedTodos) != len(expectedTodos) {
		t.Errorf("Expected %d returned todos, got %d", len(expectedTodos), len(returnedTodos))
	}

	if !reflect.DeepEqual(returnedTodos, expectedTodos) {
		t.Errorf("Returned todos do not match expected.\nExpected: %+v\nGot: %+v", expectedTodos, returnedTodos)
	}

	// Verify todos were actually written to the manager
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
				"message": "Successfully updated 1 todo(s)",
				"todos": []interface{}{
					map[string]interface{}{"id": "1", "content": "Task 1", "status": "pending", "priority": "high"},
				},
			},
			want: `Successfully updated 1 todo(s).
Current Todos:
- [ ] Task 1 (High)`,
		},
		{
			name: "success with multiple todos",
			result: map[string]interface{}{
				"success": true,
				"message": "Successfully updated 2 todo(s)",
				"todos": []interface{}{
					map[string]interface{}{"id": "1", "content": "Task 1", "status": "pending", "priority": "high"},
					map[string]interface{}{"id": "2", "content": "Task 2", "status": "completed", "priority": "low"},
				},
			},
			want: `Successfully updated 2 todo(s).
Current Todos:
- [ ] Task 1 (High)
- [x] Task 2 (Low)`,
		},
		{
			name: "failure", 
			result: map[string]interface{}{
				"success": false,
				"message": "Validation failed",
				"todos": []interface{}{}, // Should be empty or absent on failure
			},
			want: "Error: Validation failed",
		},
		{
			name: "failure no todos key", 
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

// Helper to check if a string is in a slice
func contains(s []string, e string) bool {
    for _, a := range s {
        if a == e {
            return true
        }
    }
    return false
}
