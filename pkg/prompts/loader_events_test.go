package prompts

import (
	"context"
	"errors"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tool.executed events must carry a typed success flag so consumers do
// not have to sniff the human-readable message for a "Failed:" prefix.
func TestWrapHandlerWithEventsPublishesTypedOutcome(t *testing.T) {
	tests := []struct {
		name        string
		handlerErr  error
		toolError   bool
		wantSuccess bool
		wantMessage string
	}{
		{name: "success", handlerErr: nil, wantSuccess: true, wantMessage: "Executed"},
		{name: "handler failure", handlerErr: errors.New("boom"), wantSuccess: false, wantMessage: "Failed: boom"},
		{name: "tool failure", toolError: true, wantSuccess: false, wantMessage: "Failed: invalid input"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := events.NewEventBus()
			var executed []events.ToolExecutedEvent
			events.SubscribeTo(bus, func(e events.ToolExecutedEvent) {
				executed = append(executed, e)
			})

			loader := &DefaultLoader{Publisher: bus}
			handler := loader.wrapHandlerWithEvents("myTool", func(ctx context.Context, params map[string]any) (ai.ToolOutput, error) {
				if tt.toolError {
					return ai.ErrorToolOutput(map[string]any{"error": "invalid input"}), nil
				}
				return ai.JSONToolOutput(map[string]any{"ok": true}), tt.handlerErr
			})

			_, err := handler(context.Background(), map[string]any{})
			if tt.handlerErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Len(t, executed, 1, "tool.executed must be published exactly once")
			assert.Equal(t, tt.wantSuccess, executed[0].Success)
			assert.Equal(t, tt.wantMessage, executed[0].Message)
		})
	}
}

// Tool events must carry the chat request ID from the context so
// consumers can correlate tool activity with the turn that caused it.
func TestWrapHandlerWithEventsCarriesRequestID(t *testing.T) {
	bus := events.NewEventBus()
	var started []events.ToolStartingEvent
	var executed []events.ToolExecutedEvent
	events.SubscribeTo(bus, func(e events.ToolStartingEvent) {
		started = append(started, e)
	})
	events.SubscribeTo(bus, func(e events.ToolExecutedEvent) {
		executed = append(executed, e)
	})

	loader := &DefaultLoader{Publisher: bus}
	handler := loader.wrapHandlerWithEvents("myTool", func(ctx context.Context, params map[string]any) (ai.ToolOutput, error) {
		return ai.JSONToolOutput(map[string]any{"ok": true}), nil
	})

	ctx := ai.ContextWithRequestID(context.Background(), "req-42")
	_, err := handler(ctx, map[string]any{})
	require.NoError(t, err)

	require.Len(t, started, 1)
	require.Len(t, executed, 1)
	assert.Equal(t, "req-42", started[0].RequestID)
	assert.Equal(t, "req-42", executed[0].RequestID)
}

// Without a request ID in the context (e.g. tools run outside a chat),
// the events carry an empty RequestID rather than a placeholder.
func TestWrapHandlerWithEventsWithoutRequestID(t *testing.T) {
	bus := events.NewEventBus()
	var executed []events.ToolExecutedEvent
	events.SubscribeTo(bus, func(e events.ToolExecutedEvent) {
		executed = append(executed, e)
	})

	loader := &DefaultLoader{Publisher: bus}
	handler := loader.wrapHandlerWithEvents("myTool", func(ctx context.Context, params map[string]any) (ai.ToolOutput, error) {
		return ai.JSONToolOutput(map[string]any{"ok": true}), nil
	})

	_, err := handler(context.Background(), map[string]any{})
	require.NoError(t, err)

	require.Len(t, executed, 1)
	assert.Empty(t, executed[0].RequestID)
}

// A panicking tool handler must fail the tool call, not crash the
// process: in streaming mode handlers run inside producer goroutines
// where an unrecovered panic kills the whole TUI.
func TestWrapHandlerWithEventsRecoversPanics(t *testing.T) {
	bus := events.NewEventBus()
	var executed []events.ToolExecutedEvent
	events.SubscribeTo(bus, func(e events.ToolExecutedEvent) {
		executed = append(executed, e)
	})

	loader := &DefaultLoader{Publisher: bus}
	handler := loader.wrapHandlerWithEvents("explodingTool", func(ctx context.Context, params map[string]any) (ai.ToolOutput, error) {
		panic("nil map write on unexpected params")
	})

	result, err := handler(context.Background(), map[string]any{})
	require.Error(t, err, "panic must surface as an error")
	assert.Contains(t, err.Error(), "panicked")
	assert.Empty(t, result.Content)
	assert.Nil(t, result.Details)

	require.Len(t, executed, 1, "the failed execution must still be reported")
	assert.False(t, executed[0].Success)
}
