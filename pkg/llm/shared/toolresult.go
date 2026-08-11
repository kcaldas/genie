package shared

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"unicode/utf8"

	"github.com/kcaldas/genie/pkg/config"
)

// DefaultMaxToolResultBytes bounds one tool result's serialized size.
// A tool result is appended to the conversation whole and is not
// subject to the context budget, which distributes across context-part
// providers only — so an unbounded result is the one input that can
// exceed a model's window in a single step regardless of every other
// limit. Sized well above real tool surfaces (a large file read, a wide
// grep over a source tree) so the cap only bounds pathology.
const DefaultMaxToolResultBytes = 128 * 1024

// MaxToolResultBytesFromEnv resolves the per-result cap from
// GENIE_MAX_TOOL_RESULT_BYTES, falling back to the default. A
// configured value of 0 or less disables capping, which is a
// deliberate opt-out for callers that own their own bounds.
func MaxToolResultBytesFromEnv(configManager config.Manager) int {
	return configManager.GetIntWithDefault("GENIE_MAX_TOOL_RESULT_BYTES", DefaultMaxToolResultBytes)
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

// capToolResult bounds a tool result's serialized size to limit bytes.
//
// It trims the largest string field first and repeats until the whole
// result fits, which keeps small structural fields (success flags,
// error text, counts) intact while the bulk payload absorbs the cut. A
// result with no string field large enough to trim — a pathological
// nested structure — is replaced wholesale, because passing it on would
// defeat the cap.
//
// The zero or negative limit disables capping.
func capToolResult(name string, result map[string]any, limit int) map[string]any {
	if limit <= 0 || result == nil {
		return result
	}

	origBytes := serializedLen(result)
	if origBytes <= limit {
		return result
	}

	capped := make(map[string]any, len(result))
	for k, v := range result {
		capped[k] = v
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
	return capped
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
