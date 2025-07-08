package events

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test event types for testing
type TestEvent struct {
	Message string
}

type TestEventWithData struct {
	ID   int
	Name string
}

func TestCommandEventBus_Subscribe_And_Emit(t *testing.T) {
	bus := NewCommandEventBus()

	t.Run("subscriber receives emitted event", func(t *testing.T) {
		done := make(chan bool)
		var receivedEvent interface{}

		// Subscribe to TestEvent
		bus.Subscribe("test.event", func(event interface{}) {
			receivedEvent = event
			done <- true
		})

		// Emit event
		testEvent := TestEvent{Message: "hello"}
		bus.Emit("test.event", testEvent)

		// Wait for event to be received
		select {
		case <-done:
			assert.Equal(t, testEvent, receivedEvent)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Event was not received within timeout")
		}
	})

	t.Run("multiple subscribers receive same event", func(t *testing.T) {
		bus := NewCommandEventBus()
		
		var wg sync.WaitGroup
		received1 := false
		received2 := false

		wg.Add(2)

		// First subscriber
		bus.Subscribe("multi.event", func(event interface{}) {
			received1 = true
			wg.Done()
		})

		// Second subscriber  
		bus.Subscribe("multi.event", func(event interface{}) {
			received2 = true
			wg.Done()
		})

		// Emit event
		bus.Emit("multi.event", TestEvent{Message: "broadcast"})

		// Wait for both subscribers
		wg.Wait()

		assert.True(t, received1, "First subscriber should have received the event")
		assert.True(t, received2, "Second subscriber should have received the event")
	})

	t.Run("subscriber only receives subscribed event type", func(t *testing.T) {
		bus := NewCommandEventBus()
		
		done := make(chan bool)
		receivedCorrect := false
		receivedWrong := false

		// Subscribe to specific event
		bus.Subscribe("specific.event", func(event interface{}) {
			receivedCorrect = true
			done <- true
		})

		// Subscribe to different event
		bus.Subscribe("other.event", func(event interface{}) {
			receivedWrong = true
		})

		// Emit only the first event
		bus.Emit("specific.event", TestEvent{Message: "specific"})

		// Wait for handler to execute
		select {
		case <-done:
			// Handler completed
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Handler did not execute")
		}

		assert.True(t, receivedCorrect, "Should receive subscribed event")
		assert.False(t, receivedWrong, "Should not receive unsubscribed event")
	})

	t.Run("unsubscribe stops receiving events", func(t *testing.T) {
		bus := NewCommandEventBus()
		
		callCount := 0
		done := make(chan bool)
		
		// Subscribe and get unsubscribe function
		unsubscribe := bus.Subscribe("unsub.event", func(event interface{}) {
			callCount++
			done <- true
		})

		// Emit first event - should be received
		bus.Emit("unsub.event", TestEvent{})
		
		// Wait for handler to execute
		select {
		case <-done:
			// Handler completed
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Handler did not execute")
		}
		
		assert.Equal(t, 1, callCount)

		// Unsubscribe
		unsubscribe()

		// Emit second event - should not be received
		bus.Emit("unsub.event", TestEvent{})
		
		// Give some time for potential handler to execute
		time.Sleep(10 * time.Millisecond)
		
		assert.Equal(t, 1, callCount, "Should not receive events after unsubscribe")
	})

	t.Run("emit with no subscribers does not panic", func(t *testing.T) {
		bus := NewCommandEventBus()
		
		// Should not panic
		assert.NotPanics(t, func() {
			bus.Emit("no.subscribers", TestEvent{Message: "alone"})
		})
	})

	t.Run("concurrent emit and subscribe is safe", func(t *testing.T) {
		bus := NewCommandEventBus()
		
		var wg sync.WaitGroup
		eventCount := 0
		var mu sync.Mutex
		done := make(chan bool)

		// Subscribe
		bus.Subscribe("concurrent.event", func(event interface{}) {
			mu.Lock()
			eventCount++
			if eventCount == 100 {
				done <- true
			}
			mu.Unlock()
		})

		// Emit events concurrently
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				bus.Emit("concurrent.event", TestEventWithData{ID: id})
			}(i)
		}

		wg.Wait()

		// Wait for all handlers to complete
		select {
		case <-done:
			// All handlers completed
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Not all handlers completed")
		}

		mu.Lock()
		assert.Equal(t, 100, eventCount, "All events should be received")
		mu.Unlock()
	})

	t.Run("event data is passed correctly", func(t *testing.T) {
		bus := NewCommandEventBus()
		
		var receivedData TestEventWithData
		done := make(chan bool)

		bus.Subscribe("data.event", func(event interface{}) {
			if data, ok := event.(TestEventWithData); ok {
				receivedData = data
			}
			done <- true
		})

		expectedData := TestEventWithData{ID: 42, Name: "test"}
		bus.Emit("data.event", expectedData)

		// Wait for handler to execute
		select {
		case <-done:
			// Handler completed
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Handler did not execute")
		}

		assert.Equal(t, expectedData.ID, receivedData.ID)
		assert.Equal(t, expectedData.Name, receivedData.Name)
	})

	t.Run("handlers execute asynchronously", func(t *testing.T) {
		bus := NewCommandEventBus()
		
		handlerStarted := make(chan bool)
		handlerCompleted := make(chan bool)

		bus.Subscribe("async.event", func(event interface{}) {
			handlerStarted <- true
			time.Sleep(50 * time.Millisecond) // Simulate work
			handlerCompleted <- true
		})

		// Emit should return immediately
		start := time.Now()
		bus.Emit("async.event", TestEvent{})
		emitDuration := time.Since(start)

		// Emit should be fast (not wait for handler)
		assert.Less(t, emitDuration, 10*time.Millisecond, "Emit should return quickly")

		// Wait for handler to start and complete
		select {
		case <-handlerStarted:
			// Handler started
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Handler did not start")
		}

		select {
		case <-handlerCompleted:
			// Handler completed
		case <-time.After(200 * time.Millisecond):
			t.Fatal("Handler did not complete")
		}
	})
}

