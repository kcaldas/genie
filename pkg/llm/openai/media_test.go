package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

func mediaResult(tool, mimeType string, body []byte) llmshared.ToolResult {
	return llmshared.ToolResult{
		Call: llmshared.ToolCall{ID: "call-1", Name: tool},
		Result: map[string]any{
			"success":     true,
			"mime_type":   mimeType,
			"data_base64": base64.StdEncoding.EncodeToString(body),
			"path":        "shot.png",
		},
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
				[]llmshared.ToolResult{mediaResult(tool, "image/png", []byte("\x89PNG body"))})

			require.NoError(t, err)
			require.Len(t, turn.messages, 2, "expected the tool response plus an image message")

			payload := toolMessagePayload(t, turn.messages[0])
			assert.NotContains(t, payload, "data_base64",
				"the payload should leave as media, not as JSON text")
		})
	}
}

func TestResponsesTurnDeliversMediaFromAnyTool(t *testing.T) {
	for _, tool := range []string{"viewDocument", "some_mcp_export"} {
		t.Run(tool, func(t *testing.T) {
			turn := &responsesTurnState{client: &Client{eventBus: events.NewEventBus()}}

			err := turn.AddToolResults(context.Background(),
				[]llmshared.ToolResult{mediaResult(tool, "application/pdf", []byte("%PDF-1.4 body"))})

			require.NoError(t, err)
			require.Len(t, turn.input, 2, "expected the tool response plus a document message")
		})
	}
}

// The other half: a type no provider renders stays in the tool result as
// text, where the size cap applies, rather than being stripped away.
func TestChatTurnKeepsUndeliverablePayloadAsText(t *testing.T) {
	turn := &turnState{client: &Client{eventBus: events.NewEventBus()}}

	err := turn.AddToolResults(context.Background(),
		[]llmshared.ToolResult{mediaResult("some_mcp_recorder", "audio/wav", []byte("RIFF....WAVE"))})

	require.NoError(t, err)
	require.Len(t, turn.messages, 1, "no media message for a type no provider renders")
	assert.Contains(t, toolMessagePayload(t, turn.messages[0]), "data_base64",
		"an undeliverable payload must remain visible in the tool result")
}

func toolMessagePayload(t *testing.T, message any) string {
	t.Helper()
	encoded, err := json.Marshal(message)
	require.NoError(t, err)
	return string(encoded)
}
