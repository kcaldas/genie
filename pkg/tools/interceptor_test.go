package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/require"
)

type countingTool struct {
	name string
	ran  int
	seen map[string]any
}

func (c *countingTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{Name: c.name}
}
func (c *countingTool) Handler() ai.HandlerFunc {
	return func(_ context.Context, args map[string]any) (ai.ToolOutput, error) {
		c.ran++
		c.seen = args
		return ai.ToolOutput{Details: map[string]any{"ok": true}}, nil
	}
}
func (c *countingTool) FormatOutput(map[string]interface{}) string { return "" }

func TestInterceptRefusesReplacesAndPassesThrough(t *testing.T) {
	send := &countingTool{name: "send_message"}
	read := &countingTool{name: "read_file"}
	base := NewRegistry()
	require.NoError(t, base.Register(send))

	registry := Intercept(base, InterceptorFunc(func(_ context.Context, call ToolCall) (ToolCall, error) {
		switch call.Name {
		case "send_message":
			if text, _ := call.Args["message"].(string); text == "BANANA" {
				return call, errors.New("messages must not mention BANANA")
			}
			call.Args["cc"] = "owner@example.com"
			return call, nil
		}
		return call, nil
	}))
	// Registered after the guard: guarded too.
	require.NoError(t, registry.Register(read))

	tool, ok := registry.Get("send_message")
	require.True(t, ok)
	_, err := tool.Handler()(context.Background(), map[string]any{"message": "BANANA"})
	require.EqualError(t, err, "messages must not mention BANANA")
	require.Equal(t, 0, send.ran, "a refused call must not run")

	out, err := tool.Handler()(context.Background(), map[string]any{"message": "hello"})
	require.NoError(t, err)
	require.Equal(t, true, out.Details["ok"])
	require.Equal(t, 1, send.ran)
	require.Equal(t, "owner@example.com", send.seen["cc"], "replaced args reach the tool")

	tool, ok = registry.Get("read_file")
	require.True(t, ok)
	_, err = tool.Handler()(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, 1, read.ran)

	for _, tool := range registry.GetAll() {
		_, guarded := tool.(*interceptedTool)
		require.True(t, guarded, "GetAll hands out guarded tools")
	}
	_, missing := registry.Get("nope")
	require.False(t, missing)
}

func TestInterceptWithoutInterceptorIsTheRegistry(t *testing.T) {
	base := NewRegistry()
	require.Equal(t, base, Intercept(base, nil))
	require.Nil(t, Intercept(nil, InterceptorFunc(func(_ context.Context, c ToolCall) (ToolCall, error) { return c, nil })))
}
