package anthropic

import (
	"context"
	"testing"
	"time"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// Usage events must carry the chat request ID attached to the context by the
// core, so a host running concurrent invocations can attribute token spend.
func TestPublishUsageCarriesRequestIDFromContext(t *testing.T) {
	bus := events.NewEventBus()
	received := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.TokenCountEvent); ok {
			received <- event
		}
	})

	rawClient, err := NewClient(bus)
	assert.NoError(t, err)
	client := rawClient.(*Client)

	ctx := ai.ContextWithRequestID(context.Background(), "req-7")
	client.publishUsage(ctx, "claude-test", anthropic_sdk.Usage{InputTokens: 12, OutputTokens: 4})

	select {
	case event := <-received:
		assert.Equal(t, "req-7", event.RequestID)
		assert.Equal(t, "anthropic", event.Provider)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for token count event")
	}
}
