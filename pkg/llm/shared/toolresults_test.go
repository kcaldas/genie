package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type toolMsg struct {
	role    string
	callID  string
	payload string
}

func newToolMsg(callID, payload string) toolMsg {
	return toolMsg{role: "tool", callID: callID, payload: payload}
}

func newAttachmentMsg(a Attachment) toolMsg {
	return toolMsg{role: a.Kind.String(), payload: a.MIMEType}
}

func TestBuildToolResultMessages(t *testing.T) {
	t.Run("marshals bodies in order", func(t *testing.T) {
		messages, err := BuildToolResultMessages([]PreparedToolResult{
			{Call: ToolCall{ID: "1", Name: "a"}, Body: map[string]any{"ok": true}},
			{Call: ToolCall{ID: "2", Name: "b"}, Body: map[string]any{"n": 2}},
		}, SupportsImagesOnly, newToolMsg, newAttachmentMsg)
		require.NoError(t, err)
		require.Len(t, messages, 2)
		assert.Equal(t, "1", messages[0].callID)
		assert.JSONEq(t, `{"ok":true}`, messages[0].payload)
		assert.Equal(t, "2", messages[1].callID)
		assert.JSONEq(t, `{"n":2}`, messages[1].payload)
	})

	// Delivery follows the attachment, not the tool name. A payload
	// lifted out of the body and then not emitted is content lost
	// without a trace.
	t.Run("delivers attachments from any tool", func(t *testing.T) {
		for _, tool := range []string{"viewImage", "some_mcp_screenshot"} {
			messages, err := BuildToolResultMessages([]PreparedToolResult{{
				Call:        ToolCall{ID: "1", Name: tool},
				Body:        map[string]any{"success": true},
				Attachments: []Attachment{{Kind: AttachmentImage, MIMEType: "image/png"}},
			}}, SupportsImagesOnly, newToolMsg, newAttachmentMsg)

			require.NoError(t, err)
			require.Len(t, messages, 2, "tool %q lost its attachment", tool)
			assert.Equal(t, "image", messages[1].role)
			assert.Equal(t, "image/png", messages[1].payload)
		}
	})

	// What a provider cannot render is reported, not dropped: the model
	// is told the content exists and why it did not arrive.
	t.Run("reports unsupported attachments in the body", func(t *testing.T) {
		messages, err := BuildToolResultMessages([]PreparedToolResult{{
			Call: ToolCall{ID: "1", Name: "viewDocument"},
			Body: map[string]any{"success": true},
			Attachments: []Attachment{{
				Kind:     AttachmentDocument,
				MIMEType: "application/pdf",
				Data:     []byte("%PDF"),
				Path:     "report.pdf",
			}},
		}}, SupportsImagesOnly, newToolMsg, newAttachmentMsg)

		require.NoError(t, err)
		require.Len(t, messages, 1, "no media message for a type this provider cannot render")
		assert.Contains(t, messages[0].payload, "attachment_error")
		assert.Contains(t, messages[0].payload, "report.pdf")
	})
}
