package session

import (
	"testing"

	"github.com/kcaldas/genie/pkg/context"
	"github.com/kcaldas/genie/pkg/history"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession_AddInteraction(t *testing.T) {
	contextManager := context.NewContextManager()
	historyManager := history.NewHistoryManager()
	session := NewSessionWithManagers("test-session", contextManager, historyManager)
	
	err := session.AddInteraction("Hello", "Hi there!")
	require.NoError(t, err)
	
	// Check both context and history were updated
	contextData := session.GetContext()
	assert.Len(t, contextData, 2)
	assert.Equal(t, "Hello", contextData[0])
	assert.Equal(t, "Hi there!", contextData[1])
	
	historyData := session.GetHistory()
	assert.Len(t, historyData, 2)
	assert.Equal(t, "Hello", historyData[0])
	assert.Equal(t, "Hi there!", historyData[1])
}

func TestSession_MultipleInteractions(t *testing.T) {
	contextManager := context.NewContextManager()
	historyManager := history.NewHistoryManager()
	session := NewSessionWithManagers("test-session", contextManager, historyManager)
	
	session.AddInteraction("First question", "First answer")
	session.AddInteraction("Second question", "Second answer")
	
	context := session.GetContext()
	assert.Len(t, context, 4)
	assert.Equal(t, "First question", context[0])
	assert.Equal(t, "First answer", context[1])
	assert.Equal(t, "Second question", context[2])
	assert.Equal(t, "Second answer", context[3])
}

func TestSession_GetID(t *testing.T) {
	contextManager := context.NewContextManager()
	historyManager := history.NewHistoryManager()
	session := NewSessionWithManagers("my-session-id", contextManager, historyManager)
	assert.Equal(t, "my-session-id", session.GetID())
}