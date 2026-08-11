package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"unicode/utf8"

	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/llm/shared/toolpayload"
)

// DefaultMaxToolResultBytes bounds one tool result's serialized size.
// A tool result is appended to the conversation whole and is not
// subject to the context budget, which distributes across context-part
// providers only — so an unbounded result is the one input that can
// exceed a model's window in a single step regardless of every other
// limit. Sized well above real tool surfaces (a large file read, a wide
// grep over a source tree) so the cap only bounds pathology.
const DefaultMaxToolResultBytes = 128 * 1024

// MinMaxToolResultBytes is the floor for a configured cap. Below it the
// replacement message for an untrimmable result would not itself fit,
// so the configured bound could not be honoured.
const MinMaxToolResultBytes = 4 * 1024

// DisabledToolResultCap turns capping off. Configuring a non-positive
// limit resolves to this, which survives LoopConfig defaulting — a
// plain 0 there means "unset" and takes the default.
const DisabledToolResultCap = -1

// MaxToolResultBytesFromEnv resolves the per-result cap from
// GENIE_MAX_TOOL_RESULT_BYTES, falling back to the default. A
// configured 0 or less disables capping — a deliberate opt-out for
// callers that own their own bounds — and resolves to
// DisabledToolResultCap so it is not mistaken for an unset field. A
// configured value below the floor is raised to it.
func MaxToolResultBytesFromEnv(configManager config.Manager) int {
	limit := configManager.GetIntWithDefault("GENIE_MAX_TOOL_RESULT_BYTES", DefaultMaxToolResultBytes)
	switch {
	case limit <= 0:
		return DisabledToolResultCap
	case limit < MinMaxToolResultBytes:
		log.Printf("GENIE_MAX_TOOL_RESULT_BYTES=%d is below the %d-byte floor — using the floor",
			limit, MinMaxToolResultBytes)
		return MinMaxToolResultBytes
	default:
		return limit
	}
}

// capToolError bounds a handler error's text. Providers discard the
// result of a failed call and build the model-facing payload from the
// error string alone, so an unbounded error — a command that failed
// with megabytes on stderr, an MCP server echoing a request — reaches
// the conversation by exactly the route a capped result cannot.
func capToolError(name string, err error, limit int) error {
	if err == nil || limit <= 0 {
		return err
	}
	text := err.Error()
	if len(text) <= limit {
		return err
	}

	room := limit - len(truncationNotice(len(text), 0, limit))
	if room < 0 {
		room = 0
	}

	// The notice states how much was kept, so its own length depends on
	// what the cut leaves. Shrink until the whole message fits.
	kept := truncateUTF8(text, room)
	out := kept + truncationNotice(len(text), len(kept), limit)
	for len(out) > limit && len(kept) > 0 {
		kept = truncateUTF8(kept, len(kept)-(len(out)-limit))
		out = kept + truncationNotice(len(text), len(kept), limit)
	}

	log.Printf("tool %q: error text truncated from %d bytes to fit the %d-byte limit", name, len(text), limit)
	return errors.New(out)
}

// truncationNotice is the marker a capped field carries in place of the
// dropped bytes. It states what was lost and what to do about it: the
// model reads this as the tool's own output, so a silent cut would have
// it reason over a partial result believing it complete.
func truncationNotice(origBytes, keptBytes, limit int) string {
	return fmt.Sprintf(
		"\n\n[genie: output truncated — %d bytes exceeded the %d-byte tool result limit, %d bytes shown. "+
			"The result is INCOMPLETE. Narrow the call (scope it to a path, a file pattern, or a more "+
			"specific query) and run it again rather than drawing conclusions from this excerpt.]",
		origBytes, limit, keptBytes,
	)
}

