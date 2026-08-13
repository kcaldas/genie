package shared

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// PreparedToolResult is a tool outcome after the shared model-facing output
// policy has been applied. Details remain on Output for host consumers but are
// never inspected by providers.
type PreparedToolResult struct {
	Call   ToolCall
	Output ai.ToolOutput
}

// ToolResultLimits bounds model-facing text and individual blob allocations.
// MaxBlobBytes is an operational memory/transport guard, not model-context
// accounting; provider turns separately admit native content against the real
// input envelope where an exact token-count API exists.
type ToolResultLimits struct {
	MaxTextBytes      int
	MaxBatchTextBytes int
	MaxBlobBytes      int

	transientTextAllowance    int
	hasTransientTextAllowance bool
}

const estimatedTextBytesPerToken = 3

// withTransientAllowance bounds the pending result batch by the room left in
// the next internal request. The latest input already includes every earlier
// call/result in this user turn; the latest output will also be replayed. This
// never changes the persistent, cross-turn context budget.
func (l ToolResultLimits) withTransientAllowance(inputLimit int, usage *ai.TokenCount) ToolResultLimits {
	if inputLimit <= 0 || usage == nil {
		return l
	}
	used := int(usage.InputTokens) + int(usage.OutputTokens)
	// Some providers include reasoning or other replayed output only in the
	// total. Never admit against less than the provider's complete count.
	if total := int(usage.TotalTokens); total > used {
		used = total
	}
	remainingTokens := inputLimit - used
	if remainingTokens < 0 {
		remainingTokens = 0
	}
	// Tool output is predominantly source code and structured text. Three
	// bytes/token is deliberately conservative; exact-count providers perform
	// an authoritative check for media and candidates near the boundary.
	allowance := remainingTokens * estimatedTextBytesPerToken
	if !l.hasTransientTextAllowance || allowance < l.transientTextAllowance {
		l.transientTextAllowance = allowance
		l.hasTransientTextAllowance = true
	}
	return l
}

// NeedsExactToolResultAdmission reports whether a provider should pay for an
// authoritative token-count request before appending a tool result. Native
// media always needs provider accounting. Text-only results use the latest
// physical usage and the same conservative estimate as the shared batch cap;
// exact counting starts once the candidate reaches 75% of the input envelope.
// Missing usage also requires an exact count because no local admission signal
// is available.
func NeedsExactToolResultAdmission(inputLimit int, latestUsage *ai.TokenCount, results []PreparedToolResult) bool {
	if inputLimit <= 0 {
		return false
	}
	if latestUsage == nil {
		return true
	}

	textBytes := 0
	for _, result := range results {
		for _, block := range result.Output.Content {
			switch value := block.(type) {
			case ai.BlobContent:
				return true
			case ai.TextContent:
				textBytes += len(value.Text)
			case ai.JSONContent:
				// Prepared results normally contain only text and blobs, but
				// retain a conservative estimate for manually constructed turns.
				if encoded, err := json.Marshal(value.Value); err == nil {
					textBytes += len(encoded)
				}
			}
		}
	}

	used := int(latestUsage.InputTokens) + int(latestUsage.OutputTokens)
	if total := int(latestUsage.TotalTokens); total > used {
		used = total
	}
	estimatedPending := (textBytes + estimatedTextBytesPerToken - 1) / estimatedTextBytesPerToken
	return (used+estimatedPending)*4 >= inputLimit*3
}

func (l ToolResultLimits) withDefaults() ToolResultLimits {
	if l.MaxTextBytes == 0 {
		l.MaxTextBytes = DefaultMaxToolTextBytes
	} else if l.MaxTextBytes > 0 && l.MaxTextBytes < MinMaxToolTextBytes {
		l.MaxTextBytes = MinMaxToolTextBytes
	}
	if l.MaxBatchTextBytes == 0 {
		l.MaxBatchTextBytes = DefaultMaxBatchTextBytes
	} else if l.MaxBatchTextBytes > 0 && l.MaxBatchTextBytes < MinMaxToolTextBytes {
		l.MaxBatchTextBytes = MinMaxToolTextBytes
	}
	if l.MaxBlobBytes == 0 {
		l.MaxBlobBytes = DefaultMaxToolBlobBytes
	}
	return l
}

type batchBudget struct {
	textRemaining    int
	textUnlimited    bool
	resultsRemaining int
}

const toolOutputOmittedNotice = "[tool output omitted: step text budget exhausted; narrow the tool call and retry]"

// OmittedToolResult preserves call correlation and host details while replacing
// model-facing content with a small admission notice.
func OmittedToolResult(result PreparedToolResult, reason string) PreparedToolResult {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "model input budget exhausted"
	}
	result.Output.Content = []ai.ToolContent{ai.TextContent{Text: fmt.Sprintf(
		"[tool output omitted: %s; narrow the tool call and retry]", reason,
	)}}
	return result
}

