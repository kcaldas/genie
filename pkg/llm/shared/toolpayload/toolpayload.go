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
// data. They are stripped from the sanitized result: the bytes leave as
// a provider-native media message and never reach the model as JSON
// text. The convention is open to any tool, MCP servers included — a
// result declares binary content by putting it in these fields.
var nativeFields = []string{"data_base64", "data_url"}

// IsImageMIME reports whether a payload is an image. Classification
// only — whether a given provider can render it is that provider's
// decision, made where the request is built.
func IsImageMIME(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

// Native parses the inline binary payload a result declares, together
// with a copy of the result that omits the native fields.
//
// This is the compatibility parser for tools that return binary content
// as `data_base64` in an untyped result map, which is every built-in
// tool today. It has one caller — the shared preparation step, which
// converts what it returns into a typed attachment. When handlers
// return typed output directly, this goes away.
//
// ok is false when the result declares no payload or declares one that
// cannot be read; the caller decides what to report.
func Native(input map[string]any) (*Payload, map[string]any, bool) {
	if input == nil {
		return nil, nil, false
	}

	sanitized := maps.Clone(input)

	base64Str, ok := input["data_base64"].(string)
	if !ok || base64Str == "" {
		return nil, sanitized, false
	}

	if success, ok := input["success"].(bool); ok && !success {
		// A failed call carries no usable media; drop the fields rather
		// than ship a payload the tool itself disowned.
		dropNativeFields(sanitized)
		return nil, sanitized, false
	}

	mimeType, _ := input["mime_type"].(string)
	if mimeType == "" {
		return nil, sanitized, false
	}

	data, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, sanitized, false
	}

	// A tool need not report the size; the decoded length is authoritative
	// anyway, so an absent or unparseable value is not worth failing over.
	sizeBytes, err := asInt64(input["size_bytes"])
	if err != nil || sizeBytes <= 0 {
		sizeBytes = int64(len(data))
	}

	path, _ := input["path"].(string)
	dropNativeFields(sanitized)

	return &Payload{
		Path:       path,
		MIMEType:   mimeType,
		SizeBytes:  sizeBytes,
		Base64Data: base64Str,
		Data:       data,
	}, sanitized, true
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

// DropNativeFields removes inline binary data from a result. Callers
// that report a payload instead of delivering it use this so the bytes
// do not fall through to the text limits, where they would be truncated
// into noise.
func DropNativeFields(result map[string]any) {
	dropNativeFields(result)
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
