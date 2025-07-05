package ctx

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/tools"
	"github.com/stretchr/testify/assert"
)

// waitForEventProcessing waits for asynchronous event processing to complete
func waitForEventProcessing() {
	time.Sleep(10 * time.Millisecond)
}

func TestTodoContextPartProvider_GetPart_EmptyList(t *testing.T) {
	eventBus := events.NewEventBus()
	provider := NewTodoContextPartProvider(eventBus)

	part, err := provider.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "todo", part.Key)
	assert.Empty(t, part.Content)
}

func TestTodoContextPartProvider_HandleToolExecutedEvent(t *testing.T) {
	eventBus := events.NewEventBus()
	provider := NewTodoContextPartProvider(eventBus)

	// Create a TodoWrite tool executed event
	todosData := []interface{}{
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
		map[string]interface{}{
			"id":       "3",
			"content":  "Test task 3",
			"status":   "completed",
			"priority": "low",
		},
	}

	toolEvent := events.ToolExecutedEvent{
		ExecutionID: "test-123",
		ToolName:    "TodoWrite",
		Parameters: map[string]any{
			"todos": todosData,
		},
		Result: map[string]any{
			"success": true,
			"message": "Successfully updated 3 todo(s)",
		},
	}

	// Publish the event
	eventBus.Publish("tool.executed", toolEvent)

	// Wait for asynchronous event processing
	waitForEventProcessing()

	// Verify todos were stored
	todos := provider.GetTodos()
	assert.Len(t, todos, 3)
	
	// Check first todo
	assert.Equal(t, "1", todos[0].ID)
	assert.Equal(t, "Test task 1", todos[0].Content)
	assert.Equal(t, tools.StatusPending, todos[0].Status)
	assert.Equal(t, tools.PriorityHigh, todos[0].Priority)

	// Check second todo
	assert.Equal(t, "2", todos[1].ID)
	assert.Equal(t, "Test task 2", todos[1].Content)
	assert.Equal(t, tools.StatusInProgress, todos[1].Status)
	assert.Equal(t, tools.PriorityMedium, todos[1].Priority)

	// Check third todo
	assert.Equal(t, "3", todos[2].ID)
	assert.Equal(t, "Test task 3", todos[2].Content)
	assert.Equal(t, tools.StatusCompleted, todos[2].Status)
	assert.Equal(t, tools.PriorityLow, todos[2].Priority)
}

func TestTodoContextPartProvider_GetPart_WithTodos(t *testing.T) {
	eventBus := events.NewEventBus()
	provider := NewTodoContextPartProvider(eventBus)

	// Add some todos via tool execution
	todosData := []interface{}{
		map[string]interface{}{
			"id":       "task1",
			"content":  "High priority pending task",
			"status":   "pending",
			"priority": "high",
		},
		map[string]interface{}{
			"id":       "task2",
			"content":  "Currently working on this",
			"status":   "in_progress",
			"priority": "medium",
		},
		map[string]interface{}{
			"id":       "task3",
			"content":  "This one is done",
			"status":   "completed",
			"priority": "low",
		},
	}

	toolEvent := events.ToolExecutedEvent{
		ToolName: "TodoWrite",
		Parameters: map[string]any{
			"todos": todosData,
		},
	}

	eventBus.Publish("tool.executed", toolEvent)

	// Wait for asynchronous event processing
	waitForEventProcessing()

	// Get the context part
	part, err := provider.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "todo", part.Key)
	assert.NotEmpty(t, part.Content)

	// Verify the content structure
	content := part.Content
	assert.Contains(t, content, "# Current Todo List")
	assert.Contains(t, content, "## In Progress")
	assert.Contains(t, content, "## Pending")
	assert.Contains(t, content, "## Completed")

	// Verify specific todos appear in content
	assert.Contains(t, content, "Currently working on this")
	assert.Contains(t, content, "High priority pending task")
	assert.Contains(t, content, "This one is done")

	// Verify task IDs appear
	assert.Contains(t, content, "[task1]")
	assert.Contains(t, content, "[task2]")
	assert.Contains(t, content, "[task3]")

	// Verify priorities appear
	assert.Contains(t, content, "high priority")
	assert.Contains(t, content, "medium priority")
	assert.Contains(t, content, "low priority")
}

