package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

// Redaction controls which sides of a message turn keep their content.
// Redacted entries are still written — chain and counting survive — but the
// stripped fields carry only the redacted marker.
type Redaction int

const (
	// RedactNone keeps both sides.
	RedactNone Redaction = iota
	// RedactUser strips the user input.
	RedactUser
	// RedactAssistant strips the assistant response.
	RedactAssistant
	// RedactAll strips both sides.
	RedactAll
)

// Recorder appends typed entries to a session Storage. All methods are
// nil-receiver-safe so callers never guard, and recording failures are
// logged and swallowed — recording must never fail a turn.
type Recorder struct {
	mu      sync.Mutex
	storage Storage
	caps    caps

	lastID      string
	turnEntries int
	turnSkipped int
	closed      bool

	// lastContext holds each context part's previous hash (and, when
	// content deltas are recorded, its content — the basis for the next
	// turn's prefix diff).
	lastContext map[string]contextPartState
}

// NewRecorder builds a Recorder for the given storage and level. Returns
// nil — the disabled recorder — when storage is nil or level is off.
func NewRecorder(storage Storage, level Level) *Recorder {
	if storage == nil || level == LevelOff {
		return nil
	}
	return &Recorder{storage: storage, caps: capsFor(level)}
}

// BeginSession writes the session header. Storages skip it when the file
// already has content (continuation).
func (r *Recorder) BeginSession(sessionID, cwd string, metadata map[string]any) {
	if r == nil {
		return
	}
	header := Header{
		Type:      "session",
		Version:   1,
		ID:        sessionID,
		Timestamp: now(),
		Cwd:       cwd,
		Metadata:  metadata,
	}
	data, err := json.Marshal(header)
	if err != nil {
		slog.Warn("session recording: marshal header failed", "error", err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	if err := r.storage.WriteHeader(data); err != nil {
		slog.Warn("session recording: write header failed", "error", err)
	}
}

// AppendMessageTurn records one completed user/assistant turn pair.
func (r *Recorder) AppendMessageTurn(requestID, model, user, assistant string, redact Redaction) {
	if r == nil {
		return
	}
	entry := MessageEntry{
		RequestID: requestID,
		Model:     model,
		User:      r.redactableField(user, redact == RedactUser || redact == RedactAll),
		Assistant: r.redactableField(assistant, redact == RedactAssistant || redact == RedactAll),
	}
	r.append(EntryTypeMessage, &entry.Base, &entry)
}

// AppendToolCall records one executed tool call with capped excerpts of the
// parameters and result.
func (r *Recorder) AppendToolCall(executionID, tool string, params map[string]any, success bool, result map[string]any) {
	if r == nil {
		return
	}
	entry := ToolCallEntry{
		ExecutionID: executionID,
		Tool:        tool,
		Success:     success,
		Params:      r.jsonExcerpt(params),
		Result:      r.jsonExcerpt(result),
	}
	r.append(EntryTypeToolCall, &entry.Base, &entry)
}

// AppendContext records the composition of the model's input for the
// upcoming turn. At every level it appends a context entry with each
// part's hash, size, and changed flag. At LevelFull it additionally
// records the content of changed parts as prefix-delta context_part
// entries (see ContextPartEntry), so "what did the model see" is
// answerable verbatim even after the sources (memory, history, files)
// have moved on.
func (r *Recorder) AppendContext(parts map[string]string) {
	if r == nil || len(parts) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}

	refs := make(map[string]ContextPartRef, len(parts))
	total := 0
	// Deterministic entry order for the delta entries.
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		content := parts[name]
		hash := contentHash(content)
		prev, seen := r.lastContext[name]
		changed := !seen || prev.hash != hash
		refs[name] = ContextPartRef{Hash: hash, Bytes: len(content), Changed: changed}
		total += len(content)

		if changed && r.caps.recordContextParts {
			part := ContextPartEntry{Name: name, Hash: hash}
			suffix := content
			if seen {
				part.BasedOn = prev.hash
				part.CommonPrefixBytes = commonPrefixLen(prev.content, content)
				suffix = content[part.CommonPrefixBytes:]
			}
			part.Content = capField(suffix, r.caps.maxContextPartBytes)
			r.appendLocked(EntryTypeContextPart, &part.Base, &part)
		}

		state := contextPartState{hash: hash}
		if r.caps.recordContextParts {
			// Content retained only when deltas are recorded — it is
			// the basis for the next turn's prefix diff.
			state.content = content
		}
		if r.lastContext == nil {
			r.lastContext = make(map[string]contextPartState)
		}
		r.lastContext[name] = state
	}

	entry := ContextEntry{Parts: refs, TotalBytes: total}
	r.appendLocked(EntryTypeContext, &entry.Base, &entry)
}

// appendLocked writes one entry under the caller-held lock, enforcing
// the per-turn entry cap like append.
func (r *Recorder) appendLocked(entryType string, base *Base, entry any) {
	if r.turnEntries >= r.caps.maxEntriesPerTurn {
		r.turnSkipped++
		return
	}
	if r.writeLocked(entryType, base, entry) {
		r.turnEntries++
	}
}

// contextPartState is the per-part basis for the next turn's delta.
type contextPartState struct {
	hash    string
	content string
}

// contentHash is a short content digest for context parts.
func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:8])
}

