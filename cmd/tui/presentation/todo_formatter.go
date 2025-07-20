package presentation

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/tui/types"
)

// TodoFormatter handles formatting of todo tool results for display in the TUI
type TodoFormatter struct {
	theme *types.Theme
}

// NewTodoFormatter creates a new TodoFormatter instance
func NewTodoFormatter(theme *types.Theme) *TodoFormatter {
	return &TodoFormatter{
		theme: theme,
	}
}

// TodoItem represents a single todo item (matching the structure from tools package)
type TodoItem struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

// FormatTodoList formats a list of todos for display
func (f *TodoFormatter) FormatTodoList(todos []TodoItem) string {
	if len(todos) == 0 {
		return f.formatEmptyList()
	}

	var output strings.Builder

	// Group todos by status
	pending := make([]TodoItem, 0)
	inProgress := make([]TodoItem, 0)
	completed := make([]TodoItem, 0)

	for _, todo := range todos {
		switch todo.Status {
		case "pending":
			pending = append(pending, todo)
		case "in_progress":
			inProgress = append(inProgress, todo)
		case "completed":
			completed = append(completed, todo)
		}
	}

	// Format header
	totalCount := len(todos)
	completedCount := len(completed)
	output.WriteString(f.formatHeader(totalCount, completedCount))

	// Format completed todos first
	for _, todo := range completed {
		output.WriteString(f.formatTodoItem(todo))
	}

	// Then in-progress todos
	for _, todo := range inProgress {
		output.WriteString(f.formatTodoItem(todo))
	}

	// Finally pending todos
	for _, todo := range pending {
		output.WriteString(f.formatTodoItem(todo))
	}

	return strings.TrimSpace(output.String())
}

// FormatTodoToolResult formats the result from TodoWrite tools
func (f *TodoFormatter) FormatTodoToolResult(result map[string]interface{}) string {
	// Handle TodoWrite tool results
	success, ok := result["success"].(bool)
	if !ok || !success {
		errorMsg, _ := result["message"].(string)
		if errorMsg == "" {
			errorMsg = "Unknown error occurred"
		}
		return f.formatError(errorMsg)
	}

	// Extract todos from result
	todosInterface, ok := result["todos"]
	if !ok {
		return f.formatError("No todos found in result")
	}

	todos, err := f.parseTodos(todosInterface)
	if err != nil {
		return f.formatError(fmt.Sprintf("Error parsing todos: %v", err))
	}

	// TodoWrite returns the list
	return f.FormatTodoList(todos)
}

// formatHeader creates a summary header for the todo list
func (f *TodoFormatter) formatHeader(total, completed int) string {
	// Return empty string - no header needed
	return ""
}

// formatTodoItem formats a single todo item with checkbox
func (f *TodoFormatter) formatTodoItem(todo TodoItem) string {
	checkbox := f.getCheckbox(todo.Status)
	resetColor := "\033[0m"

	// Apply different colors based on status
	var statusColor string
	switch todo.Status {
	case "completed":
		statusColor = ConvertColorToAnsi(f.theme.Primary) // Primary color for completed
	case "in_progress":
		statusColor = ConvertColorToAnsi(f.theme.Secondary) // Secondary color for in progress
	case "pending":
		statusColor = "" // No color for pending (default white)
	default:
		statusColor = ""
	}

	return fmt.Sprintf("%s%s %s%s\n",
		statusColor,
		checkbox,
		todo.Content,
		resetColor)
}

// formatEmptyList formats display for when there are no todos
func (f *TodoFormatter) formatEmptyList() string {
	mutedColor := ConvertColorToAnsi(f.theme.Muted)
	resetColor := "\033[0m"

	return fmt.Sprintf("%sNo todos found.\n\nUse TodoWrite to create your first todo item!%s",
		mutedColor, resetColor)
}

// formatError formats error messages
func (f *TodoFormatter) formatError(message string) string {
	errorColor := ConvertColorToAnsi(f.theme.Error)
	resetColor := "\033[0m"

	return fmt.Sprintf("%sError: %s%s", errorColor, message, resetColor)
}

// formatSuccessMessage formats success messages for TodoWrite operations
func (f *TodoFormatter) formatSuccessMessage(message string) string {
	primaryColor := ConvertColorToAnsi(f.theme.Primary)
	resetColor := "\033[0m"

	return fmt.Sprintf("%s%s%s", primaryColor, message, resetColor)
}

// getCheckbox returns the appropriate checkbox for a status
func (f *TodoFormatter) getCheckbox(status string) string {
	switch status {
	case "pending":
		return "[ ]"
	case "in_progress":
		return "[~]"
	case "completed":
		return "[x]"
	default:
		return "[?]"
	}
}

// parseTodos converts interface{} todos to []TodoItem
func (f *TodoFormatter) parseTodos(todosInterface interface{}) ([]TodoItem, error) {
	switch v := todosInterface.(type) {
	case []map[string]interface{}:
		todos := make([]TodoItem, 0, len(v))
		for _, todoMap := range v {
			todo := TodoItem{}
			if id, exists := todoMap["id"].(string); exists {
				todo.ID = id
			}
			if content, exists := todoMap["content"].(string); exists {
				todo.Content = content
			}
			if status, exists := todoMap["status"].(string); exists {
				todo.Status = status
			}
			if priority, exists := todoMap["priority"].(string); exists {
				todo.Priority = priority
			}
			todos = append(todos, todo)
		}
		return todos, nil
	default:
		return nil, fmt.Errorf("unexpected todo format: %T", todosInterface)
	}
}

