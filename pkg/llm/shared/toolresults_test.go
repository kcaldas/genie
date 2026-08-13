package shared

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeToolResultPreservesContentOrder(t *testing.T) {
	result := PreparedToolResult{Output: ai.ToolOutput{Content: []ai.ToolContent{
		ai.TextContent{Text: "first"},
		ai.BlobContent{MIMEType: "image/png", Data: []byte("png")},
		ai.TextContent{Text: "last"},
	}}}

	encoded := EncodeToolResult(result, SupportsImagesOnly)
	require.Len(t, encoded.Blobs, 1)
	assert.Equal(t, "first\nlast", encoded.Text)
}

func TestEncodeToolResultCarriesErrorFlag(t *testing.T) {
	result := PreparedToolResult{Output: ai.ErrorToolOutput(map[string]any{"error": "bad input"})}
	result.Output.Content = []ai.ToolContent{ai.TextContent{Text: `{"error":"bad input"}`}}

	encoded := EncodeToolResult(result, SupportsImagesOnly)
	assert.True(t, encoded.IsError)
	assert.JSONEq(t, `{"error":"bad input"}`, encoded.Text)
}
