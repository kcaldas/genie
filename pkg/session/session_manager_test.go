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

	session, err := manager.CreateSession("/test/workdir")
	require.NoError(t, err)
	assert.Equal(t, "/test/workdir", session.GetWorkingDirectory())
}

func TestSessionManager_GetSession(t *testing.T) {
	publisher := events.NewEventBus()
	manager := NewSessionManager(publisher)

	// Create a session
	created, err := manager.CreateSession("/my/workdir")
	require.NoError(t, err)

	// Get the same session
	retrieved, err := manager.GetSession()
	require.NoError(t, err)
	assert.Equal(t, created.GetWorkingDirectory(), retrieved.GetWorkingDirectory())
}

func TestSessionManager_GetNonExistentSession(t *testing.T) {
	publisher := events.NewEventBus()
	manager := NewSessionManager(publisher)

	_, err := manager.GetSession()
	assert.Error(t, err)
}
