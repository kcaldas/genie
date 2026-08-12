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

// ToolResultLimits bounds model-facing text. Native blobs deliberately bypass
// these synthetic byte limits; a provider capability layer must account for
// their encoded token cost against the real model input envelope.
type ToolResultLimits struct {
	MaxTextBytes      int
	MaxBatchTextBytes int
}

func (l ToolResultLimits) withDefaults() ToolResultLimits {
	if l.MaxTextBytes == 0 {
		l.MaxTextBytes = DefaultMaxToolTextBytes
	} else if l.MaxTextBytes > 0 && l.MaxTextBytes < MinMaxToolTextBytes {
		l.MaxTextBytes = MinMaxToolTextBytes
	}
	if l.MaxBatchTextBytes == 0 {
		l.MaxBatchTextBytes = DefaultMaxBatchTextBytes
	}
	return l
}

type batchBudget struct {
	textRemaining int
	textUnlimited bool
}

func newBatchBudget(limits ToolResultLimits) *batchBudget {
	return &batchBudget{
		textRemaining: limits.MaxBatchTextBytes,
		textUnlimited: limits.MaxBatchTextBytes < 0,
	}
}

func (b *batchBudget) textLimit(perResult int) int {
	return effectiveLimit(perResult, b.textRemaining, b.textUnlimited)
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
	resultTextRemaining := limits.MaxTextBytes
	textUnlimited := limits.MaxTextBytes < 0

	appendText := func(text string) {
		limit := budget.textLimit(resultTextRemaining)
		if textUnlimited {
			limit = budget.textLimit(-1)
		}
		if limit == 0 {
			return
		}
		bounded := truncateToolText(text, limit)
		if bounded == "" {
			return
		}
		prepared = append(prepared, ai.TextContent{Text: bounded})
		used := len(bounded)
		budget.spendText(used)
		if !textUnlimited {
			resultTextRemaining = maxInt(resultTextRemaining-used, 0)
		}
	}

	for _, block := range content {
		switch value := block.(type) {
		case ai.TextContent:
			appendText(value.Text)
		case ai.JSONContent:
			serialized, err := json.Marshal(value.Value)
			if err != nil {
				appendText(fmt.Sprintf("tool output JSON could not be serialized: %v", err))
				continue
			}
			appendText(string(serialized))
		case ai.BlobContent:
			if strings.TrimSpace(value.MIMEType) == "" || len(value.Data) == 0 {
				appendText(fmt.Sprintf("[%s returned an unusable binary item; it was omitted]", toolName))
				continue
			}
			prepared = append(prepared, value)
		default:
			appendText(fmt.Sprintf("[%s returned unsupported content type %T; it was omitted]", toolName, block))
		}
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

	notice := fmt.Sprintf("\n[tool output truncated: %d bytes total]", len(text))
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
