package toolpayload

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtract_BinaryPayload(t *testing.T) {
	data := []byte("%PDF-1.4 body")
	encoded := base64.StdEncoding.EncodeToString(data)

	payload, sanitized, err := Extract(map[string]any{
		"success":     true,
		"mime_type":   "application/pdf",
		"size_bytes":  int64(len(data)),
		"data_base64": encoded,
		"data_url":    "data:application/pdf;base64," + encoded,
		"path":        "report.pdf",
	})
	require.NoError(t, err)
	require.NotNil(t, payload)
	assert.Equal(t, data, payload.Data)
	assert.Equal(t, "application/pdf", payload.MIMEType)
	assert.NotContains(t, sanitized, "data_base64")
	assert.NotContains(t, sanitized, "data_url")
}

func TestExtract_TextOnlyResult(t *testing.T) {
	payload, sanitized, err := Extract(map[string]any{
		"success":   true,
		"mime_type": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"content":   "# Coversheet\n\nHello world.",
		"path":      "report.docx",
	})
	require.NoError(t, err, "text-form results carry their content in the tool result and have no binary payload")
	assert.Nil(t, payload)
	assert.Equal(t, "# Coversheet\n\nHello world.", sanitized["content"])
}

func TestExtract_MissingPayloadStillErrors(t *testing.T) {
	payload, _, err := Extract(map[string]any{
		"success":   true,
		"mime_type": "image/png",
		"path":      "shot.png",
	})
	require.Error(t, err)
	assert.Nil(t, payload)
}
