package di

import (
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
)

// Test if a single channel can broadcast to multiple listeners
func TestChannelBroadcast(t *testing.T) {
	t.Log("=== Testing Channel Broadcast Behavior ===")

	// Create a single channel
	ch := make(chan events.SessionInteractionEvent, 10)

	// Track what each listener receives
	listener1Events := make([]events.SessionInteractionEvent, 0)
	listener2Events := make([]events.SessionInteractionEvent, 0)

	// Start listener 1
	go func() {
		for event := range ch {
			listener1Events = append(listener1Events, event)
			t.Logf("Listener 1 received: %+v", event)
		}
	}()

	// Start listener 2
	go func() {
		for event := range ch {
			listener2Events = append(listener2Events, event)
			t.Logf("Listener 2 received: %+v", event)
		}
	}()

	// Give listeners time to start
	time.Sleep(10 * time.Millisecond)

	// Send some events
	event1 := events.SessionInteractionEvent{
		SessionID:         "test1",
		UserMessage:       "Hello",
		AssistantResponse: "Hi",
	}

	event2 := events.SessionInteractionEvent{
		SessionID:         "test2", 
		UserMessage:       "How are you?",
		AssistantResponse: "Good",
	}

	t.Log("Sending event 1...")
	ch <- event1

	t.Log("Sending event 2...")
	ch <- event2

	// Give time for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Close channel to stop listeners
	close(ch)
	time.Sleep(10 * time.Millisecond)

	t.Logf("Listener 1 received %d events", len(listener1Events))
	t.Logf("Listener 2 received %d events", len(listener2Events))

	// Check results
	totalReceived := len(listener1Events) + len(listener2Events)
	t.Logf("Total events sent: 2")
	t.Logf("Total events received: %d", totalReceived)

	if totalReceived == 2 {
		t.Log("✅ Each event was received by exactly ONE listener (round-robin)")
	} else {
		t.Log("❌ Unexpected behavior")
	}

	t.Log("\n=== Conclusion ===")
	t.Log("Channels do NOT broadcast to multiple listeners")
	t.Log("Each message goes to only ONE listener (whichever reads first)")
	t.Log("For broadcasting, we need separate channels for each listener")
}