func TestCommandEventBus_Clear(t *testing.T) {
	bus := NewCommandEventBus()
	
	received := false

	// Subscribe to event
	bus.Subscribe("clear.event", func(event interface{}) {
		received = true
	})

	// Clear all subscriptions
	bus.Clear()

	// Emit event - should not be received
	bus.Emit("clear.event", TestEvent{})

	assert.False(t, received, "Should not receive event after clear")
}

func TestCommandEventBus_SubscribeOnce(t *testing.T) {
	bus := NewCommandEventBus()
	
	callCount := 0
	done := make(chan bool)

	// Subscribe once
	bus.SubscribeOnce("once.event", func(event interface{}) {
		callCount++
		done <- true
	})

	// Emit multiple times
	bus.Emit("once.event", TestEvent{})
	
	// Wait for first handler to execute
	select {
	case <-done:
		// Handler completed
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Handler did not execute")
	}
	
	bus.Emit("once.event", TestEvent{})
	bus.Emit("once.event", TestEvent{})
	
	// Give some time for potential handlers to execute
	time.Sleep(10 * time.Millisecond)

	// Should only be called once
	assert.Equal(t, 1, callCount, "SubscribeOnce handler should only be called once")
}

// Benchmark to ensure performance
func BenchmarkCommandEventBus_Emit(b *testing.B) {
	bus := NewCommandEventBus()
	
	// Add some subscribers
	for i := 0; i < 10; i++ {
		bus.Subscribe("bench.event", func(event interface{}) {
			// Do nothing - just testing emit performance
		})
	}

	event := TestEvent{Message: "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Emit("bench.event", event)
	}
}

// Test common command event scenarios
func TestCommandEventBus_RealWorldScenarios(t *testing.T) {
	t.Run("focus change event flow", func(t *testing.T) {
		bus := NewCommandEventBus()
		
		var focusHistory []string
		var mu sync.Mutex
		eventCount := 0
		done := make(chan bool)
		
		// Component subscribes to focus events
		bus.Subscribe("focus.changed", func(event interface{}) {
			if focusEvent, ok := event.(map[string]string); ok {
				mu.Lock()
				focusHistory = append(focusHistory, focusEvent["to"])
				eventCount++
				if eventCount == 2 {
					done <- true
				}
				mu.Unlock()
			}
		})

		// Simulate focus changes
		bus.Emit("focus.changed", map[string]string{"from": "input", "to": "messages"})
		bus.Emit("focus.changed", map[string]string{"from": "messages", "to": "debug"})
		
		// Wait for both handlers to execute
		select {
		case <-done:
			// All handlers completed
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Handlers did not execute")
		}
		
		mu.Lock()
		require.Len(t, focusHistory, 2)
		// Since handlers run async, we need to check both possible orderings
		if focusHistory[0] == "messages" && focusHistory[1] == "debug" {
			// Expected order
			assert.Equal(t, "messages", focusHistory[0])
			assert.Equal(t, "debug", focusHistory[1])
		} else if focusHistory[0] == "debug" && focusHistory[1] == "messages" {
			// Reverse order due to async execution
			assert.Equal(t, "debug", focusHistory[0])
			assert.Equal(t, "messages", focusHistory[1])
		} else {
			t.Fatalf("Unexpected focus history: %v", focusHistory)
		}
		mu.Unlock()
	})

	t.Run("user input event flow", func(t *testing.T) {
		bus := NewCommandEventBus()
		
		var commands []string
		var messages []string
		eventCount := 0
		done := make(chan bool)

		// Command handler subscribes
		bus.Subscribe("user.command", func(event interface{}) {
			if cmd, ok := event.(map[string]string); ok {
				commands = append(commands, cmd["command"])
				eventCount++
				if eventCount == 2 {
					done <- true
				}
			}
		})

		// Chat handler subscribes  
		bus.Subscribe("user.message", func(event interface{}) {
			if msg, ok := event.(map[string]string); ok {
				messages = append(messages, msg["text"])
				eventCount++
				if eventCount == 2 {
					done <- true
				}
			}
		})

		// Input component emits based on input
		bus.Emit("user.command", map[string]string{"command": ":help"})
		bus.Emit("user.message", map[string]string{"text": "Hello AI"})

		// Wait for both handlers to execute
		select {
		case <-done:
			// All handlers completed
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Handlers did not execute")
		}

		assert.Equal(t, []string{":help"}, commands)
		assert.Equal(t, []string{"Hello AI"}, messages)
	})
}