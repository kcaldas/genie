package session

import (
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionManager_CreateSession(t *testing.T) {
	publisher := events.NewEventBus()
	manager := NewSessionManager(publisher)

	session, err := manager.CreateSession("test-session", "/test/workdir")
	require.NoError(t, err)
	assert.Equal(t, "test-session", session.GetID())
	assert.Equal(t, "/test/workdir", session.GetWorkingDirectory())
}

func TestSessionManager_GetSession(t *testing.T) {
	publisher := events.NewEventBus()
	manager := NewSessionManager(publisher)

	// Create a session
	created, err := manager.CreateSession("my-session", "/my/workdir")
	require.NoError(t, err)

	// Get the same session
	retrieved, err := manager.GetSession("my-session")
	require.NoError(t, err)
	assert.Equal(t, created.GetID(), retrieved.GetID())
	assert.Equal(t, created.GetWorkingDirectory(), retrieved.GetWorkingDirectory())
}

func TestSessionManager_GetNonExistentSession(t *testing.T) {
	publisher := events.NewEventBus()
	manager := NewSessionManager(publisher)

	_, err := manager.GetSession("does-not-exist")
	assert.Error(t, err)
}
