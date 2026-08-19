package shared

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Token events published by the local providers (Ollama, LM Studio) must
// carry the chat request ID from the context so hosts can attribute usage
// per invocation.
func TestPublishTokenCountCarriesRequestIDFromContext(t *testing.T) {
	bus := events.NewEventBus()
	received := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.TokenCountEvent); ok {
			received <- event
		}
	})

	core := NewLocalClientCore("ollama", bus)
	ctx := ai.ContextWithRequestID(context.Background(), "req-9")
	core.PublishTokenCount(ctx, &ai.TokenCount{InputTokens: 10, OutputTokens: 3, TotalTokens: 13})

	select {
	case event := <-received:
		assert.Equal(t, "req-9", event.RequestID)
		assert.Equal(t, "ollama", event.Provider)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for token count event")
	}
}

// A token count requested outside any chat invocation carries no request ID.
func TestPublishTokenCountWithoutRequestID(t *testing.T) {
	bus := events.NewEventBus()
	received := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.TokenCountEvent); ok {
			received <- event
		}
	})

	core := NewLocalClientCore("lmstudio", bus)
	core.PublishTokenCount(context.Background(), &ai.TokenCount{TotalTokens: 5})

	select {
	case event := <-received:
		require.Empty(t, event.RequestID)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for token count event")
	}
}
