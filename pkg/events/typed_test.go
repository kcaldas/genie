package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscribeToDeliversTypedEvents(t *testing.T) {
	bus := NewEventBus().(*InMemoryBus)
	defer bus.Shutdown()

	var got []ChatResponseEvent
	SubscribeTo(bus, func(e ChatResponseEvent) {
		got = append(got, e)
	})

	event := ChatResponseEvent{RequestID: "r1", Response: "hello"}
	bus.PublishSync(event.Topic(), event)

	assert.Len(t, got, 1)
	assert.Equal(t, "hello", got[0].Response)
}

func TestSubscribeToIgnoresMistypedPayloads(t *testing.T) {
	bus := NewEventBus().(*InMemoryBus)
	defer bus.Shutdown()

	calls := 0
	SubscribeTo(bus, func(e ChatResponseEvent) { calls++ })

	// A payload of the wrong type on the same topic must be ignored,
	// not panic.
	bus.PublishSync(ChatResponseEvent{}.Topic(), "not-a-chat-response")

	assert.Equal(t, 0, calls)
}

func TestSubscribeToUnsubscribes(t *testing.T) {
	bus := NewEventBus().(*InMemoryBus)
	defer bus.Shutdown()

	calls := 0
	unsub := SubscribeTo(bus, func(e ChatStartedEvent) { calls++ })

	event := ChatStartedEvent{RequestID: "r1"}
	bus.PublishSync(event.Topic(), event)
	unsub()
	bus.PublishSync(event.Topic(), event)

	assert.Equal(t, 1, calls)
}
