package session

import (
	"testing"

	"github.com/kcaldas/genie/pkg/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionManager_CreateSession(t *testing.T) {
	contextManager := context.NewContextManager()
	var manager SessionManager = NewSessionManagerWithContext(contextManager)
	
	session, err := manager.CreateSession("test-session")
	require.NoError(t, err)
	assert.Equal(t, "test-session", session.GetID())
}

func TestSessionManager_GetSession(t *testing.T) {
	contextManager := context.NewContextManager()
	var manager SessionManager = NewSessionManagerWithContext(contextManager)
	
	// Create a session
	created, err := manager.CreateSession("my-session")
	require.NoError(t, err)
	
	// Get the same session
	retrieved, err := manager.GetSession("my-session")
	require.NoError(t, err)
	assert.Equal(t, created.GetID(), retrieved.GetID())
}

func TestSessionManager_GetNonExistentSession(t *testing.T) {
	contextManager := context.NewContextManager()
	var manager SessionManager = NewSessionManagerWithContext(contextManager)
	
	_, err := manager.GetSession("does-not-exist")
	assert.Error(t, err)
}

func TestSessionManager_SessionPersistence(t *testing.T) {
	contextManager := context.NewContextManager()
	var manager SessionManager = NewSessionManagerWithContext(contextManager)
	
	// Create session and add interaction
	session, err := manager.CreateSession("persistent-session")
	require.NoError(t, err)
	
	err = session.AddInteraction("Hello", "Hi there!")
	require.NoError(t, err)
	
	// Get the session again and verify interaction is still there
	retrieved, err := manager.GetSession("persistent-session")
	require.NoError(t, err)
	
	context := retrieved.GetContext()
	assert.Len(t, context, 2)
	assert.Equal(t, "Hello", context[0])
	assert.Equal(t, "Hi there!", context[1])
}