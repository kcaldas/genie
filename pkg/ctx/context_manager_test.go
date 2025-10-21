package ctx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextManager_CanBeCreated(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)

	// This should compile and create a context manager
	manager := NewContextManager(registry)

	assert.NotNil(t, manager)
}

func TestContextManager_ClearContextDelegatesToChatCtxManager(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)

	manager := NewContextManager(registry)

	// Add chat context via event
	chatEvent := events.ChatResponseEvent{
		Message:  "Hello",
		Response: "Hi there!",
		Error:    nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)

	// Verify context exists
	ctx := context.Background()
	parts, err := manager.GetContextParts(ctx)
	require.NoError(t, err)
	assert.Contains(t, parts["chat"], "User: Hello")

	// Clear context should delegate to ChatCtxManager
	err = manager.ClearContext()
	require.NoError(t, err)

	// Verify chat context is cleared
	parts, err = manager.GetContextParts(ctx)
	require.NoError(t, err)
	assert.NotContains(t, parts["chat"], "User: Hello")
}

func TestContextManager_GetContextParts_EmptyContext(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)

	manager := NewContextManager(registry)

	// Get context parts without any data
	ctx := context.Background()
	parts, err := manager.GetContextParts(ctx)
	require.NoError(t, err)
	assert.Empty(t, parts, "Should return empty map when no context exists")
}

func TestContextManager_GetContextParts_ProjectContextOnly(t *testing.T) {
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
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)

	manager := NewContextManager(registry)

	// Create context with CWD pointing to temp directory
	ctx := context.WithValue(context.Background(), "cwd", tempDir)

	// Get context parts - should include only project context
	parts, err := manager.GetContextParts(ctx)
	require.NoError(t, err)
	assert.Len(t, parts, 1, "Should contain exactly one context part")
	assert.Contains(t, parts, "project", "Should contain project key")
	assert.Equal(t, projectContent, parts["project"], "Project content should match")
	assert.NotContains(t, parts, "chat", "Should not contain chat key when no chat context")
}

func TestContextManager_GetContextParts_ChatContextOnly(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)

	manager := NewContextManager(registry)

	// Add chat context via event
	chatEvent := events.ChatResponseEvent{
		Message:  "Hello",
		Response: "Hi there!",
		Error:    nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)

	// Get context parts - should include only chat context
	ctx := context.Background()
	parts, err := manager.GetContextParts(ctx)
	require.NoError(t, err)
	assert.Len(t, parts, 1, "Should contain exactly one context part")
	assert.Contains(t, parts, "chat", "Should contain chat key")
	assert.Contains(t, parts["chat"], "User: Hello", "Chat content should contain user message")
	assert.Contains(t, parts["chat"], "Assistant: Hi there!", "Chat content should contain assistant response")
	assert.NotContains(t, parts, "project", "Should not contain project key when no chat context")
}

func TestContextManager_GetContextParts_BothProjectAndChatContext(t *testing.T) {
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
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)

	manager := NewContextManager(registry)

	// Add chat context via event
	chatEvent := events.ChatResponseEvent{
		Message:  "Hello",
		Response: "Hi there!",
		Error:    nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)

	// Create context with CWD pointing to temp directory
	ctx := context.WithValue(context.Background(), "cwd", tempDir)

	// Get context parts - should include both project and chat context
	parts, err := manager.GetContextParts(ctx)
	require.NoError(t, err)
	assert.Len(t, parts, 2, "Should contain exactly two context parts")

	// Verify project context
	assert.Contains(t, parts, "project", "Should contain project key")
	assert.Equal(t, projectContent, parts["project"], "Project content should match")

	// Verify chat context
	assert.Contains(t, parts, "chat", "Should contain chat key")
	assert.Contains(t, parts["chat"], "User: Hello", "Chat content should contain user message")
	assert.Contains(t, parts["chat"], "Assistant: Hi there!", "Chat content should contain assistant response")
}

func TestContextManager_SeedChatHistory(t *testing.T) {
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)

	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)

	manager := NewContextManager(registry)

	manager.SeedChatHistory([]Message{
		{User: "Hi Genie", Assistant: "Hello there!"},
	})

	parts, err := manager.GetContextParts(context.Background())
	require.NoError(t, err)
	assert.Contains(t, parts, "chat")
	assert.Contains(t, parts["chat"], "User: Hi Genie")
	assert.Contains(t, parts["chat"], "Assistant: Hello there!")
}

