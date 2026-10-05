package openai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

func mediaResult(tool, mimeType string, body []byte) llmshared.PreparedToolResult {
	name := "report.pdf"
	if strings.HasPrefix(mimeType, "image/") {
		name = "shot.png"
	}
	return llmshared.PreparedToolResult{
		Call: llmshared.ToolCall{ID: "call-1", Name: tool},
		Output: ai.ContentToolOutput(map[string]any{"success": true},
			ai.TextContent{Text: `{"success":true}`},
			ai.BlobContent{MIMEType: mimeType, Data: body, Name: name},
		),
	}
}

// The size cap exempts a native payload on the promise that it leaves as
// media. If the provider strips it and then emits nothing — which it did
// while delivery was keyed on the tool name — the content is lost
// silently and the exemption bought nothing.
func TestChatTurnDeliversMediaFromAnyTool(t *testing.T) {
	for _, tool := range []string{"viewImage", "some_mcp_screenshot"} {
		t.Run(tool, func(t *testing.T) {
			turn := &turnState{client: &Client{eventBus: events.NewEventBus()}, supportsBlob: llmshared.SupportsImagesOnly}

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
			turn := &responsesTurnState{client: &Client{eventBus: events.NewEventBus()}, supportsBlob: llmshared.SupportsImagesOnly}

			err := turn.AddToolResults(context.Background(),
				[]llmshared.PreparedToolResult{mediaResult(tool, "image/png", []byte("\x89PNG body"))})

			require.NoError(t, err)
			require.Len(t, turn.input, 2, "expected the tool response plus an image message")
		})
	}
}

// The Responses API reads a PDF as an input_file part, so a document a tool
// returns reaches the model instead of a note that it exists.
func TestResponsesTurnDeliversPDFAsInputFile(t *testing.T) {
	turn := &responsesTurnState{client: &Client{eventBus: events.NewEventBus()}, supportsBlob: supportsResponsesBlob}

	err := turn.AddToolResults(context.Background(),
		[]llmshared.PreparedToolResult{mediaResult("viewDocument", "application/pdf", []byte("%PDF-1.4 body"))})

	require.NoError(t, err)
	require.Len(t, turn.input, 2, "expected the tool response plus a document message")
	output := toolMessagePayload(t, turn.input[0])
	assert.NotContains(t, output, "cannot be displayed")
	document := toolMessagePayload(t, turn.input[1])
	assert.Contains(t, document, `"type":"input_file"`)
	assert.Contains(t, document, `"file_data":"data:application/pdf;base64,`)
	assert.Contains(t, document, `"filename":"report.pdf"`)
}

func TestResponsesBlobSupport(t *testing.T) {
	for mimeType, want := range map[string]bool{
		"image/png":                   true,
		"application/pdf":             true,
		"Application/PDF; name=a.pdf": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"text/plain; charset=utf-8": true,
		"text/markdown":             true,
		"text/csv":                  true,
		"text/html":                 true,
		"audio/mpeg":                false,
		"application/vnd.ms-excel":  false,
		"application/zip":           false,
	} {
		assert.Equal(t, want, supportsResponsesBlob(ai.BlobContent{MIMEType: mimeType}), mimeType)
	}
}

// A document's media type survives into the data URL without its
// parameters, so the API sees the type it reads.
func TestResponsesTurnDeliversTextDocumentAsInputFile(t *testing.T) {
	turn := &responsesTurnState{client: &Client{eventBus: events.NewEventBus()}, supportsBlob: supportsResponsesBlob}

	result := mediaResult("some_mcp_export", "text/csv; charset=utf-8", []byte("field,value\n"))
	err := turn.AddToolResults(context.Background(), []llmshared.PreparedToolResult{result})

	require.NoError(t, err)
	require.Len(t, turn.input, 2)
	assert.Contains(t, toolMessagePayload(t, turn.input[1]), `"file_data":"data:text/csv;base64,`)
}

// The other half: what this provider cannot render is reported in the
// body, so the model learns the content exists rather than receiving
// nothing.
func TestChatTurnReportsUndeliverableAttachment(t *testing.T) {
	turn := &turnState{client: &Client{eventBus: events.NewEventBus()}, supportsBlob: llmshared.SupportsImagesOnly}

	err := turn.AddToolResults(context.Background(),
		[]llmshared.PreparedToolResult{mediaResult("viewDocument", "application/pdf", []byte("%PDF-1.4"))})

	require.NoError(t, err)
	require.Len(t, turn.messages, 1, "no media message for a type this provider cannot render")
	assert.Contains(t, toolMessagePayload(t, turn.messages[0]), "cannot be displayed",
		"an undeliverable attachment must be reported, not dropped")
}

func toolMessagePayload(t *testing.T, message any) string {
	t.Helper()
	encoded, err := json.Marshal(message)
	require.NoError(t, err)
	return string(encoded)
}
