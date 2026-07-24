package session

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeEntries(t *testing.T, storage *MemoryStorage) []GenericEntry {
	t.Helper()
	_, entries, err := ReadSession(strings.NewReader(string(storage.Contents())))
	require.NoError(t, err)
	return entries
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Level
		wantErr bool
	}{
		{name: "empty is off", input: "", want: LevelOff},
		{name: "off", input: "off", want: LevelOff},
		{name: "standard", input: "standard", want: LevelStandard},
		{name: "full", input: "full", want: LevelFull},
		{name: "case insensitive", input: "Standard", want: LevelStandard},
		{name: "whitespace trimmed", input: " full ", want: LevelFull},
		{name: "unknown errors and defaults off", input: "verbose", want: LevelOff, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLevel(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewRecorder_LevelOffReturnsNil(t *testing.T) {
	storage := NewMemoryStorage()
	rec := NewRecorder(storage, LevelOff)
	assert.Nil(t, rec)

	rec = NewRecorder(nil, LevelStandard)
	assert.Nil(t, rec)
}

func TestNilRecorder_AllMethodsSafeAndWriteNothing(t *testing.T) {
	var rec *Recorder

	// None of these may panic.
	rec.BeginSession("s1", "/tmp", map[string]any{"k": "v"})
	rec.AppendMessageTurn("req", "model", "hi", "hello", RedactNone)
	rec.AppendToolCall("exec", "readFile", map[string]any{"path": "x"}, true, map[string]any{"ok": true})
	rec.AppendPersonaChange("a", "b")
	rec.AppendPrune("sliding_window", 10, 5, 5, 100, 200)
	rec.AppendError("req", assert.AnError)
	rec.AppendCustom("mutiro.turn", map[string]any{"conversation_id": "c1"})
	rec.EndTurn()
	rec.Close()
}

func TestRecorder_HeaderWritten(t *testing.T) {
	storage := NewMemoryStorage()
	rec := NewRecorder(storage, LevelStandard)
	require.NotNil(t, rec)

	rec.BeginSession("sess-1", "/work/dir", map[string]any{"persona": "genie", "seeded_messages": 2})

	header, entries, err := ReadSession(strings.NewReader(string(storage.Contents())))
	require.NoError(t, err)
	assert.Equal(t, "session", header.Type)
	assert.Equal(t, 1, header.Version)
	assert.Equal(t, "sess-1", header.ID)
	assert.Equal(t, "/work/dir", header.Cwd)
	assert.NotEmpty(t, header.Timestamp)
	assert.Equal(t, "genie", header.Metadata["persona"])
	assert.Empty(t, entries)
}

func TestRecorder_CapsAndTruncation(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		maxField int
	}{
		{name: "standard 1KB", level: LevelStandard, maxField: 1024},
		{name: "full 8KB", level: LevelFull, maxField: 8192},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMemoryStorage()
			rec := NewRecorder(storage, tt.level)
			require.NotNil(t, rec)
			rec.BeginSession("s", "/w", nil)

			long := strings.Repeat("x", tt.maxField+500)
			rec.AppendMessageTurn("req-1", "model-1", long, "short answer", RedactNone)
			rec.EndTurn()

			entries := decodeEntries(t, storage)
			require.Len(t, entries, 1)
			entry := entries[0]
			assert.Equal(t, "message", entry.Type)

			user, ok := entry.Payload["user"].(map[string]any)
			require.True(t, ok, "user field must be an object")
			assert.Equal(t, true, user["truncated"])
			assert.Equal(t, float64(tt.maxField+500), user["origBytes"])
			assert.Len(t, user["text"].(string), tt.maxField)

			asst, ok := entry.Payload["assistant"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "short answer", asst["text"])
			_, hasTrunc := asst["truncated"]
			assert.False(t, hasTrunc, "short field must not carry truncated marker")
		})
	}
}

