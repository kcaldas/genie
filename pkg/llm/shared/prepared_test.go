package shared

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepare(t *testing.T, name string, output ai.ToolOutput, err error, limits ToolResultLimits) PreparedToolResult {
	t.Helper()
	limits = limits.withDefaults()
	return prepareToolResult(nil, ToolCall{Name: name}, output, err, limits, newBatchBudget(limits))
}

func outputText(t *testing.T, result PreparedToolResult) string {
	t.Helper()
	encoded := EncodeToolResult(result, func(ai.BlobContent) bool { return true })
	return encoded.Text
}

func TestPrepareSerializesJSONAndPreservesDetails(t *testing.T) {
	details := map[string]any{"success": true, "answer": 42}
	prepared := prepare(t, "lookup", ai.JSONToolOutput(details), nil, ToolResultLimits{})

	assert.Equal(t, details, prepared.Output.Details)
	assert.JSONEq(t, `{"success":true,"answer":42}`, outputText(t, prepared))
}

func TestPreparePreservesTypedBlobWithoutPuttingBytesInText(t *testing.T) {
	raw := []byte{0x89, 0x50, 0x4e, 0x47}
	prepared := prepare(t, "screenshot", ai.ContentToolOutput(
		map[string]any{"path": "diagram.png"},
		ai.BlobContent{MIMEType: "image/png", Data: raw, Name: "diagram.png"},
	), nil, ToolResultLimits{})

	encoded := EncodeToolResult(prepared, SupportsImagesOnly)
	require.Len(t, encoded.Blobs, 1)
	assert.Equal(t, raw, encoded.Blobs[0].Data)
	assert.Equal(t, "(no tool output)", encoded.Text)
}

func TestPrepareDoesNotChargeBlobsToTextBudget(t *testing.T) {
	raw := make([]byte, 2*1024*1024)
	prepared := prepare(t, "viewImage", ai.ContentToolOutput(nil,
		ai.BlobContent{MIMEType: "image/png", Data: raw},
	), nil, ToolResultLimits{MaxTextBytes: 4096, MaxBatchTextBytes: 4096})

	encoded := EncodeToolResult(prepared, SupportsImagesOnly)
	require.Len(t, encoded.Blobs, 1)
	assert.Equal(t, raw, encoded.Blobs[0].Data)
}

func TestPrepareOmitsBlobAboveOperationalLimit(t *testing.T) {
	prepared := prepare(t, "screenshot", ai.ContentToolOutput(nil,
		ai.BlobContent{MIMEType: "image/png", Data: make([]byte, 5), Name: "large.png"},
	), nil, ToolResultLimits{MaxBlobBytes: 4})

	encoded := EncodeToolResult(prepared, SupportsImagesOnly)
	assert.Empty(t, encoded.Blobs)
	assert.Contains(t, encoded.Text, "attachment safety limit")
}

func TestPrepareBoundsTextAndKeepsUTF8Valid(t *testing.T) {
	prepared := prepare(t, "search", ai.ContentToolOutput(nil,
		ai.TextContent{Text: strings.Repeat("cafe \u2615 result\n", 10000)},
	), nil, ToolResultLimits{MaxTextBytes: 8192})

	text := outputText(t, prepared)
	assert.LessOrEqual(t, len(text), 8192)
	assert.True(t, utf8.ValidString(text))
	assert.Contains(t, text, "tool output truncated")
}

func TestPrepareFoldsHandlerErrorsIntoTypedFailure(t *testing.T) {
	prepared := prepare(t, "bash", ai.ToolOutput{}, errors.New("process failed"), ToolResultLimits{})

	assert.True(t, prepared.Output.IsError)
	assert.Contains(t, outputText(t, prepared), "process failed")
}

func TestBatchTextBudgetDoesNotBoundBlobs(t *testing.T) {
	handler := func(context.Context, map[string]any) (ai.ToolOutput, error) {
		return ai.ContentToolOutput(nil,
			ai.TextContent{Text: strings.Repeat("x", 10000)},
			ai.BlobContent{MIMEType: "image/png", Data: make([]byte, 1024)},
		), nil
	}
	results := executeToolCalls(context.Background(), nil,
		[]ToolCall{{Name: "a"}, {Name: "a"}},
		map[string]ai.HandlerFunc{"a": handler},
		ToolResultLimits{
			MaxTextBytes:      -1,
			MaxBatchTextBytes: 4096,
		}.withDefaults(),
	)

	totalText := 0
	totalBlobs := 0
	for _, result := range results {
		for _, block := range result.Output.Content {
			switch value := block.(type) {
			case ai.TextContent:
				totalText += len(value.Text)
			case ai.BlobContent:
				totalBlobs += len(value.Data)
			}
		}
	}
	assert.LessOrEqual(t, totalText, 4096)
	assert.Equal(t, 2048, totalBlobs, "native blobs must bypass the synthetic text budget")
}

func TestBatchExhaustionKeepsNoticeForEveryResult(t *testing.T) {
	handler := func(context.Context, map[string]any) (ai.ToolOutput, error) {
		return ai.ContentToolOutput(nil, ai.TextContent{Text: strings.Repeat("x", 10_000)}), nil
	}
	results := executeToolCalls(context.Background(), nil,
		[]ToolCall{{Name: "a"}, {Name: "a"}}, map[string]ai.HandlerFunc{"a": handler},
		ToolResultLimits{MaxTextBytes: -1, MaxBatchTextBytes: len(toolOutputOmittedNotice)}.withDefaults())

	require.Len(t, results, 2)
	for _, result := range results {
		assert.NotEqual(t, "(no tool output)", outputText(t, result))
		assert.Contains(t, outputText(t, result), "tool output")
	}
}

func TestEncodeToolResultReportsUnsupportedBlob(t *testing.T) {
	prepared := PreparedToolResult{Output: ai.ContentToolOutput(nil,
		ai.TextContent{Text: "metadata"},
		ai.BlobContent{MIMEType: "application/pdf", Data: []byte("%PDF"), Name: "report.pdf"},
	)}

	encoded := EncodeToolResult(prepared, SupportsImagesOnly)
	assert.Empty(t, encoded.Blobs)
	assert.Contains(t, encoded.Text, "metadata")
	assert.Contains(t, encoded.Text, "report.pdf")
	assert.Contains(t, encoded.Text, "cannot be displayed")
}
