package history

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryManager_AddInteraction(t *testing.T) {
	var manager HistoryManager = NewHistoryManager()
	
	err := manager.AddInteraction("session-1", "Hello", "Hi there!")
	require.NoError(t, err)
	
	history, err := manager.GetHistory("session-1")
	require.NoError(t, err)
	assert.Len(t, history, 2) // user message + assistant response
	assert.Equal(t, "Hello", history[0])
	assert.Equal(t, "Hi there!", history[1])
}

func TestHistoryManager_MultipleInteractions(t *testing.T) {
	var manager HistoryManager = NewHistoryManager()
	
	manager.AddInteraction("session-1", "First question", "First answer")
	manager.AddInteraction("session-1", "Second question", "Second answer")
	
	history, err := manager.GetHistory("session-1")
	require.NoError(t, err)
	assert.Len(t, history, 4)
	assert.Equal(t, "First question", history[0])
	assert.Equal(t, "First answer", history[1])
	assert.Equal(t, "Second question", history[2])
	assert.Equal(t, "Second answer", history[3])
}

func TestHistoryManager_GetNonExistentHistory(t *testing.T) {
	var manager HistoryManager = NewHistoryManager()
	
	_, err := manager.GetHistory("non-existent")
	assert.Error(t, err)
}

func TestHistoryManager_ClearHistory(t *testing.T) {
	var manager HistoryManager = NewHistoryManager()
	
	// Add some history
	manager.AddInteraction("session-1", "Hello", "Hi")
	
	// Clear history
	err := manager.ClearHistory("session-1")
	require.NoError(t, err)
	
	// Verify history is cleared
	_, err = manager.GetHistory("session-1")
	assert.Error(t, err)
}