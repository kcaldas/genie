package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

func mediaResult(tool, mimeType string, body []byte) llmshared.PreparedToolResult {
	kind := llmshared.AttachmentDocument
	if strings.HasPrefix(mimeType, "image/") {
		kind = llmshared.AttachmentImage
	}
	return llmshared.PreparedToolResult{
		Call: llmshared.ToolCall{ID: "call-1", Name: tool},
		Body: map[string]any{"success": true},
		Attachments: []llmshared.Attachment{{
			Kind:     kind,
			MIMEType: mimeType,
			Data:     body,
			Base64:   base64.StdEncoding.EncodeToString(body),
			Path:     "shot.png",
		}},
	}
}

// The size cap exempts a native payload on the promise that it leaves as
// media. If the provider strips it and then emits nothing — which it did
// while delivery was keyed on the tool name — the content is lost
// silently and the exemption bought nothing.
func TestChatTurnDeliversMediaFromAnyTool(t *testing.T) {
	for _, tool := range []string{"viewImage", "some_mcp_screenshot"} {
		t.Run(tool, func(t *testing.T) {
			turn := &turnState{client: &Client{eventBus: events.NewEventBus()}}

			err := turn.AddToolResults(context.Background(),
				[]llmshared.PreparedToolResult{mediaResult(tool, "image/png", []byte("\x89PNG body"))})

			require.NoError(t, err)
			require.Len(t, turn.messages, 2, "expected the tool response plus an image message")

			payload := toolMessagePayload(t, turn.messages[0])
			assert.NotContains(t, payload, "data_base64",
				"the payload should leave as media, not as JSON text")
		})
	}
}

func TestResponsesTurnDeliversMediaFromAnyTool(t *testing.T) {
	for _, tool := range []string{"viewImage", "some_mcp_screenshot"} {
		t.Run(tool, func(t *testing.T) {
			turn := &responsesTurnState{client: &Client{eventBus: events.NewEventBus()}}

			err := turn.AddToolResults(context.Background(),
				[]llmshared.PreparedToolResult{mediaResult(tool, "image/png", []byte("\x89PNG body"))})

			require.NoError(t, err)
			require.Len(t, turn.input, 2, "expected the tool response plus an image message")
		})
	}
}

// The other half: what this provider cannot render is reported in the
// body, so the model learns the content exists rather than receiving
// nothing.
func TestChatTurnReportsUndeliverableAttachment(t *testing.T) {
	turn := &turnState{client: &Client{eventBus: events.NewEventBus()}}

	err := turn.AddToolResults(context.Background(),
		[]llmshared.PreparedToolResult{mediaResult("viewDocument", "application/pdf", []byte("%PDF-1.4"))})

	require.NoError(t, err)
	require.Len(t, turn.messages, 1, "no media message for a type this provider cannot render")
	assert.Contains(t, toolMessagePayload(t, turn.messages[0]), "attachment_error",
		"an undeliverable attachment must be reported, not dropped")
}

func toolMessagePayload(t *testing.T, message any) string {
	t.Helper()
	encoded, err := json.Marshal(message)
	require.NoError(t, err)
	return string(encoded)
}
