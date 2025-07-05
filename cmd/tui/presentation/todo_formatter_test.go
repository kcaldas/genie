package presentation

import (
	"testing"

	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/stretchr/testify/assert"
)

func TestTodoFormatter_FormatTodoList(t *testing.T) {
	theme := &types.Theme{
		Primary:       "#00FF00",
		Secondary:     "#FFFF00", 
		Error:         "#FF0000",
		Muted:         "#808080",
		TextPrimary:   "#FFFFFF",
	}
	formatter := NewTodoFormatter(theme)

	t.Run("empty todo list", func(t *testing.T) {
		result := formatter.FormatTodoList([]TodoItem{})
		assert.Contains(t, result, "No todos found")
		assert.Contains(t, result, "TodoWrite")
	})

	t.Run("single todo item", func(t *testing.T) {
		todos := []TodoItem{
			{
				ID:       "1",
				Content:  "Test task",
				Status:   "pending",
				Priority: "high",
			},
		}
		result := formatter.FormatTodoList(todos)
		assert.Contains(t, result, "[ ]")
		assert.Contains(t, result, "Test task")
	})

	t.Run("multiple todos with different statuses", func(t *testing.T) {
		todos := []TodoItem{
			{ID: "1", Content: "Pending task", Status: "pending", Priority: "high"},
			{ID: "2", Content: "In progress task", Status: "in_progress", Priority: "medium"},
			{ID: "3", Content: "Completed task", Status: "completed", Priority: "low"},
		}
		result := formatter.FormatTodoList(todos)
		
		// Check checkboxes are present (order is maintained: completed, in_progress, pending)
		assert.Contains(t, result, "[ ]") // pending
		assert.Contains(t, result, "[~]") // in_progress
		assert.Contains(t, result, "[x]") // completed
	})
}

func TestTodoFormatter_Checkboxes(t *testing.T) {
	theme := &types.Theme{}
	formatter := NewTodoFormatter(theme)

	tests := []struct {
		status   string
		expected string
	}{
		{"pending", "[ ]"},
		{"in_progress", "[~]"},
		{"completed", "[x]"},
		{"unknown", "[?]"},
	}

	for _, test := range tests {
		t.Run(test.status, func(t *testing.T) {
			result := formatter.getCheckbox(test.status)
			assert.Equal(t, test.expected, result)
		})
	}
}