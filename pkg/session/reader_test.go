package session

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadSession_RoundTrip(t *testing.T) {
	raw := strings.Join([]string{
		`{"type":"session","version":1,"id":"s1","timestamp":"2026-07-24T10:00:00Z","cwd":"/w","metadata":{"persona":"genie"}}`,
		`{"type":"custom","id":"e1","timestamp":"t1","customType":"mutiro.turn","data":{"conversation_id":"c1"}}`,
		`{"type":"tool_call","id":"e2","parentId":"e1","timestamp":"t2","tool":"readFile"}`,
		`{"type":"message","id":"e3","parentId":"e2","timestamp":"t3","requestId":"r1"}`,
	}, "\n") + "\n"

	header, entries, err := ReadSession(strings.NewReader(raw))
	require.NoError(t, err)

	assert.Equal(t, "session", header.Type)
	assert.Equal(t, 1, header.Version)
	assert.Equal(t, "s1", header.ID)
	assert.Equal(t, "/w", header.Cwd)
	assert.Equal(t, "genie", header.Metadata["persona"])

	require.Len(t, entries, 3)
	assert.Equal(t, "custom", entries[0].Type)
	assert.Equal(t, "e1", entries[0].ID)
	assert.Equal(t, "tool_call", entries[1].Type)
	assert.Equal(t, "e1", entries[1].ParentID)
	assert.Equal(t, "message", entries[2].Type)
	assert.Equal(t, "r1", entries[2].Payload["requestId"])
}

func TestReadSession_TolerantOfUnknownEntryTypes(t *testing.T) {
	raw := `{"type":"session","version":1,"id":"s1"}` + "\n" +
		`{"type":"future_thing","id":"e1","timestamp":"t1","whatever":42}` + "\n"

	_, entries, err := ReadSession(strings.NewReader(raw))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "future_thing", entries[0].Type)
	assert.Equal(t, float64(42), entries[0].Payload["whatever"])
}

func TestReadSession_SkipsMalformedLines(t *testing.T) {
	raw := `{"type":"session","version":1,"id":"s1"}` + "\n" +
		`{"type":"custom","id":"e1"}` + "\n" +
		`{"type":"custom","id":"e2","trunc` // crash-torn tail

	_, entries, err := ReadSession(strings.NewReader(raw))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "e1", entries[0].ID)
}

func TestReadSession_EmptyInput(t *testing.T) {
	header, entries, err := ReadSession(strings.NewReader(""))
	require.NoError(t, err)
	assert.Empty(t, header.ID)
	assert.Empty(t, entries)
}

func TestNewestFirst(t *testing.T) {
	entries := []GenericEntry{
		{Base: Base{ID: "e1"}},
		{Base: Base{ID: "e2"}},
		{Base: Base{ID: "e3"}},
	}

	reversed := NewestFirst(entries)
	require.Len(t, reversed, 3)
	assert.Equal(t, "e3", reversed[0].ID)
	assert.Equal(t, "e1", reversed[2].ID)

	// Input must not be mutated.
	assert.Equal(t, "e1", entries[0].ID)
}
