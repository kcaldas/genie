package ctx

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/events"
)

// TodoContextPartProvider manages todo context by listening to TodoWrite executions
type TodoContextPartProvider struct {
	eventBus  events.EventBus
	mu        sync.RWMutex
	todosJSON string
}

// NewTodoContextPartProvider creates a new todo context provider
func NewTodoContextPartProvider(eventBus events.EventBus) *TodoContextPartProvider {
	provider := &TodoContextPartProvider{
		eventBus:  eventBus,
		todosJSON: "",
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

	// Extract todos from the result and marshal directly to JSON string
	resultTodos, ok := toolEvent.Result["todos"]
	if !ok {
		return
	}

	// Validate that resultTodos is a slice of maps before marshaling
	if _, isSliceOfMaps := resultTodos.([]map[string]interface{}); !isSliceOfMaps {
		// If it's not a slice of maps, it's invalid data for our context.
		// We should not update todosJSON with malformed data.
		return
	}

	// Marshal the todos directly to JSON string
	if todosJSON, err := json.MarshalIndent(resultTodos, "", "  "); err == nil {
		p.mu.Lock()
		p.todosJSON = string(todosJSON)
		p.mu.Unlock()
	}
}

func (p *TodoContextPartProvider) SetTokenBudget(int) {}

// GetPart returns the current todos as context
func (p *TodoContextPartProvider) GetPart(ctx context.Context) (ContextPart, error) {
	p.mu.RLock()
	todosJSON := p.todosJSON
	p.mu.RUnlock()

	if todosJSON == "" {
		return ContextPart{
			Key:     "todo",
			Content: "",
		}, nil
	}

	var todos []map[string]interface{}
	if err := json.Unmarshal([]byte(todosJSON), &todos); err != nil {
		return ContextPart{
			Key:     "todo",
			Content: fmt.Sprintf("Error unmarshaling todos: %v", err),
		}, nil
	}

	// Sort todos by status (completed first) then by priority: high > medium > low
	priorityOrder := map[string]int{
		"high":   0,
		"medium": 1,
		"low":    2,
	}

	sort.Slice(todos, func(i, j int) bool {
		statusI := todos[i]["status"].(string)
		statusJ := todos[j]["status"].(string)

		// Completed tasks come first
		if statusI == "completed" && statusJ != "completed" {
			return true
		}
		if statusJ == "completed" && statusI != "completed" {
			return false
		}

		// If both have the same completion status, sort by priority
		p1 := todos[i]["priority"].(string)
		p2 := todos[j]["priority"].(string)
		return priorityOrder[p1] < priorityOrder[p2]
	})

	var sb strings.Builder
	for _, todo := range todos {
		status := todo["status"].(string)
		content := todo["content"].(string)
		marker := ""
		switch status {
		case "completed":
			marker = "[x]"
		case "in_progress":
			marker = "[~]"
		case "pending":
			marker = "[ ]"
		default:
			marker = "[?]" // Unknown status
		}
		sb.WriteString(fmt.Sprintf("%s %s\n", marker, content))
	}

	return ContextPart{
		Key:     "todo",
		Content: sb.String(),
	}, nil
}

// ClearPart clears the stored todos
func (p *TodoContextPartProvider) ClearPart() error {
	p.mu.Lock()
	p.todosJSON = ""
	p.mu.Unlock()
	return nil
}

// GetTodosJSON returns the current todos JSON (for testing)
func (p *TodoContextPartProvider) GetTodosJSON() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.todosJSON
}
