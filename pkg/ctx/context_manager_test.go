package ctx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/kcaldas/genie/pkg/events"
)

func TestContextManager_ReceivesChatResponseEvents(t *testing.T) {
	// Create event bus and context manager
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	manager := NewContextManager(eventBus, projectCtxManager)

	// Simulate a chat response event
	chatEvent := events.ChatResponseEvent{
		SessionID: "session-1",
		Message:   "Hello",
		Response:  "Hi there!",
		Error:     nil,
	}
	eventBus.Publish("chat.response", chatEvent)

	// Give time for event processing
	time.Sleep(10 * time.Millisecond)

	// Get LLM context
	ctx := context.Background()
	result, err := manager.GetLLMContext(ctx, "session-1")
	require.NoError(t, err)
	
	// Should contain the conversation
	assert.Contains(t, result, "User: Hello")
	assert.Contains(t, result, "Assistant: Hi there!")
}

func TestContextManager_MultipleInteractions(t *testing.T) {
	// Create event bus and context manager
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	manager := NewContextManager(eventBus, projectCtxManager)

	// Simulate multiple chat response events
	chatEvent1 := events.ChatResponseEvent{
		SessionID: "session-1",
		Message:   "First question",
		Response:  "First answer",
		Error:     nil,
	}
	chatEvent2 := events.ChatResponseEvent{
		SessionID: "session-1",
		Message:   "Second question",
		Response:  "Second answer",
		Error:     nil,
	}
	
	eventBus.Publish("chat.response", chatEvent1)
	eventBus.Publish("chat.response", chatEvent2)

	// Give time for event processing
	time.Sleep(10 * time.Millisecond)

	// Get LLM context
	ctx := context.Background()
	result, err := manager.GetLLMContext(ctx, "session-1")
	require.NoError(t, err)
	
	// Should contain both conversations
	assert.Contains(t, result, "User: First question")
	assert.Contains(t, result, "Assistant: First answer")
	assert.Contains(t, result, "User: Second question")
	assert.Contains(t, result, "Assistant: Second answer")
}

func TestContextManager_GetNonExistentContext(t *testing.T) {
	var manager ContextManager = NewInMemoryContextManager()

	ctx := context.Background()
	result, err := manager.GetLLMContext(ctx, "non-existent")
	assert.NoError(t, err)
	assert.Empty(t, result) // Should return empty string, not error
}

func TestContextManager_ClearContext(t *testing.T) {
	// Create event bus and context manager
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	manager := NewContextManager(eventBus, projectCtxManager)

	// Add some context via event
	chatEvent := events.ChatResponseEvent{
		SessionID: "session-1",
		Message:   "Hello",
		Response:  "Hi",
		Error:     nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)

	// Clear context
	err := manager.ClearContext("session-1")
	require.NoError(t, err)

	// Verify context is cleared
	ctx := context.Background()
	result, err := manager.GetLLMContext(ctx, "session-1")
	assert.NoError(t, err)
	assert.Empty(t, result) // Should return empty string after clearing
}

func TestContextManager_CanBeCreatedWithProjectCtxManager(t *testing.T) {
	// Create event bus
	eventBus := events.NewEventBus()
	
	// Create project context manager
	projectCtxManager := NewProjectCtxManager(eventBus)
	
	// This should compile and create a context manager with project context manager
	manager := NewContextManager(eventBus, projectCtxManager)
	
	assert.NotNil(t, manager)
}

func TestContextManager_GetLLMContextIncludesProjectContextAtTop(t *testing.T) {
	// Create a temporary directory with GENIE.md
	tempDir := t.TempDir()
	projectContent := "# Project Context\n\nThis is project-specific context."
	genieMdPath := filepath.Join(tempDir, "GENIE.md")
	err := os.WriteFile(genieMdPath, []byte(projectContent), 0644)
	require.NoError(t, err)
	
	// Create event bus and project context manager
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	
	// Create context manager with project context manager
	manager := NewContextManager(eventBus, projectCtxManager)
	
	// Add some session interactions via event
	chatEvent := events.ChatResponseEvent{
		SessionID: "session-1",
		Message:   "Hello",
		Response:  "Hi there!",
		Error:     nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)
	
	// Create context with CWD pointing to temp directory
	ctx := context.WithValue(context.Background(), "cwd", tempDir)
	
	// Get LLM context - should include project context at the top
	result, err := manager.GetLLMContext(ctx, "session-1")
	require.NoError(t, err)
	
	// Should start with project context when project context manager is present
	assert.Contains(t, result, projectContent)
	assert.Contains(t, result, "User: Hello")
	assert.Contains(t, result, "Assistant: Hi there!")
	// Project context should come before conversation
	projectIndex := strings.Index(result, projectContent)
	conversationIndex := strings.Index(result, "User: Hello")
	assert.True(t, projectIndex < conversationIndex, "Project context should come before conversation")
}

func TestContextManager_GetLLMContextWithEmptyProjectContext(t *testing.T) {
	// Create a temporary directory WITHOUT GENIE.md
	tempDir := t.TempDir()
	
	// Create event bus and project context manager
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	
	// Create context manager with project context manager
	manager := NewContextManager(eventBus, projectCtxManager)
	
	// Add some session interactions via event
	chatEvent := events.ChatResponseEvent{
		SessionID: "session-1",
		Message:   "Hello",
		Response:  "Hi there!",
		Error:     nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)
	
	// Create context with CWD pointing to temp directory (no GENIE.md)
	ctx := context.WithValue(context.Background(), "cwd", tempDir)
	
	// Get LLM context - should only have session context since no project context
	result, err := manager.GetLLMContext(ctx, "session-1")
	require.NoError(t, err)
	
	// Should only have session context, no project context
	assert.Contains(t, result, "User: Hello")
	assert.Contains(t, result, "Assistant: Hi there!")
	assert.NotContains(t, result, "#") // No project context (markdown headers)
}
