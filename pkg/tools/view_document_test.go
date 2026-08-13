package tools_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/toolctx"
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

func (c *capturingDocPublisher) PublishSync(topic string, event interface{}) {
	c.Publish(topic, event)
}

func TestViewDocumentTool_Success(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "doc.pdf")
	data, err := base64.StdEncoding.DecodeString(samplePDFBase64)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filePath, data, 0o600))

	ctx := toolctx.WithWorkingDir(context.Background(), tmpDir)

	publisher := &capturingDocPublisher{}
	tool := tools.NewViewDocumentTool(publisher)

	handler := tool.Handler()
	result, err := handler(ctx, map[string]any{
		"file_path":        "doc.pdf",
		"_display_message": "Reviewing specification",
	})
	require.NoError(t, err)

	success, _ := result.Details["success"].(bool)
	assert.True(t, success)
	assert.Equal(t, "application/pdf", result.Details["mime_type"])
	assert.Equal(t, int64(len(data)), result.Details["size_bytes"])
	assert.Equal(t, "doc.pdf", result.Details["path"])
	require.Len(t, result.Content, 1)
	blob, ok := result.Content[0].(ai.BlobContent)
	require.True(t, ok)
	assert.Equal(t, data, blob.Data)
	assert.Equal(t, "application/pdf", blob.MIMEType)

	require.Len(t, publisher.messages, 1)
	assert.Equal(t, "viewDocument", publisher.messages[0].ToolName)
	assert.Equal(t, "Reviewing specification", publisher.messages[0].Message)

	formatted := tool.FormatOutput(result.Details)
	assert.Contains(t, formatted, "Attached document `doc.pdf`")
}

const sampleDocxDocumentXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Coversheet</w:t></w:r></w:p>
    <w:p><w:r><w:t xml:space="preserve">Hello </w:t></w:r><w:r><w:t>world.</w:t></w:r></w:p>
    <w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>First item</w:t></w:r></w:p>
    <w:tbl>
      <w:tr><w:tc><w:p><w:r><w:t>Name</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Value</w:t></w:r></w:p></w:tc></w:tr>
      <w:tr><w:tc><w:p><w:r><w:t>Type</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>MI-15</w:t></w:r></w:p></w:tc></w:tr>
    </w:tbl>
  </w:body>
</w:document>`

func writeDocx(t *testing.T, path, documentXML string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	part, err := zw.Create("word/document.xml")
	require.NoError(t, err)
	_, err = part.Write([]byte(documentXML))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
}

func TestViewDocumentTool_DocxReturnsMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "report.docx")
	writeDocx(t, filePath, sampleDocxDocumentXML)

	ctx := toolctx.WithWorkingDir(context.Background(), tmpDir)
	tool := tools.NewViewDocumentTool(&events.NoOpPublisher{})
	handler := tool.Handler()

	result, err := handler(ctx, map[string]any{
		"file_path":        "report.docx",
		"_display_message": "Reading the coversheet",
	})
	require.NoError(t, err)

	require.True(t, result.Details["success"].(bool), "expected success, got error: %v", result.Details["error"])
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", result.Details["mime_type"])
	assert.Equal(t, "report.docx", result.Details["path"])

	content, _ := result.Details["content"].(string)
	assert.Contains(t, content, "# Coversheet")
	assert.Contains(t, content, "Hello world.")
	assert.Contains(t, content, "- First item")
	assert.Contains(t, content, "| Name | Value |")
	assert.Contains(t, content, "| Type | MI-15 |")

	_, hasBase64 := result.Details["data_base64"]
	_, hasDataURL := result.Details["data_url"]
	assert.False(t, hasBase64, "docx results must travel as text, not base64")
	assert.False(t, hasDataURL, "docx results must travel as text, not a data URL")
}

func TestViewDocumentTool_DocxWithoutDocumentPart(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "broken.docx")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	part, err := zw.Create("unrelated.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("not a word document"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, os.WriteFile(filePath, buf.Bytes(), 0o600))

	ctx := toolctx.WithWorkingDir(context.Background(), tmpDir)
	tool := tools.NewViewDocumentTool(&events.NoOpPublisher{})
	handler := tool.Handler()

	result, err := handler(ctx, map[string]any{
		"file_path":        "broken.docx",
		"_display_message": "Reading a broken file",
	})
	require.NoError(t, err)
	assert.False(t, result.Details["success"].(bool))
	assert.Contains(t, result.Details["error"], "word/document.xml")
}

func TestViewDocumentTool_PathOutsideWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := toolctx.WithWorkingDir(context.Background(), tmpDir)

	tool := tools.NewViewDocumentTool(&events.NoOpPublisher{})
	handler := tool.Handler()

	result, err := handler(ctx, map[string]any{
		"file_path":        "../secret.pdf",
		"_display_message": "Testing",
	})
	require.NoError(t, err)

	success, _ := result.Details["success"].(bool)
	assert.False(t, success)
	assert.Contains(t, result.Details["error"], "outside")
}

func TestViewDocumentTool_SizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := toolctx.WithWorkingDir(context.Background(), tmpDir)

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
	assert.False(t, result.Details["success"].(bool))
	assert.Contains(t, result.Details["error"], "exceeds maximum")
}

func TestViewDocumentTool_MissingDisplayMessage(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := toolctx.WithWorkingDir(context.Background(), tmpDir)

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
	ctx := toolctx.WithWorkingDir(context.Background(), tmpDir)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "doc.txt"), []byte("hello"), 0o600))

	tool := tools.NewViewDocumentTool(&events.NoOpPublisher{})
	handler := tool.Handler()

	result, err := handler(ctx, map[string]any{
		"file_path":        "doc.txt",
		"_display_message": "Should fail",
	})
	require.NoError(t, err)
	assert.False(t, result.Details["success"].(bool))
	assert.Contains(t, result.Details["error"], "unsupported")
}
