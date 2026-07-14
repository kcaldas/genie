package ctx

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Chat history is correctness state: the core records turns
// synchronously via AddTurn. The provider must not also build history
// from bus events, or turns would be double-recorded and history would
// depend on asynchronous delivery.
func TestChatProviderDoesNotRecordFromBusEvents(t *testing.T) {
	bus := events.NewEventBus()
	provider := NewChatCtxManager(bus)

	event := events.ChatResponseEvent{
		Message:  "a question",
		Response: "an answer",
	}
	bus.PublishSync(event.Topic(), event)

	part, err := provider.GetPart(context.Background())
	require.NoError(t, err)
	assert.Empty(t, part.Content, "history must come from AddTurn, not from bus events")
}

func TestChatProviderAddTurn(t *testing.T) {
	bus := events.NewEventBus()
	provider := NewChatCtxManager(bus)

	provider.AddTurn("first question", "complete answer")
	provider.AddTurn("", "assistant-only note") // ephemeral input
	provider.AddTurn("user-only note", "")      // ephemeral output

	part, err := provider.GetPart(context.Background())
	require.NoError(t, err)

	assert.Contains(t, part.Content, "first question")
	assert.Contains(t, part.Content, "complete answer")
	assert.Contains(t, part.Content, "assistant-only note")
	assert.Contains(t, part.Content, "user-only note")
}

func TestChatProviderAddTurnSkipsEmptyTurns(t *testing.T) {
	bus := events.NewEventBus()
	provider := NewChatCtxManager(bus)

	provider.AddTurn("", "")

	part, err := provider.GetPart(context.Background())
	require.NoError(t, err)
	assert.Empty(t, part.Content)
}
