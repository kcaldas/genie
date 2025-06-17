package session

import (
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession_AddInteraction(t *testing.T) {
	publisher := events.NewEventBus()
	session := NewSession("test-session", publisher)

	err := session.AddInteraction("Hello", "Hi there!")
	require.NoError(t, err)

	// Session publishes events - we can only verify the interaction was accepted without error
	assert.Equal(t, "test-session", session.GetID())
}

func TestSession_MultipleInteractions(t *testing.T) {
	publisher := events.NewEventBus()
	session := NewSession("test-session", publisher)

	err1 := session.AddInteraction("First question", "First answer")
	err2 := session.AddInteraction("Second question", "Second answer")

	// Verify both interactions were accepted
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, "test-session", session.GetID())
}

func TestSession_GetID(t *testing.T) {
	publisher := events.NewEventBus()
	session := NewSession("my-session-id", publisher)
	assert.Equal(t, "my-session-id", session.GetID())
}
