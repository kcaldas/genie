package shared

import (
	"fmt"
	"log"
	"strings"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/llm/shared/toolpayload"
)

// AttachmentKind is how an attachment is delivered when a provider
// supports it.
type AttachmentKind int

const (
	AttachmentDocument AttachmentKind = iota
	AttachmentImage
)

func (k AttachmentKind) String() string {
	if k == AttachmentImage {
		return "image"
	}
	return "document"
}

// Attachment is binary content a tool returned alongside its text.
// The bytes are decoded once, here, rather than in each adapter.
type Attachment struct {
	Kind     AttachmentKind
	MIMEType string
	Data     []byte
	Base64   string
	Path     string
}

// DataURL renders the attachment as a data URI, for providers whose
// image parts take a URL.
func (a Attachment) DataURL() string {
	return fmt.Sprintf("data:%s;base64,%s", a.MIMEType, a.Base64)
}

// Describe names the attachment for a text part accompanying it.
func (a Attachment) Describe() string {
	return fmt.Sprintf("%s retrieved from %s", strings.Title(a.Kind.String()), toolpayload.SanitizePath(a.Path))
}

// SupportsImagesOnly is the common `supports` policy: providers that
// render images natively but have no document part.
func SupportsImagesOnly(a Attachment) bool { return a.Kind == AttachmentImage }

// PreparedToolResult is a tool's outcome in model-facing form: the body
// a provider serializes into the tool response, and the attachments it
// encodes natively. Errors are already folded into Body — providers
// never see a raw handler error or reinterpret a raw result map.
type PreparedToolResult struct {
	Call        ToolCall
	Body        map[string]any
	Attachments []Attachment
}

// ToolResultLimits bounds what one step of tool calls may add to the
// conversation. Bodies and attachments are bounded separately because
// they are different things: body text competes with the context the
// model reasons over, while an attachment is delivered natively and
// costs whatever the provider charges for media.
type ToolResultLimits struct {
	// MaxBodyBytes caps one serialized body. Zero takes the default;
	// negative disables capping.
	MaxBodyBytes int
	// MaxAttachmentBytes caps one decoded attachment. An attachment
	// over it is reported in the body instead of delivered.
	MaxAttachmentBytes int
	// MaxBatchBytes caps the bodies of one step's results together.
	// Per-result limits alone still let twenty parallel calls overflow
	// a small window.
	MaxBatchBytes int
}

func (l ToolResultLimits) withDefaults() ToolResultLimits {
	if l.MaxBodyBytes == 0 {
		l.MaxBodyBytes = DefaultMaxToolResultBytes
	} else if l.MaxBodyBytes > 0 && l.MaxBodyBytes < MinMaxToolResultBytes {
		l.MaxBodyBytes = MinMaxToolResultBytes
	}
	if l.MaxAttachmentBytes == 0 {
		l.MaxAttachmentBytes = DefaultMaxAttachmentBytes
	}
	if l.MaxBatchBytes == 0 {
		l.MaxBatchBytes = DefaultMaxBatchBytes
	}
	return l
}

// batchBudget spends a step's shared body allowance across its results
// in execution order. Earlier results keep full fidelity and later ones
// tighten, which is deterministic; splitting the allowance evenly would
// truncate a small result that had room, and dropping the tail would
// lose whichever tool happened to run last.
type batchBudget struct {
	remaining int
	unlimited bool
}

func newBatchBudget(limits ToolResultLimits) *batchBudget {
	if limits.MaxBatchBytes < 0 {
		return &batchBudget{unlimited: true}
	}
	return &batchBudget{remaining: limits.MaxBatchBytes}
}

// bodyLimit is the cap for the next result: the per-result limit, or
// what is left of the step's allowance when that is smaller.
func (b *batchBudget) bodyLimit(perResult int) int {
	if b.unlimited || perResult < 0 {
		return perResult
	}
	if b.remaining < perResult {
		return maxInt(b.remaining, 0)
	}
	return perResult
}

func (b *batchBudget) spend(n int) {
	if !b.unlimited {
		b.remaining = maxInt(b.remaining-n, 0)
	}
}

