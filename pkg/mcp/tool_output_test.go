package mcp

import (
	"encoding/base64"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPContentBlocksPreservesNativeMedia(t *testing.T) {
	raw := []byte{1, 2, 3, 4}
	blocks, err := mcpContentBlocks(Content{
		Type:     "image",
		Data:     base64.StdEncoding.EncodeToString(raw),
		MIMEType: "image/png",
		Name:     "capture.png",
	}, llmshared.DefaultMaxToolBlobBytes)

	require.NoError(t, err)
	require.Len(t, blocks, 1)
	blob, ok := blocks[0].(ai.BlobContent)
	require.True(t, ok)
	assert.Equal(t, raw, blob.Data)
	assert.Equal(t, "image/png", blob.MIMEType)
	assert.Equal(t, "capture.png", blob.Name)
}

func TestMCPContentBlocksConvertsEmbeddedResources(t *testing.T) {
	blocks, err := mcpContentBlocks(Content{
		Type: "resource",
		Resource: &ResourceContents{
			URI:  "file:///guide.md",
			Text: "guide body",
		},
	}, llmshared.DefaultMaxToolBlobBytes)

	require.NoError(t, err)
	assert.Equal(t, []ai.ToolContent{ai.TextContent{Text: "guide body"}}, blocks)
}

func TestMCPContentBlocksRejectsInvalidBase64(t *testing.T) {
	_, err := mcpContentBlocks(Content{Type: "audio", Data: "not-base64", MIMEType: "audio/wav"}, llmshared.DefaultMaxToolBlobBytes)
	assert.ErrorContains(t, err, "decode base64")
}

func TestMCPContentBlocksRejectsOversizedBlobBeforeDecode(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4})

	_, err := mcpContentBlocks(Content{Type: "image", Data: encoded, MIMEType: "image/png"}, 3)

	assert.ErrorContains(t, err, "may exceed the 3-byte attachment safety limit")
}

func TestMCPContentBlocksAllowsBlobAtExactLimit(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4})

	blocks, err := mcpContentBlocks(Content{Type: "image", Data: encoded, MIMEType: "image/png"}, 4)

	require.NoError(t, err)
	require.Len(t, blocks, 1)
}

func TestMCPContentDetailsDropsBase64(t *testing.T) {
	original := []Content{
		{Type: "image", Data: "aW1hZ2U=", MIMEType: "image/png"},
		{Type: "resource", Resource: &ResourceContents{URI: "file:///doc.pdf", MIMEType: "application/pdf", Blob: "cGRm"}},
	}

	details := mcpContentDetails(original)

	assert.Empty(t, details[0].Data)
	require.NotNil(t, details[1].Resource)
	assert.Empty(t, details[1].Resource.Blob)
	assert.Equal(t, "aW1hZ2U=", original[0].Data)
	assert.Equal(t, "cGRm", original[1].Resource.Blob)
}
