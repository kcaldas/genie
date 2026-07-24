package session

import (
	"bufio"
	"encoding/json"
	"io"
)

// maxLineBytes bounds a single JSONL line when reading. Entries are capped
// at write time well below this; the slack covers seeded metadata.
const maxLineBytes = 1 << 20 // 1MB

// GenericEntry is a tolerant view of one session entry: the common Base
// fields plus the full raw payload. Unknown entry types read fine.
type GenericEntry struct {
	Base
	Payload map[string]any
}

// ReadSession reads a session JSONL stream and returns its header and
// entries oldest-first. It is tolerant by design: unknown entry types are
// kept, and malformed lines (e.g. a crash-torn tail) are skipped.
func ReadSession(r io.Reader) (Header, []GenericEntry, error) {
	var header Header
	var entries []GenericEntry

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), maxLineBytes)

	first := true
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var payload map[string]any
		if err := json.Unmarshal(line, &payload); err != nil {
			continue // torn or corrupt line — skip
		}

		entryType, _ := payload["type"].(string)
		if first && entryType == "session" {
			if err := json.Unmarshal(line, &header); err == nil {
				first = false
				continue
			}
		}
		first = false

		entry := GenericEntry{Payload: payload}
		entry.Type = entryType
		entry.ID, _ = payload["id"].(string)
		entry.ParentID, _ = payload["parentId"].(string)
		entry.Timestamp, _ = payload["timestamp"].(string)
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return header, entries, err
	}
	return header, entries, nil
}

// NewestFirst returns a reversed copy of entries for newest-first paging.
func NewestFirst(entries []GenericEntry) []GenericEntry {
	reversed := make([]GenericEntry, len(entries))
	for i, entry := range entries {
		reversed[len(entries)-1-i] = entry
	}
	return reversed
}