// commonPrefixLen returns how many leading bytes a and b share, aligned
// down to a valid UTF-8 boundary so the recorded suffix never starts on
// a torn rune.
func commonPrefixLen(a, b string) int {
	n := min(len(a), len(b))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	// Back off a continuation byte at the split so the suffix starts on
	// a rune boundary.
	for i > 0 && i < len(b) && b[i]&0xC0 == 0x80 {
		i--
	}
	return i
}

// AppendThinking records the model's aggregated reasoning for a response.
// No-op below LevelFull: thinking is rawer than output, so standard
// recordings stay reasoning-free.
func (r *Recorder) AppendThinking(text string) {
	if r == nil || !r.caps.recordThinking {
		return
	}
	if strings.TrimSpace(text) == "" {
		return
	}
	entry := ThinkingEntry{Text: capField(text, r.caps.maxFieldBytes)}
	r.append(EntryTypeThinking, &entry.Base, &entry)
}

// AppendPersonaChange records a persona switch.
func (r *Recorder) AppendPersonaChange(from, to string) {
	if r == nil {
		return
	}
	entry := PersonaChangeEntry{From: from, To: to}
	r.append(EntryTypePersonaChange, &entry.Base, &entry)
}

// AppendPrune records a chat-history prune.
func (r *Recorder) AppendPrune(strategy string, total, kept, dropped, keptTokens, budgetTokens int) {
	if r == nil {
		return
	}
	entry := PruneEntry{
		Strategy:     strategy,
		Total:        total,
		Kept:         kept,
		Dropped:      dropped,
		KeptTokens:   keptTokens,
		BudgetTokens: budgetTokens,
	}
	r.append(EntryTypePrune, &entry.Base, &entry)
}

// AppendError records a failed turn.
func (r *Recorder) AppendError(requestID string, err error) {
	if r == nil {
		return
	}
	message := ""
	if err != nil {
		message = err.Error()
	}
	entry := ErrorEntry{
		RequestID: requestID,
		Error:     capField(message, r.caps.maxFieldBytes),
	}
	r.append(EntryTypeError, &entry.Base, &entry)
}

// AppendCustom records an opaque host-defined entry. Oversized payloads are
// replaced by a truncation marker (hard cap) rather than partially written.
func (r *Recorder) AppendCustom(customType string, data map[string]any) {
	if r == nil {
		return
	}
	if raw, err := json.Marshal(data); err != nil {
		slog.Warn("session recording: marshal custom data failed", "customType", customType, "error", err)
		data = map[string]any{"truncated": true}
	} else if len(raw) > 4*r.caps.maxFieldBytes {
		data = map[string]any{"truncated": true, "origBytes": len(raw)}
	}
	entry := CustomEntry{CustomType: customType, Data: data}
	r.append(EntryTypeCustom, &entry.Base, &entry)
}

// EndTurn closes the current turn: writes the overflow marker when entries
// were dropped, checkpoints the storage, and resets the turn counters.
func (r *Recorder) EndTurn() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	if r.turnSkipped > 0 {
		entry := OverflowEntry{Skipped: r.turnSkipped}
		r.writeLocked(EntryTypeOverflow, &entry.Base, &entry)
	}
	r.turnEntries = 0
	r.turnSkipped = 0
	if err := r.storage.Checkpoint(); err != nil {
		slog.Warn("session recording: checkpoint failed", "error", err)
	}
}

// Close checkpoints pending entries and closes the storage. Later appends
// are dropped.
func (r *Recorder) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	if err := r.storage.Checkpoint(); err != nil {
		slog.Warn("session recording: final checkpoint failed", "error", err)
	}
	if err := r.storage.Close(); err != nil {
		slog.Warn("session recording: close failed", "error", err)
	}
}

// append writes one entry under the recorder lock, enforcing the per-turn
// entry cap.
func (r *Recorder) append(entryType string, base *Base, entry any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.appendLocked(entryType, base, entry)
}

// writeLocked stamps the base fields, marshals and appends. Callers hold
// r.mu. Returns whether the entry reached storage.
func (r *Recorder) writeLocked(entryType string, base *Base, entry any) bool {
	base.Type = entryType
	base.ID = newEntryID()
	base.ParentID = r.lastID
	base.Timestamp = now()

	data, err := json.Marshal(entry)
	if err != nil {
		slog.Warn("session recording: marshal entry failed", "entryType", entryType, "error", err)
		return false
	}
	if err := r.storage.AppendEntry(data); err != nil {
		slog.Warn("session recording: append entry failed", "entryType", entryType, "error", err)
		return false
	}
	r.lastID = base.ID
	return true
}

func (r *Recorder) redactableField(text string, redact bool) Field {
	if redact {
		return Field{Redacted: true}
	}
	return capField(text, r.caps.maxFieldBytes)
}

// jsonExcerpt renders a map as a capped JSON excerpt field. Nil maps yield
// no field at all.
func (r *Recorder) jsonExcerpt(value map[string]any) *Field {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		slog.Warn("session recording: marshal excerpt failed", "error", err)
		return &Field{Truncated: true}
	}
	field := capField(string(raw), r.caps.maxFieldBytes)
	return &field
}

func newEntryID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "e-" + hex.EncodeToString([]byte(time.Now().Format("150405.000000000")))
	}
	return "e-" + hex.EncodeToString(buf)
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
