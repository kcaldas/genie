package ctx

import (
	"context"
	"errors"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Failed or cancelled turns must not enter chat history: a cancelled
// stream's partial output would otherwise be replayed to the model as
// a completed exchange on every subsequent turn.
func TestChatContextSkipsErroredResponses(t *testing.T) {
	bus := events.NewEventBus()
	provider := NewChatCtxManager(bus)

	respond := func(e events.ChatResponseEvent) {
		bus.PublishSync(e.Topic(), e)
	}

	respond(events.ChatResponseEvent{
		Message:  "first question",
		Response: "complete answer",
	})
	respond(events.ChatResponseEvent{
		Message:  "second question",
		Response: "partial answ", // truncated by cancellation
		Error:    context.Canceled,
	})
	respond(events.ChatResponseEvent{
		Message:  "third question",
		Response: "",
		Error:    errors.New("api exploded"),
	})

	part, err := provider.GetPart(context.Background())
	require.NoError(t, err)

	assert.Contains(t, part.Content, "complete answer")
	assert.NotContains(t, part.Content, "partial answ", "cancelled partial output must not be stored")
	assert.NotContains(t, part.Content, "second question", "a turn that never completed must not be stored")
	assert.NotContains(t, part.Content, "third question", "a failed turn must not be stored")
}