func TestRecorder_RedactionTable(t *testing.T) {
	tests := []struct {
		name         string
		redact       Redaction
		userRedacted bool
		asstRedacted bool
		userTextKept bool
		asstTextKept bool
	}{
		{name: "none", redact: RedactNone, userTextKept: true, asstTextKept: true},
		{name: "user", redact: RedactUser, userRedacted: true, asstTextKept: true},
		{name: "assistant", redact: RedactAssistant, asstRedacted: true, userTextKept: true},
		{name: "all", redact: RedactAll, userRedacted: true, asstRedacted: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMemoryStorage()
			rec := NewRecorder(storage, LevelStandard)
			rec.BeginSession("s", "/w", nil)
			rec.AppendMessageTurn("req", "model", "the question", "the answer", tt.redact)
			rec.EndTurn()

			entries := decodeEntries(t, storage)
			require.Len(t, entries, 1, "redacted entries must still be recorded (chain/counting preserved)")

			user := entries[0].Payload["user"].(map[string]any)
			asst := entries[0].Payload["assistant"].(map[string]any)

			if tt.userRedacted {
				assert.Equal(t, true, user["redacted"])
				assert.Empty(t, user["text"], "redacted field must strip content")
			}
			if tt.asstRedacted {
				assert.Equal(t, true, asst["redacted"])
				assert.Empty(t, asst["text"], "redacted field must strip content")
			}
			if tt.userTextKept {
				assert.Equal(t, "the question", user["text"])
			}
			if tt.asstTextKept {
				assert.Equal(t, "the answer", asst["text"])
			}
		})
	}
}

