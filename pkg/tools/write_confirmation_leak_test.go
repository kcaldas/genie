package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/require"
)

// Regression: WriteTool used to subscribe a fresh handler on every
// confirmation and never unsubscribe, so each confirmed write leaked a
// handler that re-ran on all future confirmation responses.
func TestWriteToolConfirmationsDoNotLeakHandlers(t *testing.T) {
	bus := events.NewEventBus()
	inMem := bus.(*events.InMemoryBus)

	// Auto-approve every confirmation request.
	events.SubscribeTo(bus, func(req events.UserConfirmationRequest) {
		bus.Publish(events.UserConfirmationResponse{}.Topic(), events.UserConfirmationResponse{
			ExecutionID: req.ExecutionID,
			Confirmed:   true,
		})
	})

	tool := NewWriteTool(bus, true)
	handler := tool.Handler()

	dir := t.TempDir()
	ctx := context.WithValue(context.Background(), "cwd", dir)

	baseline := inMem.SubscriberCount(events.UserConfirmationResponse{}.Topic())

	for i := 0; i < 10; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file-%d.txt", i))
		result, err := handler(ctx, map[string]any{
			"path":    path,
			"content": "hello",
		})
		require.NoError(t, err)
		require.NotNil(t, result)

		written, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "hello", string(written))
	}

	require.Equal(t, baseline, inMem.SubscriberCount(events.UserConfirmationResponse{}.Topic()),
		"confirmed writes must not accumulate response handlers")
}
