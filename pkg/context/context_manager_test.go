package context

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextManager_AddInteraction(t *testing.T) {
	var manager ContextManager = NewContextManager()
	
	err := manager.AddInteraction("session-1", "Hello", "Hi there!")
	require.NoError(t, err)
	
	context, err := manager.GetContext("session-1")
	require.NoError(t, err)
	assert.Len(t, context, 2) // user message + assistant response
	assert.Equal(t, "Hello", context[0])
	assert.Equal(t, "Hi there!", context[1])
}

func TestContextManager_MultipleInteractions(t *testing.T) {
	var manager ContextManager = NewContextManager()
	
	manager.AddInteraction("session-1", "First question", "First answer")
	manager.AddInteraction("session-1", "Second question", "Second answer")
	
	context, err := manager.GetContext("session-1")
	require.NoError(t, err)
	assert.Len(t, context, 4)
	assert.Equal(t, "First question", context[0])
	assert.Equal(t, "First answer", context[1])
	assert.Equal(t, "Second question", context[2])
	assert.Equal(t, "Second answer", context[3])
}

func TestContextManager_GetNonExistentContext(t *testing.T) {
	var manager ContextManager = NewContextManager()
	
	_, err := manager.GetContext("non-existent")
	assert.Error(t, err)
}

func TestContextManager_ClearContext(t *testing.T) {
	var manager ContextManager = NewContextManager()
	
	// Add some context
	manager.AddInteraction("session-1", "Hello", "Hi")
	
	// Clear context
	err := manager.ClearContext("session-1")
	require.NoError(t, err)
	
	// Verify context is cleared
	_, err = manager.GetContext("session-1")
	assert.Error(t, err)
}