func TestContextManager_GetContextParts_MultipleChatMessages(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)

	manager := NewContextManager(registry)

	// Add multiple chat messages via events
	chatEvent1 := events.ChatResponseEvent{
		Message:  "First question",
		Response: "First answer",
		Error:    nil,
	}
	chatEvent2 := events.ChatResponseEvent{
		Message:  "Second question",
		Response: "Second answer",
		Error:    nil,
	}
	eventBus.Publish("chat.response", chatEvent1)
	eventBus.Publish("chat.response", chatEvent2)

	// Small delay for test reliability (not needed in production due to natural user interaction delays)
	time.Sleep(1 * time.Millisecond)

	// Get context parts
	ctx := context.Background()
	parts, err := manager.GetContextParts(ctx)
	require.NoError(t, err)
	assert.Len(t, parts, 1, "Should contain exactly one context part (chat)")

	// Verify chat context contains both messages (order may vary due to async processing, which is acceptable)
	chatContent := parts["chat"]
	assert.Contains(t, chatContent, "User: First question", "Should contain first user message")
	assert.Contains(t, chatContent, "Assistant: First answer", "Should contain first assistant response")
	assert.Contains(t, chatContent, "User: Second question", "Should contain second user message")
	assert.Contains(t, chatContent, "Assistant: Second answer", "Should contain second assistant response")
}

func TestContextManager_GetContextParts_AfterClearContext(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)

	manager := NewContextManager(registry)

	// Add chat context via event
	chatEvent := events.ChatResponseEvent{
		Message:  "Hello",
		Response: "Hi there!",
		Error:    nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)

	// Verify context exists
	ctx := context.Background()
	parts, err := manager.GetContextParts(ctx)
	require.NoError(t, err)
	assert.Len(t, parts, 1, "Should contain chat context before clearing")
	assert.Contains(t, parts, "chat", "Should contain chat key before clearing")

	// Clear context
	err = manager.ClearContext()
	require.NoError(t, err)

	// Verify context is cleared
	parts, err = manager.GetContextParts(ctx)
	require.NoError(t, err)
	assert.Empty(t, parts, "Should return empty map after clearing context")
}

func TestContextManager_GetContextParts_EmptyContentFiltered(t *testing.T) {
	// Create event bus and managers with no actual content
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)

	manager := NewContextManager(registry)

	// Use context with no project files and no chat messages
	ctx := context.Background()

	// Get context parts - should filter out empty content
	parts, err := manager.GetContextParts(ctx)
	require.NoError(t, err)
	assert.Empty(t, parts, "Should filter out context parts with empty content")
}

func TestContextManager_GetContextParts_WithFileProvider(t *testing.T) {
	// Create event bus
	eventBus := events.NewEventBus()

	// Create managers
	projectCtxManager := NewProjectCtxManager(eventBus)
	chatCtxManager := NewChatCtxManager(eventBus)
	fileProvider := NewFileContextPartsProvider(eventBus)

	// Create registry and register all providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager)
	registry.Register(chatCtxManager)
	registry.Register(fileProvider)

	manager := NewContextManager(registry)

	// Add chat context
	chatEvent := events.ChatResponseEvent{
		Message:  "Read a file",
		Response: "I'll read that file for you",
		Error:    nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)

	// Simulate file read tool execution
	fileEvent := events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": "test.go"},
		Result:     map[string]any{"results": "package main\n\nfunc main() {\n\tprintln(\"Hello\")\n}"},
	}
	eventBus.Publish("tool.executed", fileEvent)
	time.Sleep(10 * time.Millisecond)

	// Get context parts
	ctx := context.Background()
	parts, err := manager.GetContextParts(ctx)
	assert.NoError(t, err)

	// Should have chat and file parts
	assert.Equal(t, 2, len(parts))
	assert.Contains(t, parts, "chat")
	assert.Contains(t, parts, "files")

	// Verify chat content
	assert.Contains(t, parts["chat"], "Read a file")
	assert.Contains(t, parts["chat"], "I'll read that file for you")

	// Verify file content
	assert.Contains(t, parts["files"], "File: test.go")
	assert.Contains(t, parts["files"], "package main")
	assert.Contains(t, parts["files"], "Hello")

	// Read another file
	fileEvent2 := events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": "test2.go"},
		Result:     map[string]any{"results": "package test2\n\nfunc Test() {}"},
	}
	eventBus.Publish("tool.executed", fileEvent2)
	time.Sleep(10 * time.Millisecond)

	// Get updated context
	parts2, err := manager.GetContextParts(ctx)
	assert.NoError(t, err)

	// Should still have 2 parts (chat and file)
	assert.Equal(t, 2, len(parts2))

	// File part should now contain both files (most recent first)
	assert.Contains(t, parts2["files"], "File: test2.go")
	assert.Contains(t, parts2["files"], "File: test.go")

	// Verify order - test2.go should come before test.go
	fileContent := parts2["files"]
	test2Index := strings.Index(fileContent, "test2.go")
	testIndex := strings.Index(fileContent, "test.go")
	assert.True(t, test2Index < testIndex, "test2.go should appear before test.go")
}
