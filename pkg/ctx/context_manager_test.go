package ctx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/toolctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chatText renders the manager's chat history the way the recorder and
// the context viewer see it.
func chatText(t *testing.T, manager ContextManager) string {
	t.Helper()
	history, err := manager.ChatHistory(context.Background())
	require.NoError(t, err)
	return FormatHistory(history)
}

func TestContextManager_CanBeCreated(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chat := NewChatHistory(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager, 0)
	registry.RegisterChat(chat, 0)

	// This should compile and create a context manager
	manager := NewContextManager(registry)

	assert.NotNil(t, manager)
}

func TestContextManager_ClearContextClearsTheChatHistory(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chat := NewChatHistory(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager, 0)
	registry.RegisterChat(chat, 0)

	manager := NewContextManager(registry)

	// Record chat history synchronously
	manager.RecordChatTurn("Hello", "Hi there!")

	assert.Contains(t, chatText(t, manager), "User: Hello")

	require.NoError(t, manager.ClearContext())

	assert.Empty(t, chatText(t, manager))
}

func TestContextManager_GetContextParts_EmptyContext(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chat := NewChatHistory(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager, 0)
	registry.RegisterChat(chat, 0)

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
	chat := NewChatHistory(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager, 0)
	registry.RegisterChat(chat, 0)

	manager := NewContextManager(registry)

	// Create context with CWD pointing to temp directory
	ctx := toolctx.WithWorkingDir(context.Background(), tempDir)

	// Get context parts - should include only project context
	parts, err := manager.GetContextParts(ctx)
	require.NoError(t, err)
	assert.Len(t, parts, 1, "Should contain exactly one context part")
	assert.Contains(t, parts, "project", "Should contain project key")
	assert.Equal(t, projectContent, parts["project"], "Project content should match")
	assert.NotContains(t, parts, "chat", "Should not contain chat key when no chat context")
}

func TestContextManager_ChatHistoryIsNotAContextPart(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chat := NewChatHistory(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager, 0)
	registry.RegisterChat(chat, 0)

	manager := NewContextManager(registry)

	// Record chat history synchronously
	manager.RecordChatTurn("Hello", "Hi there!")

	// The history is not a context part: the parts stay empty and the
	// turn is reachable through ChatHistory.
	parts, err := manager.GetContextParts(context.Background())
	require.NoError(t, err)
	assert.Empty(t, parts, "chat history must never be a template part")
	assert.Contains(t, chatText(t, manager), "User: Hello")
	assert.Contains(t, chatText(t, manager), "Assistant: Hi there!")
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
	chat := NewChatHistory(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager, 0)
	registry.RegisterChat(chat, 0)

	manager := NewContextManager(registry)

	// Record chat history synchronously
	manager.RecordChatTurn("Hello", "Hi there!")

	// Create context with CWD pointing to temp directory
	ctx := toolctx.WithWorkingDir(context.Background(), tempDir)

	// Get context parts - the project part only; the chat is separate
	parts, err := manager.GetContextParts(ctx)
	require.NoError(t, err)
	assert.Len(t, parts, 1, "Should contain exactly one context part")
	assert.Equal(t, projectContent, parts["project"], "Project content should match")

	// Verify chat history
	assert.Contains(t, chatText(t, manager), "User: Hello", "Chat content should contain user message")
	assert.Contains(t, chatText(t, manager), "Assistant: Hi there!", "Chat content should contain assistant response")
}

func TestContextManager_SeedChatHistory(t *testing.T) {
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chat := NewChatHistory(eventBus)

	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager, 0)
	registry.RegisterChat(chat, 0)

	manager := NewContextManager(registry)

	manager.SeedChatHistory([]Message{
		{User: "Hi Genie", Assistant: "Hello there!"},
	})

	assert.Contains(t, chatText(t, manager), "User: Hi Genie")
	assert.Contains(t, chatText(t, manager), "Assistant: Hello there!")
}

