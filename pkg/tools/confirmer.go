package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/kcaldas/genie/pkg/events"
)

// Confirmer requests a user decision and blocks until it is answered
// or ctx is done. Tools ask for confirmation through this interface
// instead of hand-rolling pub/sub correlation per call site.
type Confirmer interface {
	// ConfirmContent asks the user to approve content, e.g. a diff preview.
	ConfirmContent(ctx context.Context, req events.UserConfirmationRequest) (bool, error)
	// ConfirmExecution asks the user to approve running a command.
	ConfirmExecution(ctx context.Context, req events.ToolConfirmationRequest) (bool, error)
}

// BusConfirmer implements Confirmer over the event bus. It subscribes
// to each response topic exactly once and correlates answers to waiting
// requests by execution ID, so repeated confirmations never accumulate
// handlers.
type BusConfirmer struct {
	bus events.EventBus

	mu      sync.Mutex
	waiting map[string]chan bool
}

// NewBusConfirmer creates a Confirmer over the given bus.
func NewBusConfirmer(bus events.EventBus) *BusConfirmer {
	c := &BusConfirmer{
		bus:     bus,
		waiting: make(map[string]chan bool),
	}
	events.SubscribeTo(bus, func(resp events.UserConfirmationResponse) {
		c.deliver(resp.ExecutionID, resp.Confirmed)
	})
	events.SubscribeTo(bus, func(resp events.ToolConfirmationResponse) {
		c.deliver(resp.ExecutionID, resp.Confirmed)
	})
	return c
}

// ConfirmContent publishes a user.confirmation.request and waits for
// the matching user.confirmation.response.
func (c *BusConfirmer) ConfirmContent(ctx context.Context, req events.UserConfirmationRequest) (bool, error) {
	answer, cleanup, err := c.register(req.ExecutionID)
	if err != nil {
		return false, err
	}
	defer cleanup()

	c.bus.Publish(req.Topic(), req)
	return c.await(ctx, answer)
}

// ConfirmExecution publishes a tool.confirmation.request and waits for
// the matching tool.confirmation.response.
func (c *BusConfirmer) ConfirmExecution(ctx context.Context, req events.ToolConfirmationRequest) (bool, error) {
	answer, cleanup, err := c.register(req.ExecutionID)
	if err != nil {
		return false, err
	}
	defer cleanup()

	c.bus.Publish(req.Topic(), req)
	return c.await(ctx, answer)
}

func (c *BusConfirmer) register(executionID string) (chan bool, func(), error) {
	if executionID == "" {
		return nil, nil, fmt.Errorf("confirmation request requires an execution ID")
	}

	answer := make(chan bool, 1)
	c.mu.Lock()
	if _, exists := c.waiting[executionID]; exists {
		c.mu.Unlock()
		return nil, nil, fmt.Errorf("confirmation already pending for execution ID %s", executionID)
	}
	c.waiting[executionID] = answer
	c.mu.Unlock()

	cleanup := func() {
		c.mu.Lock()
		delete(c.waiting, executionID)
		c.mu.Unlock()
	}
	return answer, cleanup, nil
}

func (c *BusConfirmer) await(ctx context.Context, answer chan bool) (bool, error) {
	select {
	case confirmed := <-answer:
		return confirmed, nil
	case <-ctx.Done():
		return false, fmt.Errorf("confirmation aborted: %w", ctx.Err())
	}
}

func (c *BusConfirmer) deliver(executionID string, confirmed bool) {
	c.mu.Lock()
	ch, ok := c.waiting[executionID]
	c.mu.Unlock()
	if !ok {
		return // response for a request we are not waiting on
	}
	select {
	case ch <- confirmed:
	default: // already answered
	}
}
