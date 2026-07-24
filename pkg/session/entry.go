// Package session provides append-only session recording for Genie.
//
// A session file is JSONL: one header line followed by entry lines. Every
// entry embeds Base and carries a parent pointer to the previous entry, so
// the file forms a chain that branching and replay tooling can walk without
// any side index.
//
// Known limits (v1):
//   - Tool→turn attribution is positional: entries between message entries
//     belong to the following message entry. ToolExecutedEvent carries no
//     request ID, which is fine for the linear chains recorded today.
//   - Checkpoint cost for SpoolCheckpoint grows with file size (whole-file
//     publish per turn). Retention lifecycle bounds it; file rotation is the
//     first follow-up when files approach ~5MB.
//   - A crash loses only the in-flight turn's entries (everything since the
//     last checkpoint).
package session

// Header is the first line of a session file.
type Header struct {
	Type          string         `json:"type"` // always "session"
	Version       int            `json:"version"`
	ID            string         `json:"id"`
	Timestamp     string         `json:"timestamp"`
	Cwd           string         `json:"cwd"`
	ParentSession string         `json:"parentSession,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// Entry type discriminators (Base.Type values).
const (
	EntryTypeMessage       = "message"
	EntryTypeToolCall      = "tool_call"
	EntryTypePersonaChange = "persona_change"
	EntryTypePrune         = "prune"
	EntryTypeError         = "error"
	EntryTypeOverflow      = "overflow"
	EntryTypeCustom        = "custom"
)

// Base is embedded by every entry. ParentID points at the previous entry in
// the chain ("" for the first entry written by a process).
type Base struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	ParentID  string `json:"parentId,omitempty"`
	Timestamp string `json:"timestamp"`
}

// Field is a capped, redactable text excerpt. Truncated fields keep the
// original byte length; redacted fields keep the marker but no content so
// the chain and entry counting survive redaction.
type Field struct {
	Text      string `json:"text,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	OrigBytes int    `json:"origBytes,omitempty"`
	Redacted  bool   `json:"redacted,omitempty"`
}

// MessageEntry records one completed user/assistant turn pair.
type MessageEntry struct {
	Base
	RequestID string `json:"requestId,omitempty"`
	Model     string `json:"model,omitempty"`
	User      Field  `json:"user"`
	Assistant Field  `json:"assistant"`
}

// ToolCallEntry records one executed tool call with capped excerpts of its
// parameters and result.
type ToolCallEntry struct {
	Base
	ExecutionID string `json:"executionId,omitempty"`
	Tool        string `json:"tool"`
	Success     bool   `json:"success"`
	Params      *Field `json:"params,omitempty"`
	Result      *Field `json:"result,omitempty"`
}

// PersonaChangeEntry records a persona switch on the live session.
type PersonaChangeEntry struct {
	Base
	From string `json:"from,omitempty"`
	To   string `json:"to"`
}

// PruneEntry records a chat-history prune applied while assembling context.
type PruneEntry struct {
	Base
	Strategy     string `json:"strategy"`
	Total        int    `json:"total"`
	Kept         int    `json:"kept"`
	Dropped      int    `json:"dropped"`
	KeptTokens   int    `json:"keptTokens"`
	BudgetTokens int    `json:"budgetTokens"`
}

// ErrorEntry records a failed turn.
type ErrorEntry struct {
	Base
	RequestID string `json:"requestId,omitempty"`
	Error     Field  `json:"error"`
}

// OverflowEntry marks entries dropped after a turn hit its entry cap.
type OverflowEntry struct {
	Base
	Skipped int `json:"skipped"`
}

// CustomEntry is an opaque host-defined entry (e.g. Mutiro's "mutiro.turn"
// identity stamp). Genie records it verbatim and never interprets it.
type CustomEntry struct {
	Base
	CustomType string         `json:"customType"`
	Data       map[string]any `json:"data,omitempty"`
}