func newBatchBudget(limits ToolResultLimits, resultCount ...int) *batchBudget {
	count := 1
	if len(resultCount) > 0 && resultCount[0] > 0 {
		count = resultCount[0]
	}
	remaining := limits.MaxBatchTextBytes
	if limits.hasTransientTextAllowance && (remaining < 0 || limits.transientTextAllowance < remaining) {
		remaining = limits.transientTextAllowance
	}
	if remaining >= 0 {
		minimum := count * len(toolOutputOmittedNotice)
		if remaining < minimum {
			log.Printf("tool step text budget raised from %d to %d bytes so every result can carry an omission notice", remaining, minimum)
			remaining = minimum
		}
	}
	return &batchBudget{
		textRemaining:    remaining,
		textUnlimited:    remaining < 0,
		resultsRemaining: count,
	}
}

func (b *batchBudget) beginResult(perResult int) int {
	if b.resultsRemaining > 0 {
		b.resultsRemaining--
	}
	if b.textUnlimited {
		return perResult
	}
	reserved := b.resultsRemaining * len(toolOutputOmittedNotice)
	available := maxInt(b.textRemaining-reserved, 0)
	return effectiveLimit(perResult, available, false)
}

func effectiveLimit(perItem, remaining int, batchUnlimited bool) int {
	switch {
	case batchUnlimited:
		return perItem
	case remaining <= 0:
		return 0
	case perItem < 0:
		return remaining
	case remaining < perItem:
		return remaining
	default:
		return perItem
	}
}

func (b *batchBudget) spendText(n int) {
	if !b.textUnlimited {
		b.textRemaining = maxInt(b.textRemaining-n, 0)
	}
}

func prepareToolResult(
	bus events.EventBus,
	call ToolCall,
	output ai.ToolOutput,
	handlerErr error,
	limits ToolResultLimits,
	budget *batchBudget,
) PreparedToolResult {
	if handlerErr != nil {
		if bus != nil {
			bus.Publish(events.NotificationEvent{}.Topic(), events.NotificationEvent{
				Message: fmt.Sprintf("tool %s returned error: %v", call.Name, handlerErr),
			})
		}
		details := map[string]any{
			"error": fmt.Sprintf("tool %q returned an error: %v", call.Name, handlerErr),
		}
		output = ai.ErrorToolOutput(details)
	}

	output.Content = applyToolOutputLimits(call.Name, output.Content, limits, budget)
	return PreparedToolResult{Call: call, Output: output}
}

// applyToolOutputLimits validates and bounds every model-facing content block.
// JSON becomes text here so providers receive a small, closed set of content
// types and all serialization is measured exactly once.
func applyToolOutputLimits(
	toolName string,
	content []ai.ToolContent,
	limits ToolResultLimits,
	budget *batchBudget,
) []ai.ToolContent {
	prepared := make([]ai.ToolContent, 0, len(content))
	var diagnostics []string
	var textParts []string
	var blobs []ai.BlobContent

	for _, block := range content {
		switch value := block.(type) {
		case ai.TextContent:
			if value.Text != "" {
				textParts = append(textParts, value.Text)
			}
		case ai.JSONContent:
			serialized, err := json.Marshal(value.Value)
			if err != nil {
				diagnostics = append(diagnostics, fmt.Sprintf("tool output JSON could not be serialized: %v", err))
				continue
			}
			textParts = append(textParts, string(serialized))
		case ai.BlobContent:
			if strings.TrimSpace(value.MIMEType) == "" || len(value.Data) == 0 {
				diagnostics = append(diagnostics, fmt.Sprintf("[%s returned an unusable binary item; it was omitted]", toolName))
				continue
			}
			if limits.MaxBlobBytes >= 0 && len(value.Data) > limits.MaxBlobBytes {
				diagnostics = append(diagnostics, fmt.Sprintf(
					"[%s returned %s (%d bytes), exceeding the %d-byte attachment safety limit; it was omitted]",
					toolName, value.MIMEType, len(value.Data), limits.MaxBlobBytes,
				))
				continue
			}
			blobs = append(blobs, value)
		default:
			diagnostics = append(diagnostics, fmt.Sprintf("[%s returned unsupported content type %T; it was omitted]", toolName, block))
		}
	}

	allText := append(diagnostics, textParts...)
	if combined := strings.Join(allText, "\n"); combined != "" {
		limit := budget.beginResult(limits.MaxTextBytes)
		var bounded string
		switch {
		case limit < 0:
			bounded = combined
		case limit <= 0:
			bounded = toolOutputOmittedNotice
		case len(combined) <= limit:
			bounded = combined
		case limit <= len(toolOutputOmittedNotice):
			bounded = truncateUTF8(toolOutputOmittedNotice, limit)
		default:
			bounded = truncateToolText(combined, limit)
		}
		if bounded != "" {
			prepared = append(prepared, ai.TextContent{Text: bounded})
			budget.spendText(len(bounded))
		}
	} else {
		_ = budget.beginResult(limits.MaxTextBytes)
	}
	for _, blob := range blobs {
		prepared = append(prepared, blob)
	}

	return prepared
}

