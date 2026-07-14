package tools

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func answerContentRequests(bus events.EventBus, confirmed bool) {
	events.SubscribeTo(bus, func(req events.UserConfirmationRequest) {
		bus.Publish(events.UserConfirmationResponse{}.Topic(), events.UserConfirmationResponse{
			ExecutionID: req.ExecutionID,
			Confirmed:   confirmed,
		})
	})
}

func answerExecutionRequests(bus events.EventBus, confirmed bool) {
	events.SubscribeTo(bus, func(req events.ToolConfirmationRequest) {
		bus.Publish(events.ToolConfirmationResponse{}.Topic(), events.ToolConfirmationResponse{
			ExecutionID: req.ExecutionID,
			Confirmed:   confirmed,
		})
	})
}

func TestBusConfirmerConfirmContent(t *testing.T) {
	bus := events.NewEventBus()
	confirmer := NewBusConfirmer(bus)
	answerContentRequests(bus, true)

	ok, err := confirmer.ConfirmContent(context.Background(), events.UserConfirmationRequest{
		ExecutionID: "exec-1",
		Title:       "writeFile",
	})
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestBusConfirmerConfirmExecutionDenied(t *testing.T) {
	bus := events.NewEventBus()
	confirmer := NewBusConfirmer(bus)
	answerExecutionRequests(bus, false)

	ok, err := confirmer.ConfirmExecution(context.Background(), events.ToolConfirmationRequest{
		ExecutionID: "exec-1",
		ToolName:    "bash",
	})
	require.NoError(t, err)
	assert.False(t, ok)
}

// Repeated confirmations must not accumulate bus handlers — this was
// the WriteTool leak: one new subscription per confirmation, forever.
func TestBusConfirmerDoesNotLeakHandlers(t *testing.T) {
	bus := events.NewEventBus()
	inMem := bus.(*events.InMemoryBus)

	confirmer := NewBusConfirmer(bus)
	answerContentRequests(bus, true)

	baseline := inMem.SubscriberCount(events.UserConfirmationResponse{}.Topic())

	for i := 0; i < 25; i++ {
		ok, err := confirmer.ConfirmContent(context.Background(), events.UserConfirmationRequest{
			ExecutionID: fmt.Sprintf("exec-%d", i),
		})
		require.NoError(t, err)
		require.True(t, ok)
	}

	assert.Equal(t, baseline, inMem.SubscriberCount(events.UserConfirmationResponse{}.Topic()),
		"confirmations must not add handlers")
}

// Concurrent confirmations on one confirmer must each receive their own
// answer, correlated by execution ID.
func TestBusConfirmerCorrelatesConcurrentRequests(t *testing.T) {
	bus := events.NewEventBus()
	confirmer := NewBusConfirmer(bus)

	// Answer true only for even execution IDs.
	events.SubscribeTo(bus, func(req events.ToolConfirmationRequest) {
		confirmed := req.Command == "even"
		bus.Publish(events.ToolConfirmationResponse{}.Topic(), events.ToolConfirmationResponse{
			ExecutionID: req.ExecutionID,
			Confirmed:   confirmed,
		})
	})

	var wg sync.WaitGroup
	results := make([]bool, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			command := "odd"
			if i%2 == 0 {
				command = "even"
			}
			ok, err := confirmer.ConfirmExecution(context.Background(), events.ToolConfirmationRequest{
				ExecutionID: fmt.Sprintf("exec-%d", i),
				Command:     command,
			})
			require.NoError(t, err)
			results[i] = ok
		}(i)
	}
	wg.Wait()

	for i, got := range results {
		assert.Equal(t, i%2 == 0, got, "request %d received the wrong answer", i)
	}
}

func TestBusConfirmerRespectsContextCancellation(t *testing.T) {
	bus := events.NewEventBus()
	confirmer := NewBusConfirmer(bus)
	// Nobody answers.

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	_, err := confirmer.ConfirmContent(ctx, events.UserConfirmationRequest{ExecutionID: "exec-1"})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