func TestRecorder_CustomPassthrough(t *testing.T) {
	storage := NewMemoryStorage()
	rec := NewRecorder(storage, LevelStandard)
	rec.BeginSession("s", "/w", nil)

	rec.AppendCustom("mutiro.turn", map[string]any{
		"conversation_id": "conv-1",
		"message_id":      "msg-9",
		"sender":          "krishna",
		"role":            "owner",
	})
	rec.EndTurn()

	entries := decodeEntries(t, storage)
	require.Len(t, entries, 1)
	assert.Equal(t, "custom", entries[0].Type)
	assert.Equal(t, "mutiro.turn", entries[0].Payload["customType"])

	data, ok := entries[0].Payload["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "conv-1", data["conversation_id"])
	assert.Equal(t, "msg-9", data["message_id"])
	assert.Equal(t, "krishna", data["sender"])
	assert.Equal(t, "owner", data["role"])
}

func TestRecorder_ParentChain(t *testing.T) {
	storage := NewMemoryStorage()
	rec := NewRecorder(storage, LevelStandard)
	rec.BeginSession("s", "/w", nil)

	rec.AppendCustom("mutiro.turn", map[string]any{"conversation_id": "c"})
	rec.AppendToolCall("e1", "readFile", map[string]any{"path": "a"}, true, nil)
	rec.AppendToolCall("e2", "writeFile", map[string]any{"path": "b"}, true, nil)
	rec.AppendMessageTurn("req", "model", "q", "a", RedactNone)
	rec.EndTurn()

	entries := decodeEntries(t, storage)
	require.Len(t, entries, 4)

	assert.Empty(t, entries[0].ParentID, "first entry has no parent")
	for i := 1; i < len(entries); i++ {
		require.NotEmpty(t, entries[i].ID)
		assert.Equal(t, entries[i-1].ID, entries[i].ParentID,
			"entry %d must chain to previous entry", i)
	}
}

func TestRecorder_OverflowEntry(t *testing.T) {
	storage := NewMemoryStorage()
	rec := NewRecorder(storage, LevelStandard)
	rec.BeginSession("s", "/w", nil)

	// Standard caps at 200 entries per turn; write 205.
	for i := 0; i < 205; i++ {
		rec.AppendToolCall("e", "tool", nil, true, nil)
	}
	rec.EndTurn()

	entries := decodeEntries(t, storage)
	require.Len(t, entries, 201, "200 kept + 1 overflow marker")

	last := entries[len(entries)-1]
	assert.Equal(t, "overflow", last.Type)
	assert.Equal(t, float64(5), last.Payload["skipped"])

	// Overflow counter resets between turns.
	rec.AppendToolCall("e", "tool", nil, true, nil)
	rec.EndTurn()
	entries = decodeEntries(t, storage)
	assert.Len(t, entries, 202, "no overflow entry on a turn under the cap")
}

func TestRecorder_EndTurnCheckpoints(t *testing.T) {
	storage := NewMemoryStorage()
	rec := NewRecorder(storage, LevelStandard)
	rec.BeginSession("s", "/w", nil)

	rec.AppendMessageTurn("r1", "m", "q1", "a1", RedactNone)
	rec.EndTurn()
	assert.Equal(t, 1, storage.CheckpointCount())

	rec.AppendMessageTurn("r2", "m", "q2", "a2", RedactNone)
	rec.EndTurn()
	assert.Equal(t, 2, storage.CheckpointCount())
}

func TestRecorder_AppendError(t *testing.T) {
	storage := NewMemoryStorage()
	rec := NewRecorder(storage, LevelStandard)
	rec.BeginSession("s", "/w", nil)

	rec.AppendError("req-1", assert.AnError)
	rec.EndTurn()

	entries := decodeEntries(t, storage)
	require.Len(t, entries, 1)
	assert.Equal(t, "error", entries[0].Type)
	assert.Equal(t, "req-1", entries[0].Payload["requestId"])
	errField := entries[0].Payload["error"].(map[string]any)
	assert.Contains(t, errField["text"], assert.AnError.Error())
}

func TestRecorder_AppendPruneAndPersonaChange(t *testing.T) {
	storage := NewMemoryStorage()
	rec := NewRecorder(storage, LevelStandard)
	rec.BeginSession("s", "/w", nil)

	rec.AppendPrune("sliding_window", 20, 12, 8, 900, 1000)
	rec.AppendPersonaChange("genie", "architect")
	rec.EndTurn()

	entries := decodeEntries(t, storage)
	require.Len(t, entries, 2)

	prune := entries[0]
	assert.Equal(t, "prune", prune.Type)
	assert.Equal(t, "sliding_window", prune.Payload["strategy"])
	assert.Equal(t, float64(20), prune.Payload["total"])
	assert.Equal(t, float64(12), prune.Payload["kept"])
	assert.Equal(t, float64(8), prune.Payload["dropped"])
	assert.Equal(t, float64(900), prune.Payload["keptTokens"])
	assert.Equal(t, float64(1000), prune.Payload["budgetTokens"])

	persona := entries[1]
	assert.Equal(t, "persona_change", persona.Type)
	assert.Equal(t, "genie", persona.Payload["from"])
	assert.Equal(t, "architect", persona.Payload["to"])
}

func TestRecorder_ToolCallFields(t *testing.T) {
	storage := NewMemoryStorage()
	rec := NewRecorder(storage, LevelStandard)
	rec.BeginSession("s", "/w", nil)

	rec.AppendToolCall("exec-1", "readFile",
		map[string]any{"path": "/etc/hosts"},
		true,
		map[string]any{"content": "127.0.0.1 localhost"},
	)
	rec.EndTurn()

	entries := decodeEntries(t, storage)
	require.Len(t, entries, 1)
	entry := entries[0]
	assert.Equal(t, "tool_call", entry.Type)
	assert.Equal(t, "exec-1", entry.Payload["executionId"])
	assert.Equal(t, "readFile", entry.Payload["tool"])
	assert.Equal(t, true, entry.Payload["success"])

	params := entry.Payload["params"].(map[string]any)
	assert.Contains(t, params["text"], "/etc/hosts")
	result := entry.Payload["result"].(map[string]any)
	assert.Contains(t, result["text"], "localhost")
}

func TestRecorder_CloseCheckpointsAndStopsWrites(t *testing.T) {
	storage := NewMemoryStorage()
	rec := NewRecorder(storage, LevelStandard)
	rec.BeginSession("s", "/w", nil)
	rec.AppendMessageTurn("r", "m", "q", "a", RedactNone)
	rec.Close()

	assert.True(t, storage.Closed())
	require.GreaterOrEqual(t, storage.CheckpointCount(), 1, "Close must flush pending entries")

	before := len(decodeEntries(t, storage))
	rec.AppendMessageTurn("r2", "m", "q2", "a2", RedactNone)
	rec.EndTurn()
	assert.Len(t, decodeEntries(t, storage), before, "writes after Close are dropped")
}
