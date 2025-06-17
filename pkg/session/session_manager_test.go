package session

import (
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionManager_CreateSession(t *testing.T) {
	historyCh := make(chan events.SessionInteractionEvent, 10)
	contextCh := make(chan events.SessionInteractionEvent, 10)
	manager := NewSessionManager(historyCh, contextCh)

	session, err := manager.CreateSession("test-session")
	require.NoError(t, err)
	assert.Equal(t, "test-session", session.GetID())
}

func TestSessionManager_GetSession(t *testing.T) {
	historyCh := make(chan events.SessionInteractionEvent, 10)
	contextCh := make(chan events.SessionInteractionEvent, 10)
	manager := NewSessionManager(historyCh, contextCh)

	// Create a session
	created, err := manager.CreateSession("my-session")
	require.NoError(t, err)

	// Get the same session
	retrieved, err := manager.GetSession("my-session")
	require.NoError(t, err)
	assert.Equal(t, created.GetID(), retrieved.GetID())
}

func TestSessionManager_GetNonExistentSession(t *testing.T) {
	historyCh := make(chan events.SessionInteractionEvent, 10)
	contextCh := make(chan events.SessionInteractionEvent, 10)
	manager := NewSessionManager(historyCh, contextCh)

	_, err := manager.GetSession("does-not-exist")
	assert.Error(t, err)
}

func TestSessionManager_SessionPersistence(t *testing.T) {
	historyCh := make(chan events.SessionInteractionEvent, 10)
	contextCh := make(chan events.SessionInteractionEvent, 10)
	manager := NewSessionManager(historyCh, contextCh)

	// Create session and add interaction
	session, err := manager.CreateSession("persistent-session")
	require.NoError(t, err)

	err = session.AddInteraction("Hello", "Hi there!")
	require.NoError(t, err)

	// Give time for async event processing
	time.Sleep(50 * time.Millisecond)

	// Get the session again and verify it's the same session
	retrieved, err := manager.GetSession("persistent-session")
	require.NoError(t, err)

	// Verify it's the same session ID
	assert.Equal(t, session.GetID(), retrieved.GetID())
}
