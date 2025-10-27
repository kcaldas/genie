package toolimage

import (
	"encoding/base64"
	"fmt"
	"maps"
	"strings"
)

// Result captures the structured information returned by the viewImage tool.
type Result struct {
	Path       string
	MIMEType   string
	SizeBytes  int64
	Base64Data string
	Data       []byte
}

func (r Result) DataURL() string {
	return fmt.Sprintf("data:%s;base64,%s", r.MIMEType, r.Base64Data)
}

// SanitizePath returns a short descriptor safe to include in model context.
func SanitizePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "viewImage"
	}
	return trimmed
}

// Extract processes the generic tool response map and returns a well-typed result
// alongside a sanitized map that omits large inline payloads.
func Extract(input map[string]any) (*Result, map[string]any, error) {
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
		return nil, sanitized, fmt.Errorf("missing base64-encoded image data")
	}

	mimeType, _ := input["mime_type"].(string)
	if mimeType == "" {
		return nil, sanitized, fmt.Errorf("missing MIME type for image data")
	}

	sizeBytes, err := asInt64(input["size_bytes"])
	if err != nil {
		return nil, sanitized, fmt.Errorf("invalid size_bytes: %w", err)
	}

	path, _ := input["path"].(string)

	data, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, sanitized, fmt.Errorf("invalid base64 image data: %w", err)
	}

	delete(sanitized, "data_base64")
	delete(sanitized, "data_url")

	return &Result{
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
