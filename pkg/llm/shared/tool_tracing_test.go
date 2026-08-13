package shared

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/toolctx"
)

// Every tool call gets a real execution ID from the loop — no more
// "unknown" in tool lifecycle events for tools that don't mint their own.
func TestExecuteToolCallsAssignsExecutionID(t *testing.T) {
	seen := map[string]string{}
	handler := func(ctx context.Context, _ map[string]any) (ai.ToolOutput, error) {
		id, ok := toolctx.ExecutionID(ctx)
		if !ok || id == "" {
			t.Fatal("handler ctx has no execution ID")
		}
		seen[id] = id
		return ai.ToolOutput{}, nil
	}

	executeToolCalls(context.Background(), nil,
		[]ToolCall{{Name: "a"}, {Name: "a"}},
		map[string]ai.HandlerFunc{"a": handler}, ToolResultLimits{})

	if len(seen) != 2 {
		t.Fatalf("expected 2 distinct execution IDs, got %d", len(seen))
	}
}

// A caller-provided execution ID is respected, not replaced.
func TestExecuteToolCallsKeepsCallerExecutionID(t *testing.T) {
	got := ""
	handler := func(ctx context.Context, _ map[string]any) (ai.ToolOutput, error) {
		got, _ = toolctx.ExecutionID(ctx)
		return ai.ToolOutput{}, nil
	}

	ctx := toolctx.WithExecutionID(context.Background(), "preset-id")
	executeToolCalls(ctx, nil,
		[]ToolCall{{Name: "a"}},
		map[string]ai.HandlerFunc{"a": handler}, ToolResultLimits{})

	if got != "preset-id" {
		t.Fatalf("execution ID = %q, want preset-id", got)
	}
}
