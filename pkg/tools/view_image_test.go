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

const oneByOnePng = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGNgYAAAAAMAAWgmWQ0AAAAASUVORK5CYII="

type capturingPublisher struct {
	messages []events.ToolCallMessageEvent
}

func (c *capturingPublisher) Publish(topic string, event interface{}) {
	if topic != "tool.call.message" {
		return
	}
	if msg, ok := event.(events.ToolCallMessageEvent); ok {
		c.messages = append(c.messages, msg)
	}
}

func TestViewImageTool_Success(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "sample.png")

	data, err := base64.StdEncoding.DecodeString(oneByOnePng)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(imagePath, data, 0o600))

	ctx := context.WithValue(context.Background(), "cwd", tmpDir)

	publisher := &capturingPublisher{}
	tool := tools.NewViewImageTool(publisher)

	handler := tool.Handler()
	result, err := handler(ctx, map[string]any{
		"file_path":        "sample.png",
		"_display_message": "Reviewing UI mockup",
	})
	require.NoError(t, err)

	success, _ := result["success"].(bool)
	assert.True(t, success)
	assert.Equal(t, "image/png", result["mime_type"])
	assert.Equal(t, int64(len(data)), result["size_bytes"])
	assert.Equal(t, oneByOnePng, result["data_base64"])
	assert.Equal(t, "sample.png", result["path"])

	dataURL, _ := result["data_url"].(string)
	assert.Equal(t, "data:image/png;base64,"+oneByOnePng, dataURL)

	require.Len(t, publisher.messages, 1)
	assert.Equal(t, "viewImage", publisher.messages[0].ToolName)
	assert.Equal(t, "Reviewing UI mockup", publisher.messages[0].Message)

	formatted := tool.FormatOutput(result)
	assert.Contains(t, formatted, "Attached image `sample.png`")
}

func TestViewImageTool_PathOutsideWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.WithValue(context.Background(), "cwd", tmpDir)

	tool := tools.NewViewImageTool(&events.NoOpPublisher{})
	handler := tool.Handler()

	result, err := handler(ctx, map[string]any{
		"file_path":        "../outside.png",
		"_display_message": "Testing",
	})
	require.NoError(t, err)

	success, _ := result["success"].(bool)
	assert.False(t, success)
	assert.Contains(t, result["error"], "outside the working directory")
}

func TestViewImageTool_UnsupportedMimeType(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "notes.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("just text"), 0o600))

	ctx := context.WithValue(context.Background(), "cwd", tmpDir)
	tool := tools.NewViewImageTool(&events.NoOpPublisher{})
	handler := tool.Handler()

	result, err := handler(ctx, map[string]any{
		"file_path":        "notes.txt",
		"_display_message": "Should fail",
	})
	require.NoError(t, err)

	success, _ := result["success"].(bool)
	assert.False(t, success)
	assert.Contains(t, result["error"], "unsupported image type")
}

func TestViewImageTool_MissingDisplayMessage(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.WithValue(context.Background(), "cwd", tmpDir)

	data, err := base64.StdEncoding.DecodeString(oneByOnePng)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "img.png"), data, 0o600))

	publisher := &capturingPublisher{}
	tool := tools.NewViewImageTool(publisher)
	handler := tool.Handler()

	_, err = handler(ctx, map[string]any{
		"file_path": "img.png",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "_display_message")
}

func TestViewImageTool_SizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.WithValue(context.Background(), "cwd", tmpDir)

	data, err := base64.StdEncoding.DecodeString(oneByOnePng)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "img.png"), data, 0o600))

	// Lower the max size so the sample image is over the limit.
	tool := tools.NewViewImageTool(&events.NoOpPublisher{}, tools.WithMaxImageBytes(int64(len(data)-1)))
	handler := tool.Handler()
	result, err := handler(ctx, map[string]any{
		"file_path":        "img.png",
		"_display_message": "Should fail",
	})
	require.NoError(t, err)

	success, _ := result["success"].(bool)
	assert.False(t, success)
	assert.Contains(t, result["error"], "exceeds maximum supported size")
}
