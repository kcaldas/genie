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

// MaxNativePayloadBytes bounds one decoded native payload. The text cap
// deliberately does not apply to these fields, so without a limit of
// their own any tool — an MCP server in particular, which is bound by
// none of the built-in tools' own limits — could return hundreds of
// megabytes, force repeated decoding, and blow the provider's request
// limit. Matches the largest built-in tool ceiling (viewDocument).
const MaxNativePayloadBytes = 20 * 1024 * 1024

// deliverableMIMETypes are the payload types every provider adapter can
// render as native media. The set is shared rather than per-provider on
// purpose: a payload is stripped from the tool result exactly when it
// will be delivered, so a type one provider could send and another
// could not would be silently dropped on the second.
var deliverableMIMETypes = map[string]Kind{
	"image/png":       KindImage,
	"image/jpeg":      KindImage,
	"image/jpg":       KindImage,
	"image/gif":       KindImage,
	"image/webp":      KindImage,
	"application/pdf": KindDocument,
}

// Kind classifies a payload for delivery. Routing is by MIME type, not
// by which tool produced the result.
type Kind int

const (
	KindDocument Kind = iota
	KindImage
)

// Kind reports how the payload should be delivered to the model. Only
// deliverable types reach a Payload, so the lookup always hits.
func (p Payload) Kind() Kind {
	return deliverableMIMETypes[p.MIMEType]
}

// Native returns the inline binary payload a result declares, together
// with a copy of the result that omits the native fields.
//
// ok is false when the result declares no payload, and also when it
// declares one that cannot be used — no MIME type, undecodable base64.
// An unusable payload is left in the returned result rather than
// dropped or turned into an error, so it stays ordinary text subject to
// the size cap. That is the invariant callers depend on: a native field
// is either delivered as media or bounded as text, never exempt from
// both.
//
// Every caller — the size cap and each provider adapter — must decide
// with this one function. Two predicates that disagree is precisely how
// a field ends up exempted from the cap and then serialized as text
// anyway.
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

	// A type no adapter can render would be stripped here and then
	// dropped by the provider, losing the content silently. Leaving it
	// in the result keeps it visible to the model as text — capped like
	// any other field — which is the honest outcome.
	mimeType, _ := input["mime_type"].(string)
	if _, deliverable := deliverableMIMETypes[mimeType]; !deliverable {
		return nil, sanitized, false
	}

	// Reject before decoding: the encoded form is 4/3 the decoded size,
	// so this bounds the allocation as well as the payload.
	if int64(len(base64Str)) > MaxNativePayloadBytes*4/3+4 {
		return nil, sanitized, false
	}

	data, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil || int64(len(data)) > MaxNativePayloadBytes {
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
