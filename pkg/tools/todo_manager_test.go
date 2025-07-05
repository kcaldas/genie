package tools

import (
	"testing"
)

func TestTodoManager_Read_EmptyList(t *testing.T) {
	manager := NewTodoManager()
	
	todos := manager.Read()
	
	if len(todos) != 0 {
		t.Errorf("Expected empty list, got %d items", len(todos))
	}
}

func TestTodoManager_Write_ValidTodos(t *testing.T) {
	manager := NewTodoManager()
	
	todos := []TodoItem{
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
	
	err := manager.Write(todos)
	if err != nil {
		t.Errorf("Write() failed with valid todos: %v", err)
	}
	
	// Verify the todos were stored correctly
	stored := manager.Read()
	if len(stored) != 2 {
		t.Errorf("Expected 2 todos, got %d", len(stored))
	}
	
	if stored[0].ID != "1" || stored[0].Content != "Test task 1" {
		t.Errorf("First todo not stored correctly: %+v", stored[0])
	}
	
	if stored[1].ID != "2" || stored[1].Content != "Test task 2" {
		t.Errorf("Second todo not stored correctly: %+v", stored[1])
	}
}

func TestTodoManager_Write_InvalidTodos(t *testing.T) {
	manager := NewTodoManager()
	
	tests := []struct {
		name  string
		todos []TodoItem
	}{
		{
			name: "empty content",
			todos: []TodoItem{
				{
					ID:       "1",
					Content:  "",
					Status:   StatusPending,
					Priority: PriorityHigh,
				},
			},
		},
		{
			name: "invalid status",
			todos: []TodoItem{
				{
					ID:       "1",
					Content:  "Test task",
					Status:   "invalid",
					Priority: PriorityHigh,
				},
			},
		},
		{
			name: "duplicate IDs",
			todos: []TodoItem{
				{
					ID:       "1",
					Content:  "Test task 1",
					Status:   StatusPending,
					Priority: PriorityHigh,
				},
				{
					ID:       "1",
					Content:  "Test task 2",
					Status:   StatusPending,
					Priority: PriorityMedium,
				},
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.Write(tt.todos)
			if err == nil {
				t.Errorf("Write() should have failed for %s", tt.name)
			}
		})
	}
}

func TestTodoManager_Write_ReplacesExistingList(t *testing.T) {
	manager := NewTodoManager()
	
	// Write initial todos
	initial := []TodoItem{
		{
			ID:       "1",
			Content:  "Initial task",
			Status:   StatusPending,
			Priority: PriorityHigh,
		},
	}
	
	err := manager.Write(initial)
	if err != nil {
		t.Fatalf("Initial write failed: %v", err)
	}
	
	// Write replacement todos
	replacement := []TodoItem{
		{
			ID:       "2",
			Content:  "Replacement task 1",
			Status:   StatusInProgress,
			Priority: PriorityMedium,
		},
		{
			ID:       "3",
			Content:  "Replacement task 2", 
			Status:   StatusCompleted,
			Priority: PriorityLow,
		},
	}
	
	err = manager.Write(replacement)
	if err != nil {
		t.Fatalf("Replacement write failed: %v", err)
	}
	
	// Verify the list was completely replaced
	stored := manager.Read()
	if len(stored) != 2 {
		t.Errorf("Expected 2 todos after replacement, got %d", len(stored))
	}
	
	// Verify the old todo is gone
	for _, todo := range stored {
		if todo.ID == "1" {
			t.Errorf("Old todo should have been replaced but was found: %+v", todo)
		}
	}
	
	// Verify new todos are present
	expectedIDs := map[string]bool{"2": false, "3": false}
	for _, todo := range stored {
		if _, exists := expectedIDs[todo.ID]; exists {
			expectedIDs[todo.ID] = true
		}
	}
	
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("Expected todo with ID %s not found after replacement", id)
		}
	}
}