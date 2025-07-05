package ctx

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/tools"
)

// TodoContextPartProvider manages todo context by listening to TodoWrite executions
type TodoContextPartProvider struct {
	eventBus events.EventBus
	mu       sync.RWMutex
	todos    []tools.TodoItem
}

// NewTodoContextPartProvider creates a new todo context provider
func NewTodoContextPartProvider(eventBus events.EventBus) *TodoContextPartProvider {
	provider := &TodoContextPartProvider{
		eventBus: eventBus,
		todos:    make([]tools.TodoItem, 0),
	}

	if eventBus != nil {
		eventBus.Subscribe("tool.executed", provider.handleToolExecutedEvent)
	}

	return provider
}

// handleToolExecutedEvent handles tool.executed events
func (p *TodoContextPartProvider) handleToolExecutedEvent(event interface{}) {
	toolEvent, ok := event.(events.ToolExecutedEvent)
	if !ok {
		return
	}

	// Only handle TodoWrite tool executions
	if toolEvent.ToolName != "TodoWrite" {
		return
	}

	// Extract todos from the result
	// TodoWrite tool returns: {"success": bool, "message": string}
	// But we need to get the todos from the parameters (what was written)
	todosParam, ok := toolEvent.Parameters["todos"]
	if !ok {
		return
	}

	// Convert the interface{} to []tools.TodoItem
	todosArray, ok := todosParam.([]interface{})
	if !ok {
		return
	}

	// Parse the todos
	todos := make([]tools.TodoItem, 0, len(todosArray))
	for _, todoInterface := range todosArray {
		todoMap, ok := todoInterface.(map[string]interface{})
		if !ok {
			continue
		}

		id, idOk := todoMap["id"].(string)
		content, contentOk := todoMap["content"].(string)
		status, statusOk := todoMap["status"].(string)
		priority, priorityOk := todoMap["priority"].(string)

		if !idOk || !contentOk || !statusOk || !priorityOk {
			continue
		}

		todos = append(todos, tools.TodoItem{
			ID:       id,
			Content:  content,
			Status:   tools.Status(status),
			Priority: tools.Priority(priority),
		})
	}

	// Update the stored todos
	p.mu.Lock()
	p.todos = todos
	p.mu.Unlock()
}

// GetPart returns the current todos as context
func (p *TodoContextPartProvider) GetPart(ctx context.Context) (ContextPart, error) {
	p.mu.RLock()
	todos := make([]tools.TodoItem, len(p.todos))
	copy(todos, p.todos)
	p.mu.RUnlock()

	if len(todos) == 0 {
		return ContextPart{
			Key:     "todo",
			Content: "",
		}, nil
	}

	// Format todos as a readable context
	var content strings.Builder
	content.WriteString("# Current Todo List\n\n")

	// Group by status
	pendingTodos := make([]tools.TodoItem, 0)
	inProgressTodos := make([]tools.TodoItem, 0)
	completedTodos := make([]tools.TodoItem, 0)

	for _, todo := range todos {
		switch todo.Status {
		case tools.StatusPending:
			pendingTodos = append(pendingTodos, todo)
		case tools.StatusInProgress:
			inProgressTodos = append(inProgressTodos, todo)
		case tools.StatusCompleted:
			completedTodos = append(completedTodos, todo)
		}
	}

	// Write in-progress todos first (most important)
	if len(inProgressTodos) > 0 {
		content.WriteString("## In Progress\n")
		for _, todo := range inProgressTodos {
			content.WriteString(fmt.Sprintf("- **[%s]** %s (%s priority)\n", 
				todo.ID, todo.Content, todo.Priority))
		}
		content.WriteString("\n")
	}

	// Then pending todos
	if len(pendingTodos) > 0 {
		content.WriteString("## Pending\n")
		for _, todo := range pendingTodos {
			content.WriteString(fmt.Sprintf("- [%s] %s (%s priority)\n", 
				todo.ID, todo.Content, todo.Priority))
		}
		content.WriteString("\n")
	}

	// Finally completed todos (for reference)
	if len(completedTodos) > 0 {
		content.WriteString("## Completed\n")
		for _, todo := range completedTodos {
			content.WriteString(fmt.Sprintf("- ✓ [%s] %s (%s priority)\n", 
				todo.ID, todo.Content, todo.Priority))
		}
	}

	return ContextPart{
		Key:     "todo",
		Content: strings.TrimSpace(content.String()),
	}, nil
}

// ClearPart clears the stored todos
func (p *TodoContextPartProvider) ClearPart() error {
	p.mu.Lock()
	p.todos = make([]tools.TodoItem, 0)
	p.mu.Unlock()
	return nil
}

// GetTodos returns the current todos (for testing)
func (p *TodoContextPartProvider) GetTodos() []tools.TodoItem {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	todos := make([]tools.TodoItem, len(p.todos))
	copy(todos, p.todos)
	return todos
}