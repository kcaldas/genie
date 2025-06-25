package ctx

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/kcaldas/genie/pkg/events"
)

func TestContextManager_AddInteraction(t *testing.T) {
	var manager ContextManager = NewInMemoryContextManager()

	err := manager.AddInteraction("session-1", "Hello", "Hi there!")
	require.NoError(t, err)

	ctx := context.Background()
	result, err := manager.GetContext(ctx, "session-1")
	require.NoError(t, err)
	assert.Len(t, result, 2) // user message + assistant response
	assert.Equal(t, "Hello", result[0])
	assert.Equal(t, "Hi there!", result[1])
}

func TestContextManager_MultipleInteractions(t *testing.T) {
	var manager ContextManager = NewInMemoryContextManager()

	manager.AddInteraction("session-1", "First question", "First answer")
	manager.AddInteraction("session-1", "Second question", "Second answer")

	ctx := context.Background()
	result, err := manager.GetContext(ctx, "session-1")
	require.NoError(t, err)
	assert.Len(t, result, 4)
	assert.Equal(t, "First question", result[0])
	assert.Equal(t, "First answer", result[1])
	assert.Equal(t, "Second question", result[2])
	assert.Equal(t, "Second answer", result[3])
}

func TestContextManager_GetNonExistentContext(t *testing.T) {
	var manager ContextManager = NewInMemoryContextManager()

	ctx := context.Background()
	result, err := manager.GetContext(ctx, "non-existent")
	assert.NoError(t, err)
	assert.Empty(t, result) // Should return empty slice, not error
}

func TestContextManager_ClearContext(t *testing.T) {
	var manager ContextManager = NewInMemoryContextManager()

	// Add some context
	manager.AddInteraction("session-1", "Hello", "Hi")

	// Clear context
	err := manager.ClearContext("session-1")
	require.NoError(t, err)

	// Verify context is cleared
	ctx := context.Background()
	result, err := manager.GetContext(ctx, "session-1")
	assert.NoError(t, err)
	assert.Empty(t, result) // Should return empty slice after clearing
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

func TestContextManager_GetContextIncludesProjectContextAtTop(t *testing.T) {
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
	
	// Add some session interactions
	err = manager.AddInteraction("session-1", "Hello", "Hi there!")
	require.NoError(t, err)
	
	// Create context with CWD pointing to temp directory
	ctx := context.WithValue(context.Background(), "cwd", tempDir)
	
	// Get context - should include project context at the top
	result, err := manager.GetContext(ctx, "session-1")
	require.NoError(t, err)
	
	// Should start with project context when project context manager is present
	assert.Len(t, result, 3) // project context + user message + assistant message
	assert.Equal(t, projectContent, result[0])
	assert.Equal(t, "Hello", result[1])
	assert.Equal(t, "Hi there!", result[2])
}

func TestContextManager_GetContextWithEmptyProjectContext(t *testing.T) {
	// Create a temporary directory WITHOUT GENIE.md
	tempDir := t.TempDir()
	
	// Create event bus and project context manager
	eventBus := events.NewEventBus()
	projectCtxManager := NewProjectCtxManager(eventBus)
	
	// Create context manager with project context manager
	manager := NewContextManager(eventBus, projectCtxManager)
	
	// Add some session interactions
	err := manager.AddInteraction("session-1", "Hello", "Hi there!")
	require.NoError(t, err)
	
	// Create context with CWD pointing to temp directory (no GENIE.md)
	ctx := context.WithValue(context.Background(), "cwd", tempDir)
	
	// Get context - should only have session context since no project context
	result, err := manager.GetContext(ctx, "session-1")
	require.NoError(t, err)
	
	// Should only have session context, no project context
	assert.Len(t, result, 2) // user message + assistant message
	assert.Equal(t, "Hello", result[0])
	assert.Equal(t, "Hi there!", result[1])
}
