package shared

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
)

func imageResult(size int) (map[string]any, []byte) {
	raw := bytes.Repeat([]byte{0x89, 0x50, 0x4e, 0x47}, size/4)
	return map[string]any{
		"success":     true,
		"path":        "diagram.png",
		"mime_type":   "image/png",
		"size_bytes":  len(raw),
		"data_base64": base64.StdEncoding.EncodeToString(raw),
	}, raw
}

func prepare(t *testing.T, name string, result map[string]any, err error, limits ToolResultLimits) PreparedToolResult {
	t.Helper()
	limits = limits.withDefaults()
	return prepareToolResult(nil, ToolCall{Name: name}, result, err, limits, newBatchBudget(limits))
}

// Images run to megabytes, far past any body limit. They are delivered
// natively, so the body cap must never see them — a trimmed base64
// field fails to decode and takes the turn down with it.
func TestPrepareLiftsAttachmentsOutOfTheBody(t *testing.T) {
	result, raw := imageResult(1_200_000)

	prepared := prepare(t, "viewImage", result, nil, ToolResultLimits{})

	require.Len(t, prepared.Attachments, 1)
	assert.Equal(t, raw, prepared.Attachments[0].Data, "payload should survive intact")
	assert.Equal(t, AttachmentImage, prepared.Attachments[0].Kind)
	assert.NotContains(t, prepared.Body, "data_base64", "attachment bytes must not stay in the body")
	assert.LessOrEqual(t, serializedLen(prepared.Body), DefaultMaxToolResultBytes)
}

// The attachment path is a property of the payload, so a tool outside
// the built-in set gets the same treatment.
func TestPrepareLiftsAttachmentsFromAnyTool(t *testing.T) {
	result, _ := imageResult(400_000)

	prepared := prepare(t, "some_mcp_screenshot", result, nil, ToolResultLimits{})

	require.Len(t, prepared.Attachments, 1)
	assert.Equal(t, "image/png", prepared.Attachments[0].MIMEType)
}

// Attachments skip the body cap, so they need a bound of their own —
// and exceeding it must be reported, not silently swallowed.
func TestPrepareReportsOversizedAttachment(t *testing.T) {
	result, _ := imageResult(200_000)

	prepared := prepare(t, "viewImage", result, nil, ToolResultLimits{MaxAttachmentBytes: 1024})

	assert.Empty(t, prepared.Attachments)
	assert.NotContains(t, prepared.Body, "data_base64")
	assert.Contains(t, prepared.Body["attachment_error"], "exceeds")
}

// An unreadable payload must not be left in the body either: it would
// be truncated into noise by the body cap.
func TestPrepareReportsUnusableAttachment(t *testing.T) {
	prepared := prepare(t, "some_mcp_tool", map[string]any{
		"success":     true,
		"data_base64": strings.Repeat("A", 500_000), // no mime_type
	}, nil, ToolResultLimits{})

	assert.Empty(t, prepared.Attachments)
	assert.NotContains(t, prepared.Body, "data_base64")
	assert.Contains(t, prepared.Body["attachment_error"], "could not be read")
}

// A 4.7MB grep result took a production agent's turn past the model's
// 1,048,576-token window: tool bodies bypass the context budget, so
// nothing else bounded them.
func TestPrepareBoundsPathologicalBody(t *testing.T) {
	result := map[string]any{
		"success": true,
		"results": strings.Repeat("path/to/file.go:42:Hazardous materials\n", 120_000),
	}

	prepared := prepare(t, "searchInFiles", result, nil, ToolResultLimits{})

	assert.LessOrEqual(t, serializedLen(prepared.Body), DefaultMaxToolResultBytes)
	assert.Equal(t, true, prepared.Body["success"], "structural fields survive")
	text := prepared.Body["results"].(string)
	assert.True(t, strings.HasPrefix(text, "path/to/file.go:42:"), "keeps the head of the real output")
	assert.Contains(t, text, "INCOMPLETE")
}

// Errors are normalized here, once, so providers never reinterpret a
// raw handler error — and the same cap bounds them.
func TestPrepareFoldsErrorsIntoTheBody(t *testing.T) {
	prepared := prepare(t, "bash", nil, errors.New(strings.Repeat("stderr ", 100_000)),
		ToolResultLimits{MaxBodyBytes: 8192})

	assert.Empty(t, prepared.Attachments)
	assert.LessOrEqual(t, serializedLen(prepared.Body), 8192)
	assert.Contains(t, prepared.Body["error"], "bash")
	assert.Contains(t, prepared.Body["error"], "INCOMPLETE")
}

// Per-result limits alone still let a step of many calls overflow a
// small window, so the step shares an allowance.
func TestBatchBudgetBoundsTheWholeStep(t *testing.T) {
	const batch = 64 * 1024
	handler := func(ctx context.Context, _ map[string]any) (map[string]any, error) {
		return map[string]any{"results": strings.Repeat("y", 200_000)}, nil
	}
	handlers := map[string]ai.HandlerFunc{"a": handler, "b": handler, "c": handler}
	calls := []ToolCall{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	results := executeToolCalls(context.Background(), nil, calls, handlers,
		ToolResultLimits{MaxBatchBytes: batch}.withDefaults())

	total := 0
	for _, result := range results {
		total += serializedLen(result.Body)
	}
	assert.LessOrEqual(t, total, batch, "the step as a whole must stay within the batch allowance")
	assert.Greater(t, serializedLen(results[0].Body), 0, "the first result keeps fidelity")
}

// Attachments are not bodies: a step full of media must not be starved
// by the body allowance.
func TestBatchBudgetIgnoresAttachments(t *testing.T) {
	result, _ := imageResult(400_000)
	handlers := map[string]ai.HandlerFunc{
		"viewImage": func(ctx context.Context, _ map[string]any) (map[string]any, error) {
			return result, nil
		},
	}

	results := executeToolCalls(context.Background(), nil,
		[]ToolCall{{Name: "viewImage"}, {Name: "viewImage"}}, handlers,
		ToolResultLimits{MaxBatchBytes: 8192}.withDefaults())

	for i, prepared := range results {
		require.Len(t, prepared.Attachments, 1, "result %d lost its attachment to the body budget", i)
	}
}

func TestSplitAttachmentsReportsWhatTheProviderCannotRender(t *testing.T) {
	prepared := PreparedToolResult{
		Call: ToolCall{Name: "viewDocument"},
		Body: map[string]any{"success": true},
		Attachments: []Attachment{
			{Kind: AttachmentImage, MIMEType: "image/png"},
			{Kind: AttachmentDocument, MIMEType: "application/pdf", Path: "report.pdf"},
		},
	}

	body, delivered := SplitAttachments(prepared, SupportsImagesOnly)

	require.Len(t, delivered, 1)
	assert.Equal(t, AttachmentImage, delivered[0].Kind)
	assert.Contains(t, body["attachment_error"], "report.pdf")
	assert.NotContains(t, prepared.Body, "attachment_error", "the original body must not be mutated")
}
