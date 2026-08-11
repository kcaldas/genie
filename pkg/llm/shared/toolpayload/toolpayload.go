package toolpayload

import (
	"encoding/base64"
	"fmt"
	"maps"
	"strings"
)

// Payload represents binary content returned by a tool such as viewImage or viewDocument.
type Payload struct {
	Path       string
	MIMEType   string
	SizeBytes  int64
	Base64Data string
	Data       []byte
}

// nativeFields are the result keys a tool uses to return inline binary
// data. Extract removes them from the sanitized result: they leave as a
// provider-native media message and never reach the model as JSON text.
// The convention is open to any tool, MCP servers included — a result
// declares binary content by putting it in these fields.
var nativeFields = []string{"data_base64", "data_url"}

// NativeFields returns the result keys holding inline binary data.
// Callers that size or trim a result for text limits must exclude
// these: they are megabytes by design, they are stripped before the
// result is marshalled, and cutting one corrupts the payload.
func NativeFields() []string {
	return append([]string(nil), nativeFields...)
}

// DataURL returns a data URI representation of the payload.
func (p Payload) DataURL() string {
	return fmt.Sprintf("data:%s;base64,%s", p.MIMEType, p.Base64Data)
}

// SanitizePath returns a short description safe to expose to the model.
func SanitizePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "tool payload"
	}
	return trimmed
}

// Extract decodes a tool result map, returning the binary payload and a sanitized copy
// that omits large inline data before it is re-marshalled into a tool response message.
func Extract(input map[string]any) (*Payload, map[string]any, error) {
	if input == nil {
		return nil, nil, fmt.Errorf("nil tool result")
	}

	sanitized := maps.Clone(input)

	success, ok := input["success"].(bool)
	if !ok || !success {
		dropNativeFields(sanitized)
		return nil, sanitized, nil
	}

	base64Str, ok := input["data_base64"].(string)
	if !ok || base64Str == "" {
		dropNativeFields(sanitized)
		// Text-form results (e.g. viewDocument on a .docx) carry their
		// extracted content in the tool result itself; there is no
		// binary payload to attach.
		if content, ok := input["content"].(string); ok && strings.TrimSpace(content) != "" {
			return nil, sanitized, nil
		}
		return nil, sanitized, fmt.Errorf("missing base64-encoded payload")
	}

	mimeType, _ := input["mime_type"].(string)
	if mimeType == "" {
		return nil, sanitized, fmt.Errorf("missing MIME type")
	}

	sizeBytes, err := asInt64(input["size_bytes"])
	if err != nil {
		return nil, sanitized, fmt.Errorf("invalid size_bytes: %w", err)
	}

	path, _ := input["path"].(string)

	data, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, sanitized, fmt.Errorf("invalid base64 data: %w", err)
	}

	dropNativeFields(sanitized)

	return &Payload{
		Path:       path,
		MIMEType:   mimeType,
		SizeBytes:  sizeBytes,
		Base64Data: base64Str,
		Data:       data,
	}, sanitized, nil
}

func dropNativeFields(result map[string]any) {
	for _, field := range nativeFields {
		delete(result, field)
	}
}

func asInt64(value any) (int64, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unexpected type %T", value)
	}
}
