package anthropic

import (
	"context"
	"testing"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

func blobResult(mimeType, name string, body []byte) llmshared.PreparedToolResult {
	return llmshared.PreparedToolResult{
		Call: llmshared.ToolCall{ID: "call-1", Name: "some_mcp_export"},
		Output: ai.ContentToolOutput(map[string]any{"success": true},
			ai.TextContent{Text: `{"success":true}`},
			ai.BlobContent{MIMEType: mimeType, Data: body, Name: name},
		),
	}
}

// Anthropic reads text only as a plain-text document, so a text file a tool
// returns (CSV, markdown, HTML) goes as one, titled with its file name.
func TestTurnDeliversTextFileAsPlainTextDocument(t *testing.T) {
	turn := &turnState{supportsBlob: supportsAnthropicBlob}

	err := turn.AddToolResults(context.Background(),
		[]llmshared.PreparedToolResult{blobResult("text/csv; charset=utf-8", "rates.csv", []byte("field,value\ncode,ZEBRA-4217\n"))})

	require.NoError(t, err)
	require.Len(t, turn.messages, 2, "expected the tool result plus a document message")
	var doc *anthropic_sdk.DocumentBlockParam
	for _, block := range turn.messages[1].Content {
		if block.OfDocument != nil {
			doc = block.OfDocument
		}
	}
	require.NotNil(t, doc)
	require.NotNil(t, doc.Source.OfText)
	assert.Equal(t, "field,value\ncode,ZEBRA-4217\n", doc.Source.OfText.Data)
	assert.Equal(t, "rates.csv", doc.Title.Value)
}

func TestAnthropicBlobSupport(t *testing.T) {
	for _, c := range []struct {
		mimeType string
		body     []byte
		want     bool
	}{
		{"image/png", []byte("\x89PNG"), true},
		{"application/pdf", []byte("%PDF"), true},
		{"text/plain; charset=utf-8", []byte("hello"), true},
		{"text/markdown", []byte("# memo"), true},
		{"text/html", []byte("<p>hi</p>"), true},
		{"text/plain", []byte{0xff, 0xfe, 0x00}, false},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", []byte("PK"), false},
		{"audio/mpeg", []byte("ID3"), false},
	} {
		assert.Equal(t, c.want, supportsAnthropicBlob(ai.BlobContent{MIMEType: c.mimeType, Data: c.body}), c.mimeType)
	}
}
