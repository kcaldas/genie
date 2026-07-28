package session

import (
	"fmt"
	"maps"
)

// Turn groups one model call's records: the context summary entry, the
// fully reconstructed input parts at that point, and the entries that
// followed (tool calls, thinking, the message or error).
type Turn struct {
	// Context is the turn's context summary entry.
	Context GenericEntry
	// Parts holds each input part's reconstructed content at this turn,
	// materialized by replaying context_part prefix deltas.
	Parts map[string]string
	// Order lists part names in the recorded wire/cache order (empty for
	// files written before order recording).
	Order []string
	// Entries are the turn's records after the context summary.
	Entries []GenericEntry
	// Warnings lists reconstruction gaps: truncated deltas, hash
	// mismatches, or prefixes past the known basis. Parts remain
	// best-effort when present.
	Warnings []string
}

// Turns splits a session's entries into turns and reconstructs each
// turn's input parts by replaying context_part deltas oldest-first.
// Entries recorded before the first context entry attach to the first
// turn.
func Turns(entries []GenericEntry) []Turn {
	var turns []Turn
	state := make(map[string]string)
	var current *Turn
	var pendingWarnings []string
	var prelude []GenericEntry

	for _, entry := range entries {
		switch entry.Type {
		case EntryTypeContextPart:
			if warning := applyPartDelta(state, entry); warning != "" {
				pendingWarnings = append(pendingWarnings, warning)
			}
		case EntryTypeContext:
			turns = append(turns, Turn{Context: entry})
			current = &turns[len(turns)-1]
			current.Parts = snapshotParts(state, entry)
			current.Order = orderFromPayload(entry)
			current.Warnings = pendingWarnings
			pendingWarnings = nil
			if len(prelude) > 0 {
				current.Entries = append(current.Entries, prelude...)
				prelude = nil
			}
		default:
			if current != nil {
				current.Entries = append(current.Entries, entry)
			} else {
				prelude = append(prelude, entry)
			}
		}
	}
	return turns
}

// applyPartDelta replays one context_part entry onto the running part
// state and returns a warning for any reconstruction gap.
func applyPartDelta(state map[string]string, entry GenericEntry) string {
	name, _ := entry.Payload["name"].(string)
	if name == "" {
		return "context_part without a name skipped"
	}
	hash, _ := entry.Payload["hash"].(string)
	basedOn, _ := entry.Payload["basedOn"].(string)
	prefix := intFromPayload(entry.Payload, "commonPrefixBytes")

	suffix := ""
	truncated := false
	if content, ok := entry.Payload["content"].(map[string]any); ok {
		suffix, _ = content["text"].(string)
		truncated, _ = content["truncated"].(bool)
	}

	var text string
	if basedOn == "" {
		text = suffix
	} else {
		prev := state[name]
		if prefix > len(prev) {
			return fmt.Sprintf("%s: delta prefix %db exceeds known basis %db — part dropped from reconstruction", name, prefix, len(prev))
		}
		text = prev[:prefix] + suffix
	}
	state[name] = text

	if truncated {
		return fmt.Sprintf("%s: delta content was truncated at record time — reconstruction incomplete from here on", name)
	}
	if hash != "" && contentHash(text) != hash {
		return fmt.Sprintf("%s: reconstructed content does not match recorded hash %s — earlier gap or torn file", name, hash)
	}
	return ""
}

// snapshotParts copies the running state for the part names the context
// entry declares (state may hold parts absent from this turn).
func snapshotParts(state map[string]string, entry GenericEntry) map[string]string {
	parts := make(map[string]string)
	refs, _ := entry.Payload["parts"].(map[string]any)
	for name := range refs {
		if text, ok := state[name]; ok {
			parts[name] = text
		}
	}
	if len(refs) == 0 {
		// Order-less legacy entries: expose everything known.
		maps.Copy(parts, state)
	}
	return parts
}

func orderFromPayload(entry GenericEntry) []string {
	raw, _ := entry.Payload["order"].([]any)
	order := make([]string, 0, len(raw))
	for _, item := range raw {
		if name, ok := item.(string); ok {
			order = append(order, name)
		}
	}
	return order
}

func intFromPayload(payload map[string]any, key string) int {
	if value, ok := payload[key].(float64); ok {
		return int(value)
	}
	return 0
}