// capToolResult bounds the model-facing text of a tool result to limit
// bytes.
//
// Native payload fields are excluded from both the measurement and the
// trimming: they are stripped by toolpayload.Extract and delivered as a
// provider-native media message, so they are never text the model
// reads, and truncating one corrupts it. Any tool may use that
// convention to return binary content — the cap is on text, not on
// size per se.
//
// What remains is trimmed largest-string-field first, which keeps small
// structural fields (success flags, counts, paths) intact while the
// bulk absorbs the cut. A result with no trimmable string — a
// pathological nested structure — is replaced wholesale, because
// passing it on would defeat the cap.
//
// A zero or negative limit disables capping.
func capToolResult(name string, result map[string]any, limit int) map[string]any {
	if limit <= 0 || result == nil {
		return result
	}

	// Split the result: native payload fields are held aside untouched,
	// and only the text half is measured and trimmed.
	capped, native := splitNativeFields(result)

	origBytes := serializedLen(capped)
	if origBytes <= limit {
		return result
	}

	// Halve the largest field until the whole result fits. Halving
	// rather than computing an exact cut is deliberate: JSON escaping
	// means a field's serialized cost is not its byte length, so an
	// arithmetic target can undershoot forever. Each pass strictly
	// shrinks the result, so this terminates; a pass that fails to
	// shrink it — structural bulk the trim cannot reach — falls back to
	// replacing the result outright.
	for serializedLen(capped) > limit {
		key, text, ok := largestStringField(capped)
		if !ok || text == "" {
			return untrimmableResult(name, origBytes, limit)
		}

		before := serializedLen(capped)
		notice := truncationNotice(origBytes, 0, limit)

		// Aim for the room left by everything else, but never keep
		// more than half of what this pass started with.
		room := limit - (before - len(text)) - len(notice)
		if half := len(text) / 2; room > half || room < 0 {
			room = half
		}

		kept := truncateUTF8(text, room)
		capped[key] = kept + truncationNotice(origBytes, len(kept), limit)

		if serializedLen(capped) >= before {
			return untrimmableResult(name, origBytes, limit)
		}
	}

	log.Printf("tool %q: result truncated from %d bytes to fit the %d-byte limit", name, origBytes, limit)
	for k, v := range native {
		capped[k] = v
	}
	return capped
}

// splitNativeFields separates a result into its text half — everything
// a provider marshals into the tool message — and the native payload
// fields it strips out and sends as media.
func splitNativeFields(result map[string]any) (text, native map[string]any) {
	text = make(map[string]any, len(result))
	for k, v := range result {
		text[k] = v
	}
	native = make(map[string]any, len(toolpayload.NativeFields()))
	for _, field := range toolpayload.NativeFields() {
		if v, ok := text[field]; ok {
			native[field] = v
			delete(text, field)
		}
	}
	return text, native
}

// untrimmableResult stands in for a result that cannot be cut down to
// the limit. It reports failure so the model treats the call as one to
// redo, narrowed, rather than as data.
func untrimmableResult(name string, origBytes, limit int) map[string]any {
	log.Printf("tool %q: result of %d bytes exceeds the %d-byte limit and could not be truncated — replaced",
		name, origBytes, limit)
	return map[string]any{
		"success": false,
		"error": fmt.Sprintf(
			"result of %d bytes exceeded the %d-byte tool result limit and could not be truncated; "+
				"narrow the call and run it again", origBytes, limit),
	}
}

// serializedLen measures a result the way a provider will send it. A
// result that cannot be marshalled is reported as zero so capping
// leaves it to the provider's own error handling.
func serializedLen(result map[string]any) int {
	payload, err := json.Marshal(result)
	if err != nil {
		return 0
	}
	return len(payload)
}

// largestStringField returns the longest string value in the result,
// picking the lowest key name on a tie so the choice is deterministic.
func largestStringField(result map[string]any) (string, string, bool) {
	keys := make([]string, 0, len(result))
	for k := range result {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	best, bestText := "", ""
	for _, k := range keys {
		text, ok := result[k].(string)
		if !ok {
			continue
		}
		if best == "" || len(text) > len(bestText) {
			best, bestText = k, text
		}
	}
	return best, bestText, best != ""
}

// truncateUTF8 cuts text to at most max bytes on a rune boundary.
func truncateUTF8(text string, max int) string {
	if len(text) <= max {
		return text
	}
	cut := text[:max]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}
