package ctx

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
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

	// Create a TodoWrite tool executed event with simple maps
	todosData := []map[string]interface{}{
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
		{
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
			"todos":   todosData,
		},
	}

	// Publish the event
	eventBus.Publish("tool.executed", toolEvent)

	// Wait for asynchronous event processing
	waitForEventProcessing()

	// Verify JSON was stored
	todosJSON := provider.GetTodosJSON()
	assert.NotEmpty(t, todosJSON)
	
	// Verify the JSON contains the expected data
	assert.Contains(t, todosJSON, `"id": "1"`)
	assert.Contains(t, todosJSON, `"content": "Test task 1"`)
	assert.Contains(t, todosJSON, `"status": "pending"`)
	assert.Contains(t, todosJSON, `"priority": "high"`)
	
	assert.Contains(t, todosJSON, `"id": "2"`)
	assert.Contains(t, todosJSON, `"content": "Test task 2"`)
	assert.Contains(t, todosJSON, `"status": "in_progress"`)
	
	assert.Contains(t, todosJSON, `"id": "3"`)
	assert.Contains(t, todosJSON, `"content": "Test task 3"`)
	assert.Contains(t, todosJSON, `"status": "completed"`)
}

func TestTodoContextPartProvider_GetPart_WithTodos(t *testing.T) {
	eventBus := events.NewEventBus()
	provider := NewTodoContextPartProvider(eventBus)

	// Add some todos via tool execution
	todosData := []map[string]interface{}{
		{
			"id":       "task1",
			"content":  "High priority pending task",
			"status":   "pending",
			"priority": "high",
		},
		{
			"id":       "task2",
			"content":  "Currently working on this",
			"status":   "in_progress",
			"priority": "medium",
		},
		{
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
		Result: map[string]any{
			"success": true,
			"message": "Successfully updated 3 todo(s)",
			"todos":   todosData,
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

	// Verify the content is JSON wrapped in markdown
	content := part.Content
	assert.Contains(t, content, "```json")
	assert.Contains(t, content, "```")

	// Verify specific todos appear in JSON
	assert.Contains(t, content, "task1")
	assert.Contains(t, content, "High priority pending task")
	assert.Contains(t, content, "pending")
	assert.Contains(t, content, "high")
	
	assert.Contains(t, content, "task2")
	assert.Contains(t, content, "Currently working on this")
	assert.Contains(t, content, "in_progress")
	
	assert.Contains(t, content, "task3")
	assert.Contains(t, content, "This one is done")
	assert.Contains(t, content, "completed")
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
		ToolName: "bash",
		Parameters: map[string]any{
			"command": "ls -la",
		},
	}

	eventBus.Publish("tool.executed", readFileEvent)
	eventBus.Publish("tool.executed", bashEvent)

	// Wait for asynchronous event processing
	waitForEventProcessing()

	// Verify no todos JSON was stored
	todosJSON := provider.GetTodosJSON()
	assert.Empty(t, todosJSON)

	// Verify empty context
	part, err := provider.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, part.Content)
}

func TestTodoContextPartProvider_ClearPart(t *testing.T) {
	eventBus := events.NewEventBus()
	provider := NewTodoContextPartProvider(eventBus)

	// Add some todos
	todosData := []map[string]interface{}{
		{
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
		Result: map[string]any{
			"success": true,
			"message": "Successfully updated 1 todo(s)",
			"todos":   todosData,
		},
	}

	eventBus.Publish("tool.executed", toolEvent)

	// Wait for asynchronous event processing
	waitForEventProcessing()

	// Verify todos JSON exists
	todosJSON := provider.GetTodosJSON()
	assert.NotEmpty(t, todosJSON)
	assert.Contains(t, todosJSON, "Test task")

	// Clear the todos
	err := provider.ClearPart()
	assert.NoError(t, err)

	// Verify todos JSON is cleared
	todosJSON = provider.GetTodosJSON()
	assert.Empty(t, todosJSON)

	// Verify empty context
	part, err := provider.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, part.Content)
}

func TestTodoContextPartProvider_UpdatesOnMultipleWrites(t *testing.T) {
	eventBus := events.NewEventBus()
	provider := NewTodoContextPartProvider(eventBus)

	// First write
	firstTodos := []map[string]interface{}{
		{
			"id":       "1",
			"content":  "First task",
			"status":   "pending",
			"priority": "high",
		},
	}

	eventBus.Publish("tool.executed", events.ToolExecutedEvent{
		ToolName:   "TodoWrite",
		Parameters: map[string]any{"todos": firstTodos},
		Result: map[string]any{
			"success": true,
			"message": "Successfully updated 1 todo(s)",
			"todos":   firstTodos,
		},
	})

	// Wait for asynchronous event processing
	waitForEventProcessing()

	// Verify first write
	todosJSON := provider.GetTodosJSON()
	assert.NotEmpty(t, todosJSON)
	assert.Contains(t, todosJSON, "First task")
	assert.Contains(t, todosJSON, "pending")

	// Second write (replaces first)
	secondTodos := []map[string]interface{}{
		{
			"id":       "2",
			"content":  "Second task",
			"status":   "in_progress",
			"priority": "medium",
		},
		{
			"id":       "3",
			"content":  "Third task",
			"status":   "completed",
			"priority": "low",
		},
	}

	eventBus.Publish("tool.executed", events.ToolExecutedEvent{
		ToolName:   "TodoWrite",
		Parameters: map[string]any{"todos": secondTodos},
		Result: map[string]any{
			"success": true,
			"message": "Successfully updated 2 todo(s)",
			"todos":   secondTodos,
		},
	})

	// Wait for asynchronous event processing
	waitForEventProcessing()

	// Verify second write replaced first
	todosJSON = provider.GetTodosJSON()
	assert.NotEmpty(t, todosJSON)
	assert.Contains(t, todosJSON, "Second task")
	assert.Contains(t, todosJSON, "Third task")
	assert.Contains(t, todosJSON, "in_progress")
	assert.Contains(t, todosJSON, "completed")

	// Verify first task no longer exists
	assert.NotContains(t, todosJSON, "First task")
}

func TestTodoContextPartProvider_HandlesInvalidData(t *testing.T) {
	eventBus := events.NewEventBus()
	provider := NewTodoContextPartProvider(eventBus)

	// Try with invalid data that should be ignored
	invalidEvents := []events.ToolExecutedEvent{
		{
			ToolName:   "TodoWrite",
			Parameters: map[string]any{}, // Missing todos
			Result: map[string]any{},
		},
		{
			ToolName: "TodoWrite",
			Parameters: map[string]any{
				"todos": "not an array", // Wrong type
			},
			Result: map[string]any{
				"todos": "not an array",
			},
		},
		{
			ToolName: "TodoWrite",
			Parameters: map[string]any{
				"todos": []interface{}{
					"not an object", // Wrong element type
				},
			},
			Result: map[string]any{
				"todos": []interface{}{
					"not an object",
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
			Result: map[string]any{
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

	// Verify that only valid JSON data gets stored
	// After the fix, all these invalid events should result in an empty todosJSON
	todosJSON := provider.GetTodosJSON()
	assert.Empty(t, todosJSON)
}