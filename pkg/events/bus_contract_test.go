package events

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The bus must never drop events: every published event is delivered,
// in order, even when publishing outpaces a slow handler.
func TestPublishIsLosslessUnderBurst(t *testing.T) {
	bus := NewEventBus()
	defer bus.(*InMemoryBus).Shutdown()

	const total = 5000 // well above any internal buffer size

	var mu sync.Mutex
	received := make([]int, 0, total)
	done := make(chan struct{})

	bus.Subscribe("burst.topic", func(event interface{}) {
		// Slow consumer: force the publisher to run far ahead.
		if len(received) == 0 {
			time.Sleep(50 * time.Millisecond)
		}
		mu.Lock()
		received = append(received, event.(int))
		if len(received) == total {
			close(done)
		}
		mu.Unlock()
	})

	for i := 0; i < total; i++ {
		bus.Publish("burst.topic", i)
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		mu.Lock()
		got := len(received)
		mu.Unlock()
		t.Fatalf("timed out: delivered %d of %d events (events were dropped)", got, total)
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, total)
	for i, v := range received {
		require.Equal(t, i, v, "events must be delivered in publish order")
	}
}

// Subscribe must return an unsubscribe function so subscribers with a
// bounded lifetime (dialogs, per-request waiters) can detach.
func TestSubscribeReturnsUnsubscribe(t *testing.T) {
	bus := NewEventBus()
	defer bus.(*InMemoryBus).Shutdown()

	var mu sync.Mutex
	calls := 0
	unsubscribe := bus.Subscribe("some.topic", func(event interface{}) {
		mu.Lock()
		calls++
		mu.Unlock()
	})
	require.NotNil(t, unsubscribe)

	bus.PublishSync("some.topic", "one")
	unsubscribe()
	bus.PublishSync("some.topic", "two")

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, calls, "handler must not fire after unsubscribe")
}

// Unsubscribing one handler must not detach others on the same topic.
func TestUnsubscribeLeavesOtherHandlersAttached(t *testing.T) {
	bus := NewEventBus()
	defer bus.(*InMemoryBus).Shutdown()

	var mu sync.Mutex
	first, second := 0, 0

	unsubFirst := bus.Subscribe("topic", func(event interface{}) {
		mu.Lock()
		first++
		mu.Unlock()
	})
	bus.Subscribe("topic", func(event interface{}) {
		mu.Lock()
		second++
		mu.Unlock()
	})

	unsubFirst()
	bus.PublishSync("topic", struct{}{})

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 0, first)
	assert.Equal(t, 1, second)
}

// SubscriberCount supports leak assertions: long-lived components must
// not accumulate handlers over repeated operations.
func TestSubscriberCount(t *testing.T) {
	bus := NewEventBus()
	defer bus.(*InMemoryBus).Shutdown()

	inMem := bus.(*InMemoryBus)
	assert.Equal(t, 0, inMem.SubscriberCount("topic"))

	unsub := bus.Subscribe("topic", func(event interface{}) {})
	bus.Subscribe("topic", func(event interface{}) {})
	assert.Equal(t, 2, inMem.SubscriberCount("topic"))

	unsub()
	assert.Equal(t, 1, inMem.SubscriberCount("topic"))
}
