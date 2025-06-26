package events

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventBus_Subscribe_Publish(t *testing.T) {
	bus := NewEventBus()

	// Track received events with thread-safe access
	var mu sync.Mutex
	var receivedEvents []interface{}
	var secondReceived []interface{}
	var wg sync.WaitGroup

	// Subscribe first handler
	wg.Add(1)
	bus.Subscribe("test.event", func(event interface{}) {
		mu.Lock()
		receivedEvents = append(receivedEvents, event)
		mu.Unlock()
		wg.Done()
	})

	// Subscribe second handler to same event type
	wg.Add(1)
	bus.Subscribe("test.event", func(event interface{}) {
		mu.Lock()
		secondReceived = append(secondReceived, event)
		mu.Unlock()
		wg.Done()
	})

	// Publish an event
	testEvent := ChatStartedEvent{
		Message: "Hello, Genie!",
	}

	bus.Publish("test.event", testEvent)

	// Wait for async handlers to complete
	wg.Wait()

	// Both handlers should have received the event
	mu.Lock()
	assert.Len(t, receivedEvents, 1)
	assert.Len(t, secondReceived, 1)
	assert.Equal(t, testEvent, receivedEvents[0])
	assert.Equal(t, testEvent, secondReceived[0])
	mu.Unlock()
}

func TestEventBus_MultipleEventTypes(t *testing.T) {
	bus := NewEventBus()

	var mu sync.Mutex
	var typeAEvents []interface{}
	var typeBEvents []interface{}
	var wg sync.WaitGroup

	// Subscribe to different event types
	wg.Add(1)
	bus.Subscribe("type.a", func(event interface{}) {
		mu.Lock()
		typeAEvents = append(typeAEvents, event)
		mu.Unlock()
		wg.Done()
	})

	wg.Add(1)
	bus.Subscribe("type.b", func(event interface{}) {
		mu.Lock()
		typeBEvents = append(typeBEvents, event)
		mu.Unlock()
		wg.Done()
	})

	// Publish to type A
	bus.Publish("type.a", "event-a")

	// Publish to type B
	bus.Publish("type.b", "event-b")

	// Wait for async handlers to complete
	wg.Wait()

	// Only appropriate handlers should receive events
	mu.Lock()
	assert.Len(t, typeAEvents, 1)
	assert.Equal(t, "event-a", typeAEvents[0])

	assert.Len(t, typeBEvents, 1)
	assert.Equal(t, "event-b", typeBEvents[0])
	mu.Unlock()
}

func TestEventBus_NoSubscribers(t *testing.T) {
	bus := NewEventBus()

	// Publishing to non-existent event type should not panic
	assert.NotPanics(t, func() {
		bus.Publish("non.existent", "test")
	})
}
