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

func TestContextManager_CanBeCreated(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)
	
	// Create registry and register providers
	registry := NewContextRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)
	
	// This should compile and create a context manager
	manager := NewContextManager(registry)
	
	assert.NotNil(t, manager)
}

func TestContextManager_GetLLMContextIncludesChatContext(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)
	
	// Create registry and register providers
	registry := NewContextRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)
	
	manager := NewContextManager(registry)

	// Add chat context via event
	chatEvent := events.ChatResponseEvent{
		SessionID: "session-1",
		Message:   "Hello",
		Response:  "Hi there!",
		Error:     nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)

	// Get LLM context should include chat context
	ctx := context.Background()
	result, err := manager.GetLLMContext(ctx)
	require.NoError(t, err)
	
	// Should contain chat context formatted by ChatCtxManager
	assert.Contains(t, result, "User: Hello")
	assert.Contains(t, result, "Genie: Hi there!")
}

func TestContextManager_GetLLMContextWithProjectAndChatContext(t *testing.T) {
	// Create a temporary directory with GENIE.md
	tempDir := t.TempDir()
	projectContent := "# Project Context\n\nThis is project-specific context."
	genieMdPath := filepath.Join(tempDir, "GENIE.md")
	err := os.WriteFile(genieMdPath, []byte(projectContent), 0644)
	require.NoError(t, err)
	
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)
	
	// Create registry and register providers
	registry := NewContextRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)
	
	manager := NewContextManager(registry)

	// Add chat context via event
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

	// Get LLM context should include both project and chat context
	result, err := manager.GetLLMContext(ctx)
	require.NoError(t, err)
	
	// Should contain project context
	assert.Contains(t, result, projectContent)
	// Should contain chat context
	assert.Contains(t, result, "User: Hello")
	assert.Contains(t, result, "Genie: Hi there!")
	// Project context should come before chat context
	projectIndex := strings.Index(result, projectContent)
	chatIndex := strings.Index(result, "User: Hello")
	assert.True(t, projectIndex < chatIndex, "Project context should come before chat context")
}

func TestContextManager_ClearContextDelegatesToChatCtxManager(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)
	
	// Create registry and register providers
	registry := NewContextRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)
	
	manager := NewContextManager(registry)

	// Add chat context via event
	chatEvent := events.ChatResponseEvent{
		SessionID: "session-1",
		Message:   "Hello",
		Response:  "Hi there!",
		Error:     nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)

	// Verify context exists
	ctx := context.Background()
	result, err := manager.GetLLMContext(ctx)
	require.NoError(t, err)
	assert.Contains(t, result, "User: Hello")

	// Clear context should delegate to ChatCtxManager
	err = manager.ClearContext()
	require.NoError(t, err)

	// Verify chat context is cleared
	result, err = manager.GetLLMContext(ctx)
	require.NoError(t, err)
	assert.NotContains(t, result, "User: Hello")
}

func TestContextManager_GetNonExistentContext(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)
	
	// Create registry and register providers
	registry := NewContextRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)
	
	var manager ContextManager = NewContextManager(registry)

	ctx := context.Background()
	result, err := manager.GetLLMContext(ctx)
	assert.NoError(t, err)
	assert.Empty(t, result) // Should return empty string, not error
}

func TestContextManager_EmptyContext(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)
	
	// Create registry and register providers
	registry := NewContextRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)
	
	manager := NewContextManager(registry)

	// Get context without any data
	ctx := context.Background()
	result, err := manager.GetLLMContext(ctx)
	require.NoError(t, err)
	assert.Empty(t, result)
}