// EncodedToolResult is the provider-neutral encoding view. Text is ready for a
// tool result field; supported blobs are returned separately for native parts.
type EncodedToolResult struct {
	Text    string
	Blobs   []ai.BlobContent
	IsError bool
}

func EncodeToolResult(result PreparedToolResult, supports func(ai.BlobContent) bool) EncodedToolResult {
	var text []string
	blobs := make([]ai.BlobContent, 0)
	for _, block := range result.Output.Content {
		switch value := block.(type) {
		case ai.TextContent:
			if value.Text != "" {
				text = append(text, value.Text)
			}
		case ai.BlobContent:
			if supports != nil && supports(value) {
				blobs = append(blobs, value)
			} else {
				name := strings.TrimSpace(value.Name)
				if name == "" {
					name = "binary content"
				}
				text = append(text, fmt.Sprintf(
					"[%s (%s, %d bytes) cannot be displayed by this model]",
					name, value.MIMEType, len(value.Data),
				))
			}
		}
	}
	if len(text) == 0 {
		text = append(text, "(no tool output)")
	}
	return EncodedToolResult{
		Text:    strings.Join(text, "\n"),
		Blobs:   blobs,
		IsError: result.Output.IsError,
	}
}

// SupportsImagesOnly is the capability predicate used by chat-style
// providers that accept image inputs but not arbitrary binary content.
func SupportsImagesOnly(blob ai.BlobContent) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(blob.MIMEType)), "image/")
}

// SupportsBlobForModel refines a provider fallback with discovered per-model
// input modalities. A nil modality map means the provider did not report this
// information, so existing provider behavior remains intact.
func SupportsBlobForModel(caps *ai.ModelCapabilities, fallback func(ai.BlobContent) bool) func(ai.BlobContent) bool {
	if caps == nil || caps.InputModalities == nil {
		return fallback
	}
	return func(blob ai.BlobContent) bool {
		modality, ok := blobModality(blob.MIMEType)
		return ok && caps.SupportsInput(modality) && (fallback == nil || fallback(blob))
	}
}

func blobModality(mimeType string) (ai.Modality, bool) {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if idx := strings.IndexByte(mimeType, ';'); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return ai.ModalityImage, true
	case strings.HasPrefix(mimeType, "audio/"):
		return ai.ModalityAudio, true
	case strings.HasPrefix(mimeType, "video/"):
		return ai.ModalityVideo, true
	case mimeType == "application/pdf":
		return ai.ModalityDocument, true
	case strings.HasPrefix(mimeType, "text/"):
		return ai.ModalityText, true
	default:
		return "", false
	}
}

// DescribeBlob returns concise model-facing provenance for a binary block.
func DescribeBlob(blob ai.BlobContent) string {
	name := strings.TrimSpace(blob.Name)
	if name == "" {
		name = "tool attachment"
	}
	return fmt.Sprintf("%s (%s, %d bytes)", name, blob.MIMEType, len(blob.Data))
}

// BlobDataURL encodes a blob for providers that accept data URLs.
func BlobDataURL(blob ai.BlobContent) string {
	return fmt.Sprintf("data:%s;base64,%s", blob.MIMEType, base64.StdEncoding.EncodeToString(blob.Data))
}

// BlobBase64 encodes a blob for providers with a separate MIME type field.
func BlobBase64(blob ai.BlobContent) string {
	return base64.StdEncoding.EncodeToString(blob.Data)
}

func truncateToolText(text string, limit int) string {
	if limit < 0 || len(text) <= limit {
		return text
	}
	if limit == 0 {
		return ""
	}

	notice := fmt.Sprintf("\n[tool output truncated: %d bytes total. The result is INCOMPLETE; narrow the tool call and retry.]", len(text))
	if len(notice) >= limit {
		return truncateUTF8(notice, limit)
	}
	room := limit - len(notice)
	headBytes := room / 2
	tailBytes := room - headBytes
	head := truncateUTF8(text, headBytes)
	tail := truncateUTF8Tail(text[len(text)-minInt(tailBytes, len(text)):], tailBytes)
	bounded := head + notice + tail
	if len(bounded) > limit {
		bounded = truncateUTF8(bounded, limit)
	}
	log.Printf("tool result text truncated from %d bytes to %d bytes", len(text), len(bounded))
	return bounded
}

func truncateUTF8Tail(text string, max int) string {
	for len(text) > 0 && (text[0]&0xc0) == 0x80 {
		text = text[1:]
	}
	if len(text) <= max {
		return text
	}
	start := len(text) - max
	for start < len(text) && (text[start]&0xc0) == 0x80 {
		start++
	}
	return text[start:]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
