package events

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEventBus_OrderedDeliveryPerTopic(t *testing.T) {
	bus := NewEventBusWithBuffer(8).(*InMemoryBus)
	defer bus.Shutdown()

	var mu sync.Mutex
	var received []int
	var wg sync.WaitGroup
	wg.Add(3)

	bus.Subscribe("test.event", func(event interface{}) {
		defer wg.Done()
		mu.Lock()
		received = append(received, event.(int))
		mu.Unlock()
	})

	bus.Publish("test.event", 1)
	bus.Publish("test.event", 2)
	bus.Publish("test.event", 3)

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{1, 2, 3}, received)
}

func TestEventBus_MultipleHandlersPreserveOrder(t *testing.T) {
	bus := NewEventBusWithBuffer(4).(*InMemoryBus)
	defer bus.Shutdown()

	var mu sync.Mutex
	var calls []string
	var wg sync.WaitGroup
	wg.Add(2)

	bus.Subscribe("test.event", func(event interface{}) {
		defer wg.Done()
		mu.Lock()
		calls = append(calls, "first")
		mu.Unlock()
	})

	bus.Subscribe("test.event", func(event interface{}) {
		defer wg.Done()
		mu.Lock()
		calls = append(calls, "second")
		mu.Unlock()
	})

	bus.Publish("test.event", "payload")
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"first", "second"}, calls)
}

func TestEventBus_DropsWhenQueueIsFull(t *testing.T) {
	bus := NewEventBusWithBuffer(1).(*InMemoryBus)
	defer bus.Shutdown()

	var processed atomic.Int64
	var wg sync.WaitGroup
	wg.Add(2) // expect to process two events

	blocker := make(chan struct{})
	started := make(chan struct{})

	bus.Subscribe("test.event", func(event interface{}) {
		id := event.(int)
		if id == 1 {
			close(started) // signal the first event is being handled
			<-blocker      // block to keep the worker busy
		}
		processed.Add(1)
		wg.Done()
	})

	bus.Publish("test.event", 1) // will block in handler
	<-started                    // ensure handler is running

	// This fills the queue while the worker is busy.
	bus.Publish("test.event", 2)
	// This publish should be dropped due to full queue.
	bus.Publish("test.event", 3)

	close(blocker) // allow the worker to drain

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for handler completion")
	}

	assert.Equal(t, int64(2), processed.Load())
	assert.Equal(t, int64(1), bus.DroppedCount())
}

func TestEventBus_RecoversFromHandlerPanic(t *testing.T) {
	bus := NewEventBusWithBuffer(4).(*InMemoryBus)
	defer bus.Shutdown()

	var sum atomic.Int64
	var wg sync.WaitGroup
	wg.Add(2) // two successful handler invocations expected

	bus.Subscribe("test.event", func(event interface{}) {
		panic("boom")
	})

	bus.Subscribe("test.event", func(event interface{}) {
		defer wg.Done()
		sum.Add(int64(event.(int)))
	})

	bus.Publish("test.event", 1)
	bus.Publish("test.event", 2)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for handler completion")
	}

	assert.Equal(t, int64(3), sum.Load())
}

func TestEventBus_MultipleEventTypesIsolation(t *testing.T) {
	bus := NewEventBusWithBuffer(4).(*InMemoryBus)
	defer bus.Shutdown()

	var mu sync.Mutex
	var typeA []string
	var typeB []string
	var wg sync.WaitGroup
	wg.Add(2)

	bus.Subscribe("type.a", func(event interface{}) {
		defer wg.Done()
		mu.Lock()
		typeA = append(typeA, event.(string))
		mu.Unlock()
	})

	bus.Subscribe("type.b", func(event interface{}) {
		defer wg.Done()
		mu.Lock()
		typeB = append(typeB, event.(string))
		mu.Unlock()
	})

	bus.Publish("type.a", "a1")
	bus.Publish("type.b", "b1")

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"a1"}, typeA)
	assert.Equal(t, []string{"b1"}, typeB)
}
