package tools_test

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const samplePDFBase64 = "JVBERi0xLjQKJcTl8uXrp/Og0MTGCjEgMCBvYmoKPDwvVHlwZS9DYXRhbG9nL1BhZ2VzIDIgMCBSPj4KZW5kb2JqCjIgMCBvYmoKPDwvVHlwZS9QYWdlcy9LaWRzWzMgMCBSXT4+CmVuZG9iagozIDAgb2JqCjw8L01lZGlhQm94WzAgMCA1MCA1MF0vUGFyZW50IDIgMCBSL1Jlc291cmNlczw8L0ZvbnQ8PC9GMTw8L1R5cGUvRm9udC9CYXNlRm9udC9IZWx2ZXRpY2E+Pj4+Pi9Qcm9jU2V0Wy9QREZdL1R5cGUvUGFnZS9Db250ZW50cyA0IDAgUj4+CmVuZG9iago0IDAgb2JqCjw8L0xlbmd0aCA1NCA+PgpzdHJlYW0KQlQKL0YxIDEyIFRmCjEwIDQwIFRnCihIZWxsbyBQRkQhKSBUagpFVAplbmRzdHJlYW0KZW5kb2JqCnhyZWYKMCA1CjAwMDAwMDAwMDAgNjU1MzUgZiAKMDAwMDAwMDA5OCAwMDAwMCBuIAowMDAwMDAwMTY0IDAwMDAwIG4gCjAwMDAwMDAzMDggMDAwMDAgbiAKMDAwMDAwMDM4MSAwMDAwMCBuIAp0cmFpbGVyCjw8L1Jvb3QgMSAwIFIvU2l6ZSA1L0luZm8gNiAwIFIvSURbPGU0YjY2ZjQ0ZDk1ZWQ0NjM4NzA1NjJiMzcwZDdiNDIzPl0+PgpzdGFydHhyZWYKNDk0CiUlRU9G" // minimal valid PDF

type capturingDocPublisher struct {
	messages []events.ToolCallMessageEvent
}

func (c *capturingDocPublisher) Publish(topic string, event interface{}) {
	if topic != "tool.call.message" {
		return
	}
	if msg, ok := event.(events.ToolCallMessageEvent); ok {
		c.messages = append(c.messages, msg)
	}
}

func TestViewDocumentTool_Success(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "doc.pdf")
	data, err := base64.StdEncoding.DecodeString(samplePDFBase64)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filePath, data, 0o600))

	ctx := context.WithValue(context.Background(), "cwd", tmpDir)

	publisher := &capturingDocPublisher{}
	tool := tools.NewViewDocumentTool(publisher)

	handler := tool.Handler()
	result, err := handler(ctx, map[string]any{
		"file_path":        "doc.pdf",
		"_display_message": "Reviewing specification",
	})
	require.NoError(t, err)

	success, _ := result["success"].(bool)
	assert.True(t, success)
	assert.Equal(t, "application/pdf", result["mime_type"])
	assert.Equal(t, int64(len(data)), result["size_bytes"])
	assert.Equal(t, "doc.pdf", result["path"])

	require.Len(t, publisher.messages, 1)
	assert.Equal(t, "viewDocument", publisher.messages[0].ToolName)
	assert.Equal(t, "Reviewing specification", publisher.messages[0].Message)

	formatted := tool.FormatOutput(result)
	assert.Contains(t, formatted, "Attached document `doc.pdf`")
}

func TestViewDocumentTool_PathOutsideWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.WithValue(context.Background(), "cwd", tmpDir)

	tool := tools.NewViewDocumentTool(&events.NoOpPublisher{})
	handler := tool.Handler()

	result, err := handler(ctx, map[string]any{
		"file_path":        "../secret.pdf",
		"_display_message": "Testing",
	})
	require.NoError(t, err)

	success, _ := result["success"].(bool)
	assert.False(t, success)
	assert.Contains(t, result["error"], "outside")
}

func TestViewDocumentTool_SizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.WithValue(context.Background(), "cwd", tmpDir)

	data, err := base64.StdEncoding.DecodeString(samplePDFBase64)
	require.NoError(t, err)
	filePath := filepath.Join(tmpDir, "doc.pdf")
	require.NoError(t, os.WriteFile(filePath, data, 0o600))

	tool := tools.NewViewDocumentTool(&events.NoOpPublisher{}, tools.WithMaxDocumentBytes(int64(len(data)-1)))
	handler := tool.Handler()

	result, err := handler(ctx, map[string]any{
		"file_path":        "doc.pdf",
		"_display_message": "Should fail",
	})
	require.NoError(t, err)
	assert.False(t, result["success"].(bool))
	assert.Contains(t, result["error"], "exceeds maximum")
}

func TestViewDocumentTool_MissingDisplayMessage(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.WithValue(context.Background(), "cwd", tmpDir)

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "doc.pdf"), []byte("dummy"), 0o600))

	tool := tools.NewViewDocumentTool(&capturingDocPublisher{})
	handler := tool.Handler()

	_, err := handler(ctx, map[string]any{
		"file_path": "doc.pdf",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "_display_message")
}

func TestViewDocumentTool_UnsupportedType(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.WithValue(context.Background(), "cwd", tmpDir)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "doc.txt"), []byte("hello"), 0o600))

	tool := tools.NewViewDocumentTool(&events.NoOpPublisher{})
	handler := tool.Handler()

	result, err := handler(ctx, map[string]any{
		"file_path":        "doc.txt",
		"_display_message": "Should fail",
	})
	require.NoError(t, err)
	assert.False(t, result["success"].(bool))
	assert.Contains(t, result["error"], "unsupported")
}