func TestContextManager_MultipleChatMessages(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chat := NewChatHistory(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager, 0)
	registry.RegisterChat(chat, 0)

	manager := NewContextManager(registry)

	// Record multiple chat exchanges synchronously
	manager.RecordChatTurn("First question", "First answer")
	manager.RecordChatTurn("Second question", "Second answer")

	// Verify chat history contains both messages in recorded order
	chatContent := chatText(t, manager)
	assert.Contains(t, chatContent, "User: First question", "Should contain first user message")
	assert.Contains(t, chatContent, "Assistant: First answer", "Should contain first assistant response")
	assert.Contains(t, chatContent, "User: Second question", "Should contain second user message")
	assert.Contains(t, chatContent, "Assistant: Second answer", "Should contain second assistant response")
}

func TestContextManager_ChatHistoryAfterClearContext(t *testing.T) {
	// Create event bus and managers
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chat := NewChatHistory(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager, 0)
	registry.RegisterChat(chat, 0)

	manager := NewContextManager(registry)

	// Record chat history synchronously
	manager.RecordChatTurn("Hello", "Hi there!")

	history, err := manager.ChatHistory(context.Background())
	require.NoError(t, err)
	assert.Len(t, history, 1, "Should hold the turn before clearing")

	require.NoError(t, manager.ClearContext())

	history, err = manager.ChatHistory(context.Background())
	require.NoError(t, err)
	assert.Empty(t, history, "Should hold nothing after clearing")
}

func TestContextManager_GetContextParts_EmptyContentFiltered(t *testing.T) {
	// Create event bus and managers with no actual content
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	chat := NewChatHistory(eventBus)

	// Create registry and register providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager, 0)
	registry.RegisterChat(chat, 0)

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
	chat := NewChatHistory(eventBus)
	fileProvider := NewFileContextPartsProvider(eventBus)

	// Create registry and register all providers
	registry := NewContextPartProviderRegistry()
	registry.Register(projectCtxManager, 0)
	registry.RegisterChat(chat, 0)
	registry.Register(fileProvider, 0)

	manager := NewContextManager(registry)

	// Record chat history synchronously
	manager.RecordChatTurn("Read a file", "I'll read that file for you")

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

	// Should have the file part only; the chat history is separate
	assert.Equal(t, 1, len(parts))
	assert.Contains(t, parts, "files")

	// Verify chat history
	assert.Contains(t, chatText(t, manager), "Read a file")
	assert.Contains(t, chatText(t, manager), "I'll read that file for you")

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

	// Should still have the file part only
	assert.Equal(t, 1, len(parts2))

	// File part should now contain both files (most recent first)
	assert.Contains(t, parts2["files"], "File: test2.go")
	assert.Contains(t, parts2["files"], "File: test.go")

	// Verify order - test2.go should come before test.go
	fileContent := parts2["files"]
	test2Index := strings.Index(fileContent, "test2.go")
	testIndex := strings.Index(fileContent, "test.go")
	assert.True(t, test2Index < testIndex, "test2.go should appear before test.go")
}

// The chat history takes its share of the budget beside the text providers.
func TestContextManager_SetContextBudgetIncludesTheChatShare(t *testing.T) {
	eventBus := events.NewEventBus()
	chat := NewChatHistory(eventBus).(*InMemoryChatHistory)
	files := NewFileContextPartsProvider(eventBus)

	registry := NewContextPartProviderRegistry()
	registry.RegisterChat(chat, 0.7)
	registry.Register(files, 0.3)
	manager := NewContextManager(registry)

	manager.SetContextBudget(1000)

	assert.Equal(t, 700, chat.tokenBudget)
	assert.Equal(t, 300, files.tokenBudget)
}

// A manager without a chat history answers every chat call harmlessly.
func TestContextManager_WithoutChatHistory(t *testing.T) {
	manager := NewContextManager(NewContextPartProviderRegistry())

	manager.RecordChatTurn("hello", "hi")
	manager.SeedChatHistory([]Message{{User: "a", Assistant: "b"}})
	require.NoError(t, manager.ClearContext())

	history, err := manager.ChatHistory(context.Background())
	require.NoError(t, err)
	assert.Nil(t, history)
}
