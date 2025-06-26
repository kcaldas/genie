package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventBus_Subscribe_Publish(t *testing.T) {
	bus := NewEventBus()

	// Track received events
	var receivedEvents []interface{}
	var secondReceived []interface{}

	// Subscribe first handler
	bus.Subscribe("test.event", func(event interface{}) {
		receivedEvents = append(receivedEvents, event)
	})

	// Subscribe second handler to same event type
	bus.Subscribe("test.event", func(event interface{}) {
		secondReceived = append(secondReceived, event)
	})

	// Publish an event
	testEvent := ChatStartedEvent{
		SessionID: "test-session",
		Message:   "Hello, Genie!",
	}

	bus.Publish("test.event", testEvent)

	// Both handlers should have received the event
	assert.Len(t, receivedEvents, 1)
	assert.Len(t, secondReceived, 1)
	assert.Equal(t, testEvent, receivedEvents[0])
	assert.Equal(t, testEvent, secondReceived[0])
}

func TestEventBus_MultipleEventTypes(t *testing.T) {
	bus := NewEventBus()

	var typeAEvents []interface{}
	var typeBEvents []interface{}

	// Subscribe to different event types
	bus.Subscribe("type.a", func(event interface{}) {
		typeAEvents = append(typeAEvents, event)
	})

	bus.Subscribe("type.b", func(event interface{}) {
		typeBEvents = append(typeBEvents, event)
	})

	// Publish to type A
	bus.Publish("type.a", "event-a")

	// Publish to type B
	bus.Publish("type.b", "event-b")

	// Only appropriate handlers should receive events
	assert.Len(t, typeAEvents, 1)
	assert.Equal(t, "event-a", typeAEvents[0])

	assert.Len(t, typeBEvents, 1)
	assert.Equal(t, "event-b", typeBEvents[0])
}

func TestEventBus_NoSubscribers(t *testing.T) {
	bus := NewEventBus()

	// Publishing to non-existent event type should not panic
	assert.NotPanics(t, func() {
		bus.Publish("non.existent", "test")
	})
}

