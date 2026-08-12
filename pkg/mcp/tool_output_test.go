package mcp

import (
	"encoding/base64"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
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
	})

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
	})

	require.NoError(t, err)
	assert.Equal(t, []ai.ToolContent{ai.TextContent{Text: "guide body"}}, blocks)
}

func TestMCPContentBlocksRejectsInvalidBase64(t *testing.T) {
	_, err := mcpContentBlocks(Content{Type: "audio", Data: "not-base64", MIMEType: "audio/wav"})
	assert.ErrorContains(t, err, "decode base64")
}
