package ctx

import (
	"context"
	"encoding/json"
	"fmt"
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

	return ContextPart{
		Key:     "todo",
		Content: fmt.Sprintf("```json\n%s\n```", todosJSON),
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
