package session

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Round-trip: what the recorder writes as deltas, Turns must reconstruct
// byte-identically.
func TestTurns_ReconstructsPartsAcrossDeltas(t *testing.T) {
	storage := NewMemoryStorage()
	rec := NewRecorder(storage, LevelFull)

	turn1 := map[string]string{
		"rendered.instruction": "be helpful",
		"message":              "hello",
	}
	rec.AppendContext(turn1, "rendered.instruction")
	rec.AppendMessageTurn("r1", "test-model", "hello", "hi!", RedactNone)
	rec.EndTurn()

	turn2 := map[string]string{
		"rendered.instruction": "be helpful",
		"chat":                 "User: hello\nAssistant: hi!",
		"message":              "hello again, world",
	}
	rec.AppendContext(turn2, "rendered.instruction")
	rec.AppendToolCall("x1", "bash", map[string]any{"command": "ls"}, true, nil)
	rec.AppendMessageTurn("r2", "test-model", "hello again, world", "hi again", RedactNone)
	rec.EndTurn()

	// Turn 3: message diverges early from turn 2's (prefix delta path).
	turn3 := map[string]string{
		"rendered.instruction": "be helpful",
		"chat":                 "User: hello\nAssistant: hi!\nUser: hello again, world\nAssistant: hi again",
		"message":              "hello frank",
	}
	rec.AppendContext(turn3, "rendered.instruction")
	rec.AppendMessageTurn("r3", "test-model", "hello frank", "hi frank", RedactNone)
	rec.EndTurn()

	_, entries, err := ReadSession(strings.NewReader(string(storage.Contents())))
	require.NoError(t, err)
	turns := Turns(entries)
	require.Len(t, turns, 3)

	for i, want := range []map[string]string{turn1, turn2, turn3} {
		assert.Empty(t, turns[i].Warnings, "turn %d must reconstruct cleanly", i+1)
		assert.Equal(t, want, turns[i].Parts, "turn %d parts", i+1)
	}

	assert.Equal(t, []string{"rendered.instruction", "chat", "message"}, turns[1].Order)
	require.Len(t, turns[1].Entries, 2, "tool call + message belong to turn 2")
	assert.Equal(t, "tool_call", turns[1].Entries[0].Type)
	assert.Equal(t, "message", turns[1].Entries[1].Type)
}

func TestTurns_TruncatedDeltaWarnsAndStaysBestEffort(t *testing.T) {
	storage := NewMemoryStorage()
	rec := NewRecorder(storage, LevelFull)

	big := strings.Repeat("x", 300*1024) // over the 256KB context part cap
	rec.AppendContext(map[string]string{"chat": big})
	rec.EndTurn()

	_, entries, err := ReadSession(strings.NewReader(string(storage.Contents())))
	require.NoError(t, err)
	turns := Turns(entries)
	require.Len(t, turns, 1)

	require.NotEmpty(t, turns[0].Warnings, "truncation must surface as a warning")
	assert.Contains(t, turns[0].Warnings[0], "truncated")
	assert.Len(t, turns[0].Parts["chat"], 256*1024, "best-effort content kept")
}
