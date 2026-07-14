package tools

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/toolctx"
	"github.com/stretchr/testify/require"
)

// Regression: a command that leaves a background grandchild holding the
// output pipe (e.g. "some-daemon &") used to block CombinedOutput until
// the grandchild exited — long past the command's own completion or
// timeout — hanging the whole agent turn.
func TestBashSyncCommandReturnsWhenGrandchildHoldsPipe(t *testing.T) {
	tool := NewBashTool(events.NewEventBus(), false)
	handler := tool.Handler()

	ctx := toolctx.WithWorkingDir(context.Background(), t.TempDir())

	done := make(chan map[string]any, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := handler(ctx, map[string]any{
			"command":    "sleep 30 & echo started",
			"timeout_ms": float64(10_000),
		})
		if err != nil {
			errCh <- err
			return
		}
		done <- result
	}()

	select {
	case err := <-errCh:
		t.Fatalf("command failed: %v", err)
	case result := <-done:
		require.Contains(t, result["results"], "started")
	case <-time.After(8 * time.Second):
		t.Fatal("sync bash call hung on a grandchild process holding the output pipe")
	}
}

// The timeout must terminate the whole process group, not just the
// shell, so the call returns promptly even mid-pipeline.
func TestBashSyncCommandTimesOutPromptly(t *testing.T) {
	tool := NewBashTool(events.NewEventBus(), false)
	handler := tool.Handler()

	ctx := toolctx.WithWorkingDir(context.Background(), t.TempDir())

	start := time.Now()
	result, err := handler(ctx, map[string]any{
		"command":    "sleep 30",
		"timeout_ms": float64(300),
	})
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Equal(t, false, result["success"])
	require.Contains(t, result["error"], "timed out")
	require.Less(t, elapsed, 6*time.Second)
}
