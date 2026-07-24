package genie_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/genie/genietest"
	"github.com/kcaldas/genie/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recordedEntries(t *testing.T, storage *session.MemoryStorage) []session.GenericEntry {
	t.Helper()
	_, entries, err := session.ReadSession(strings.NewReader(string(storage.Contents())))
	require.NoError(t, err)
	return entries
}

func TestRecording_ToolEntriesOrderedBeforeMessage(t *testing.T) {
	storage := session.NewMemoryStorage()
	recorder := session.NewRecorder(storage, session.LevelStandard)
	fixture := genietest.NewTestFixture(t, genietest.WithSessionRecorder(recorder))
	defer fixture.Cleanup()

	fixture.StartAndGetSession()

	message := "list my files"
	fixture.ExpectMessage(message).
		MockTool("listFiles").Returns(map[string]any{"files": "a.txt"}).
		MockTool("readFile").Returns(map[string]any{"content": "hello"}).
		RespondWith("done")

	require.NoError(t, fixture.Genie.Chat(context.Background(), message))
	fixture.WaitForResponseOrFail(2 * time.Second)

	entries := recordedEntries(t, storage)
	require.Len(t, entries, 3, "expected tool_call, tool_call, message")
	assert.Equal(t, "tool_call", entries[0].Type)
	assert.Equal(t, "listFiles", entries[0].Payload["tool"])
	assert.Equal(t, "tool_call", entries[1].Type)
	assert.Equal(t, "readFile", entries[1].Payload["tool"])
	assert.Equal(t, "message", entries[2].Type)

	user := entries[2].Payload["user"].(map[string]any)
	assert.Equal(t, message, user["text"])
	assistant := entries[2].Payload["assistant"].(map[string]any)
	assert.Equal(t, "done", assistant["text"])

	assert.Equal(t, 1, storage.CheckpointCount(), "exactly one checkpoint per turn")
}

func TestRecording_ErrorTurnRecordsErrorEntryOnly(t *testing.T) {
	storage := session.NewMemoryStorage()
	recorder := session.NewRecorder(storage, session.LevelStandard)
	fixture := genietest.NewTestFixture(t, genietest.WithSessionRecorder(recorder))
	defer fixture.Cleanup()

	fixture.StartAndGetSession()

	// No mock response configured for this message: processChat fails.
	require.NoError(t, fixture.Genie.Chat(context.Background(), "unconfigured message"))
	response := fixture.WaitForResponseOrFail(2 * time.Second)
	require.Error(t, response.Error)

	entries := recordedEntries(t, storage)
	require.Len(t, entries, 1)
	assert.Equal(t, "error", entries[0].Type)
	for _, entry := range entries {
		assert.NotEqual(t, "message", entry.Type, "failed turns must not record a message entry")
	}
	assert.Equal(t, 1, storage.CheckpointCount(), "error turns still checkpoint")
}

func TestRecording_PersonaChange(t *testing.T) {
	storage := session.NewMemoryStorage()
	recorder := session.NewRecorder(storage, session.LevelStandard)
	fixture := genietest.NewTestFixture(t, genietest.WithSessionRecorder(recorder))
	defer fixture.Cleanup()

	sess := fixture.StartAndGetSession()
	before := len(recordedEntries(t, storage))

	// Same persona: no entry.
	sess.SetPersona(sess.GetPersona())
	assert.Len(t, recordedEntries(t, storage), before, "unchanged persona must not record")

	sess.SetPersona(&genie.DefaultPersona{ID: "architect", Name: "Architect", Source: "test"})

	entries := recordedEntries(t, storage)
	require.Len(t, entries, before+1)
	last := entries[len(entries)-1]
	assert.Equal(t, "persona_change", last.Type)
	assert.Equal(t, "architect", last.Payload["to"])
	assert.NotEmpty(t, last.Payload["from"])
}

func TestRecording_HeaderWrittenOnStart(t *testing.T) {
	storage := session.NewMemoryStorage()
	recorder := session.NewRecorder(storage, session.LevelStandard)
	fixture := genietest.NewTestFixture(t, genietest.WithSessionRecorder(recorder))
	defer fixture.Cleanup()

	sess := fixture.StartAndGetSession()

	header, _, err := session.ReadSession(strings.NewReader(string(storage.Contents())))
	require.NoError(t, err)
	assert.Equal(t, "session", header.Type)
	assert.Equal(t, sess.GetID(), header.ID)
	assert.NotEmpty(t, header.Cwd)
	require.NotNil(t, header.Metadata)
	assert.Contains(t, header.Metadata, "persona")
	assert.Contains(t, header.Metadata, "seeded_messages")
}

func TestRecording_LevelOffWritesNothing(t *testing.T) {
	storage := session.NewMemoryStorage()
	// Level off yields a nil recorder — identical to the default NewGenie path.
	recorder := session.NewRecorder(storage, session.LevelOff)
	require.Nil(t, recorder)

	fixture := genietest.NewTestFixture(t, genietest.WithSessionRecorder(recorder))
	defer fixture.Cleanup()

	fixture.StartAndGetSession()
	message := "hello"
	fixture.ExpectSimpleMessage(message, "hi there")
	require.NoError(t, fixture.Genie.Chat(context.Background(), message))
	fixture.WaitForResponseOrFail(2 * time.Second)

	assert.Empty(t, storage.Contents(), "recording off must write nothing at all")
	assert.Equal(t, 0, storage.CheckpointCount())
}
