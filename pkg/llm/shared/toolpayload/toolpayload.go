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
		delete(sanitized, "data_base64")
		delete(sanitized, "data_url")
		return nil, sanitized, nil
	}

	base64Str, ok := input["data_base64"].(string)
	if !ok || base64Str == "" {
		delete(sanitized, "data_base64")
		delete(sanitized, "data_url")
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

	delete(sanitized, "data_base64")
	delete(sanitized, "data_url")

	return &Payload{
		Path:       path,
		MIMEType:   mimeType,
		SizeBytes:  sizeBytes,
		Base64Data: base64Str,
		Data:       data,
	}, sanitized, nil
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
