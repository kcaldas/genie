package toolpayload

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNative_BinaryPayload(t *testing.T) {
	data := []byte("%PDF-1.4 body")
	encoded := base64.StdEncoding.EncodeToString(data)

	payload, sanitized, ok := Native(map[string]any{
		"success":     true,
		"mime_type":   "application/pdf",
		"size_bytes":  int64(len(data)),
		"data_base64": encoded,
		"data_url":    "data:application/pdf;base64," + encoded,
		"path":        "report.pdf",
	})
	require.True(t, ok)
	require.NotNil(t, payload)
	assert.Equal(t, data, payload.Data)
	assert.Equal(t, "application/pdf", payload.MIMEType)
	assert.Equal(t, KindDocument, payload.Kind())
	assert.NotContains(t, sanitized, "data_base64")
	assert.NotContains(t, sanitized, "data_url")
}

func TestNative_TextOnlyResult(t *testing.T) {
	payload, sanitized, ok := Native(map[string]any{
		"success":   true,
		"mime_type": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"content":   "# Coversheet\n\nHello world.",
		"path":      "report.docx",
	})
	require.False(t, ok, "text-form results carry their content in the tool result and have no binary payload")
	assert.Nil(t, payload)
	assert.Equal(t, "# Coversheet\n\nHello world.", sanitized["content"])
}

// An unusable payload must stay in the result rather than error or
// vanish: callers exempt native fields from the size cap only when this
// reports a payload, so anything it rejects has to remain cappable text.
func TestNative_UnusablePayloadStaysInTheResult(t *testing.T) {
	cases := map[string]map[string]any{
		"missing mime type": {
			"success":     true,
			"data_base64": base64.StdEncoding.EncodeToString([]byte("body")),
		},
		"undecodable base64": {
			"success":     true,
			"mime_type":   "image/png",
			"data_base64": strings.Repeat("!", 32),
		},
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			payload, sanitized, ok := Native(input)

			require.False(t, ok)
			assert.Nil(t, payload)
			assert.Contains(t, sanitized, "data_base64",
				"an unextractable field must remain subject to the text cap")
		})
	}
}

func TestNative_FailedCallDropsPayload(t *testing.T) {
	payload, sanitized, ok := Native(map[string]any{
		"success":     false,
		"mime_type":   "image/png",
		"data_base64": base64.StdEncoding.EncodeToString([]byte("body")),
		"error":       "file not found",
	})

	require.False(t, ok)
	assert.Nil(t, payload)
	assert.NotContains(t, sanitized, "data_base64", "a disowned payload should not be shipped as media or text")
}

// Delivery is chosen by MIME type, so a tool that is not viewImage still
// gets its screenshot rendered as an image.
func TestNative_KindRoutesByMIMEType(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("body"))

	payload, _, ok := Native(map[string]any{
		"success":     true,
		"mime_type":   "image/webp",
		"data_base64": encoded,
	})

	require.True(t, ok)
	assert.Equal(t, KindImage, payload.Kind())
}

func TestNative_MissingSizeFallsBackToDecodedLength(t *testing.T) {
	data := []byte("body bytes")

	payload, _, ok := Native(map[string]any{
		"success":     true,
		"mime_type":   "image/png",
		"data_base64": base64.StdEncoding.EncodeToString(data),
	})

	require.True(t, ok)
	assert.Equal(t, int64(len(data)), payload.SizeBytes)
}