// prepareToolResult turns a handler's outcome into model-facing form:
// errors become body text, attachments are lifted out of the body and
// validated, and the body is capped once — measured as exactly what the
// provider will serialize.
func prepareToolResult(bus events.EventBus, call ToolCall, result map[string]any, handlerErr error, limits ToolResultLimits, budget *batchBudget) PreparedToolResult {
	body := result
	var attachments []Attachment

	if handlerErr != nil {
		// The model sees the failure as the tool's output so it can
		// recover — apologise, retry with different arguments,
		// escalate — instead of the turn aborting.
		if bus != nil {
			bus.Publish(events.NotificationEvent{}.Topic(), events.NotificationEvent{
				Message: fmt.Sprintf("tool %s returned error: %v", call.Name, handlerErr),
			})
		}
		body = map[string]any{
			"error": fmt.Sprintf("tool %q returned an error: %v", call.Name, handlerErr),
		}
	} else {
		body, attachments = liftAttachments(call.Name, result, limits.MaxAttachmentBytes)
	}

	limit := budget.bodyLimit(limits.MaxBodyBytes)
	body = capToolResult(call.Name, body, limit)
	budget.spend(serializedLen(body))

	return PreparedToolResult{Call: call, Body: body, Attachments: attachments}
}

// liftAttachments moves inline binary data out of the body. A payload
// that cannot be used — no MIME type, undecodable, or larger than the
// attachment limit — is replaced by a note rather than left in place:
// leaving it would put megabytes of base64 through the text cap, which
// truncates it into noise the model cannot act on.
func liftAttachments(name string, result map[string]any, maxBytes int) (map[string]any, []Attachment) {
	payload, sanitized, ok := toolpayload.Native(result)
	if sanitized == nil {
		return result, nil
	}
	if !ok {
		if _, declared := result["data_base64"]; !declared {
			return sanitized, nil
		}
		log.Printf("tool %q: inline payload is unusable — reporting it in the body", name)
		toolpayload.DropNativeFields(sanitized)
		sanitized["attachment_error"] = "the inline payload could not be read (missing MIME type or invalid encoding); it was omitted"
		return sanitized, nil
	}

	if maxBytes > 0 && len(payload.Data) > maxBytes {
		log.Printf("tool %q: attachment of %d bytes exceeds the %d-byte limit — reporting it in the body",
			name, len(payload.Data), maxBytes)
		toolpayload.DropNativeFields(sanitized)
		sanitized["attachment_error"] = fmt.Sprintf(
			"the %s attachment (%d bytes) exceeds the %d-byte limit and was omitted; request a smaller one",
			payload.MIMEType, len(payload.Data), maxBytes)
		return sanitized, nil
	}

	return sanitized, []Attachment{{
		Kind:     attachmentKind(payload.MIMEType),
		MIMEType: payload.MIMEType,
		Data:     payload.Data,
		Base64:   payload.Base64Data,
		Path:     payload.Path,
	}}
}

func attachmentKind(mimeType string) AttachmentKind {
	if toolpayload.IsImageMIME(mimeType) {
		return AttachmentImage
	}
	return AttachmentDocument
}

// SplitAttachments partitions a prepared result's attachments by what
// the provider can encode, returning the body to serialize and the
// attachments to deliver.
//
// What a provider cannot render is reported in the body rather than
// dropped: the model is told the content exists and why it did not
// arrive, instead of silently receiving nothing. Support is per
// provider — Gemini takes formats the Messages API will not — so the
// decision belongs here, at encode time, not in a shared allowlist.
func SplitAttachments(result PreparedToolResult, supports func(Attachment) bool) (map[string]any, []Attachment) {
	if len(result.Attachments) == 0 {
		return result.Body, nil
	}

	supported := make([]Attachment, 0, len(result.Attachments))
	var unsupported []Attachment
	for _, attachment := range result.Attachments {
		if supports(attachment) {
			supported = append(supported, attachment)
			continue
		}
		unsupported = append(unsupported, attachment)
	}

	if len(unsupported) == 0 {
		return result.Body, supported
	}

	body := make(map[string]any, len(result.Body)+1)
	for k, v := range result.Body {
		body[k] = v
	}
	notes := make([]string, 0, len(unsupported))
	for _, attachment := range unsupported {
		notes = append(notes, fmt.Sprintf("%s (%s, %d bytes) cannot be displayed by this model",
			toolpayload.SanitizePath(attachment.Path), attachment.MIMEType, len(attachment.Data)))
	}
	body["attachment_error"] = fmt.Sprintf("%s; ask for a supported format if the content matters", notes[0])
	if len(notes) > 1 {
		body["attachment_error"] = fmt.Sprintf("%d attachments could not be displayed by this model: %v", len(notes), notes)
	}
	return body, supported
}