func TestTodoContextPartProvider_IgnoresOtherTools(t *testing.T) {
	eventBus := events.NewEventBus()
	provider := NewTodoContextPartProvider(eventBus)

	// Publish events from other tools
	readFileEvent := events.ToolExecutedEvent{
		ToolName: "readFile",
		Parameters: map[string]any{
			"file_path": "test.txt",
		},
	}

	bashEvent := events.ToolExecutedEvent{
		ToolName: "runBashCommand",
		Parameters: map[string]any{
			"command": "ls -la",
		},
	}

	eventBus.Publish("tool.executed", readFileEvent)
	eventBus.Publish("tool.executed", bashEvent)

	// Wait for asynchronous event processing
	waitForEventProcessing()

	// Verify no todos were stored
	todos := provider.GetTodos()
	assert.Len(t, todos, 0)

	// Verify empty context
	part, err := provider.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, part.Content)
}

func TestTodoContextPartProvider_ClearPart(t *testing.T) {
	eventBus := events.NewEventBus()
	provider := NewTodoContextPartProvider(eventBus)

	// Add some todos
	todosData := []interface{}{
		map[string]interface{}{
			"id":       "1",
			"content":  "Test task",
			"status":   "pending",
			"priority": "high",
		},
	}

	toolEvent := events.ToolExecutedEvent{
		ToolName: "TodoWrite",
		Parameters: map[string]any{
			"todos": todosData,
		},
	}

	eventBus.Publish("tool.executed", toolEvent)

	// Wait for asynchronous event processing
	waitForEventProcessing()

	// Verify todos exist
	todos := provider.GetTodos()
	assert.Len(t, todos, 1)

	// Clear the todos
	err := provider.ClearPart()
	assert.NoError(t, err)

	// Verify todos are cleared
	todos = provider.GetTodos()
	assert.Len(t, todos, 0)

	// Verify empty context
	part, err := provider.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, part.Content)
}

func TestTodoContextPartProvider_UpdatesOnMultipleWrites(t *testing.T) {
	eventBus := events.NewEventBus()
	provider := NewTodoContextPartProvider(eventBus)

	// First write
	firstTodos := []interface{}{
		map[string]interface{}{
			"id":       "1",
			"content":  "First task",
			"status":   "pending",
			"priority": "high",
		},
	}

	eventBus.Publish("tool.executed", events.ToolExecutedEvent{
		ToolName:   "TodoWrite",
		Parameters: map[string]any{"todos": firstTodos},
	})

	// Wait for asynchronous event processing
	waitForEventProcessing()

	// Verify first write
	todos := provider.GetTodos()
	assert.Len(t, todos, 1)
	assert.Equal(t, "First task", todos[0].Content)

	// Second write (replaces first)
	secondTodos := []interface{}{
		map[string]interface{}{
			"id":       "2",
			"content":  "Second task",
			"status":   "in_progress",
			"priority": "medium",
		},
		map[string]interface{}{
			"id":       "3",
			"content":  "Third task",
			"status":   "completed",
			"priority": "low",
		},
	}

	eventBus.Publish("tool.executed", events.ToolExecutedEvent{
		ToolName:   "TodoWrite",
		Parameters: map[string]any{"todos": secondTodos},
	})

	// Wait for asynchronous event processing
	waitForEventProcessing()

	// Verify second write replaced first
	todos = provider.GetTodos()
	assert.Len(t, todos, 2)
	assert.Equal(t, "Second task", todos[0].Content)
	assert.Equal(t, "Third task", todos[1].Content)

	// Verify first task no longer exists
	for _, todo := range todos {
		assert.NotEqual(t, "First task", todo.Content)
	}
}

func TestTodoContextPartProvider_HandlesInvalidData(t *testing.T) {
	eventBus := events.NewEventBus()
	provider := NewTodoContextPartProvider(eventBus)

	// Try with invalid data that should be ignored
	invalidEvents := []events.ToolExecutedEvent{
		{
			ToolName:   "TodoWrite",
			Parameters: map[string]any{}, // Missing todos
		},
		{
			ToolName: "TodoWrite",
			Parameters: map[string]any{
				"todos": "not an array", // Wrong type
			},
		},
		{
			ToolName: "TodoWrite",
			Parameters: map[string]any{
				"todos": []interface{}{
					"not an object", // Wrong element type
				},
			},
		},
		{
			ToolName: "TodoWrite",
			Parameters: map[string]any{
				"todos": []interface{}{
					map[string]interface{}{
						"id": "1",
						// Missing required fields
					},
				},
			},
		},
	}

	for _, event := range invalidEvents {
		eventBus.Publish("tool.executed", event)
	}

	// Wait for asynchronous event processing
	waitForEventProcessing()

	// Verify no todos were stored
	todos := provider.GetTodos()
	assert.Len(t, todos, 0)
}