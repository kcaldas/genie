package prompts

import (
	"context"
	"errors"
	"testing"

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
		wantSuccess bool
	}{
		{name: "success", handlerErr: nil, wantSuccess: true},
		{name: "failure", handlerErr: errors.New("boom"), wantSuccess: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := events.NewEventBus()
			var executed []events.ToolExecutedEvent
			events.SubscribeTo(bus, func(e events.ToolExecutedEvent) {
				executed = append(executed, e)
			})

			loader := &DefaultLoader{Publisher: bus}
			handler := loader.wrapHandlerWithEvents("myTool", func(ctx context.Context, params map[string]any) (map[string]any, error) {
				return map[string]any{"ok": true}, tt.handlerErr
			})

			_, err := handler(context.Background(), map[string]any{})
			if tt.handlerErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Len(t, executed, 1, "tool.executed must be published exactly once")
			assert.Equal(t, tt.wantSuccess, executed[0].Success)
			if !tt.wantSuccess {
				assert.ErrorContains(t, errors.New(executed[0].Message), "boom",
					"message should still describe the failure for display")
			}
		})
	}
}
