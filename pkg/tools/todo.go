package tools

import (
	"fmt"
	"strings"
)

// Status represents the current state of a todo item
type Status string

// Priority represents the importance level of a todo item
type Priority string

// Status constants as defined in the specification
const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
)

// Priority constants as defined in the specification
const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// TodoItem represents a single todo item with all required fields
type TodoItem struct {
	ID       string   `json:"id"`
	Content  string   `json:"content"`
	Status   Status   `json:"status"`
	Priority Priority `json:"priority"`
}

// Validate checks if the TodoItem meets all specification requirements
func (t *TodoItem) Validate() error {
	if strings.TrimSpace(t.ID) == "" {
		return fmt.Errorf("id is required and cannot be empty")
	}

	if strings.TrimSpace(t.Content) == "" {
		return fmt.Errorf("content is required and cannot be empty")
	}

	if !t.Status.IsValid() {
		return fmt.Errorf("status must be one of: pending, in_progress, completed")
	}

	if !t.Priority.IsValid() {
		return fmt.Errorf("priority must be one of: high, medium, low")
	}

	return nil
}

// IsValid checks if the status is one of the allowed values
func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusInProgress, StatusCompleted:
		return true
	default:
		return false
	}
}

// IsValid checks if the priority is one of the allowed values
func (p Priority) IsValid() bool {
	switch p {
	case PriorityHigh, PriorityMedium, PriorityLow:
		return true
	default:
		return false
	}
}

// TodoManager interface defines operations for managing todos
type TodoManager interface {
	// Read returns the current list of todos
	Read() []TodoItem
	
	// Write replaces the entire todo list with the provided items
	// Returns error if any todo item fails validation or if there are duplicate IDs
	Write(todos []TodoItem) error
}

// InMemoryTodoManager implements TodoManager using in-memory storage
type InMemoryTodoManager struct {
	todos []TodoItem
}

// NewTodoManager creates a new TodoManager instance
func NewTodoManager() TodoManager {
	return &InMemoryTodoManager{
		todos: make([]TodoItem, 0),
	}
}

// Read returns a copy of the current todo list
func (m *InMemoryTodoManager) Read() []TodoItem {
	// Return a copy to prevent external modification
	result := make([]TodoItem, len(m.todos))
	copy(result, m.todos)
	return result
}

// Write replaces the entire todo list with validation
func (m *InMemoryTodoManager) Write(todos []TodoItem) error {
	// Validate all todos first
	for i, todo := range todos {
		if err := todo.Validate(); err != nil {
			return fmt.Errorf("todo at index %d is invalid: %w", i, err)
		}
	}
	
	// Check for duplicate IDs
	idSet := make(map[string]bool)
	for i, todo := range todos {
		if idSet[todo.ID] {
			return fmt.Errorf("duplicate ID '%s' found at index %d", todo.ID, i)
		}
		idSet[todo.ID] = true
	}
	
	// If all validation passes, replace the list
	m.todos = make([]TodoItem, len(todos))
	copy(m.todos, todos)
	
	return nil
}