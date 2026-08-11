package shared

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/llm/shared/toolpayload"
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

// Delivery must follow the payload, not the tool name. A payload
// stripped from the tool result and then not emitted is content lost
// with no trace — and it was also exempted from the size cap on the
// promise that it would be delivered.
func TestBuildToolResultMessagesDeliversMediaFromAnyTool(t *testing.T) {
	newTool := func(callID, payload string) toolMsg {
		return toolMsg{role: "tool", callID: callID, payload: payload}
	}
	newImage := func(p *toolpayload.Payload) toolMsg {
		return toolMsg{role: "image", payload: p.MIMEType}
	}
	newDocument := func(p *toolpayload.Payload) toolMsg {
		return toolMsg{role: "document", payload: p.MIMEType}
	}
	png := base64.StdEncoding.EncodeToString([]byte("\x89PNG fake body"))
	pdf := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 body"))

	for name, tc := range map[string]struct {
		tool     string
		mimeType string
		data     string
		wantRole string
	}{
		"built-in image":    {"viewImage", "image/png", png, "image"},
		"built-in document": {"viewDocument", "application/pdf", pdf, "document"},
		"mcp screenshot":    {"some_mcp_screenshot", "image/png", png, "image"},
		"mcp pdf export":    {"some_mcp_export", "application/pdf", pdf, "document"},
	} {
		t.Run(name, func(t *testing.T) {
			messages, err := BuildToolResultMessages(nil, []ToolResult{{
				Call: ToolCall{ID: "1", Name: tc.tool},
				Result: map[string]any{
					"success":     true,
					"mime_type":   tc.mimeType,
					"data_base64": tc.data,
				},
			}}, newTool, newImage, newDocument)
			require.NoError(t, err)
			require.Len(t, messages, 2, "expected the tool response plus a media message")

			assert.NotContains(t, messages[0].payload, "data_base64",
				"the payload should leave as media, not as JSON text")
			assert.Equal(t, tc.wantRole, messages[1].role)
			assert.Equal(t, tc.mimeType, messages[1].payload)
		})
	}
}

// The other half of the invariant: what cannot be delivered is not
// stripped, so it stays visible to the model as text.
func TestBuildToolResultMessagesKeepsUndeliverablePayloadAsText(t *testing.T) {
	newTool := func(callID, payload string) toolMsg {
		return toolMsg{role: "tool", callID: callID, payload: payload}
	}
	newImage := func(*toolpayload.Payload) toolMsg { return toolMsg{role: "image"} }
	newDocument := func(*toolpayload.Payload) toolMsg { return toolMsg{role: "document"} }

	messages, err := BuildToolResultMessages(nil, []ToolResult{{
		Call: ToolCall{ID: "1", Name: "some_mcp_recorder"},
		Result: map[string]any{
			"success":     true,
			"mime_type":   "audio/wav",
			"data_base64": base64.StdEncoding.EncodeToString([]byte("RIFF....WAVE")),
		},
	}}, newTool, newImage, newDocument)

	require.NoError(t, err)
	require.Len(t, messages, 1, "no media message for a type no provider renders")
	assert.Contains(t, messages[0].payload, "data_base64",
		"an undeliverable payload must remain in the tool result rather than vanish")
}
