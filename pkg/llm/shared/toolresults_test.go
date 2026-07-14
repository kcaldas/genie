package shared

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type toolMsg struct {
	role    string
	callID  string
	payload string
}

func TestBuildToolResultMessages(t *testing.T) {
	newTool := func(callID, payload string) toolMsg {
		return toolMsg{role: "tool", callID: callID, payload: payload}
	}
	t.Run("marshals results in order", func(t *testing.T) {
		messages, err := BuildToolResultMessages(nil, []ToolResult{
			{Call: ToolCall{ID: "1", Name: "a"}, Result: map[string]any{"ok": true}},
			{Call: ToolCall{ID: "2", Name: "b"}, Result: map[string]any{"n": 2}},
		}, newTool, nil, nil)
		require.NoError(t, err)
		require.Len(t, messages, 2)
		assert.Equal(t, "1", messages[0].callID)
		assert.JSONEq(t, `{"ok":true}`, messages[0].payload)
		assert.Equal(t, "2", messages[1].callID)
		assert.JSONEq(t, `{"n":2}`, messages[1].payload)
	})

	t.Run("handler errors become error payloads for the model", func(t *testing.T) {
		messages, err := BuildToolResultMessages(nil, []ToolResult{
			{Call: ToolCall{ID: "1", Name: "boom"}, Err: errors.New("kaput")},
		}, newTool, nil, nil)
		require.NoError(t, err)
		require.Len(t, messages, 1)
		assert.Contains(t, messages[0].payload, `function \"boom\" returned an error: kaput`)
	})
}
