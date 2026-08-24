package genie_test

import (
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/genie/genietest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The chat response event carries the turn's tool activity digest so
// hosts can persist it alongside the assistant message.
func TestChatResponseCarriesToolActivities(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	defer fixture.Cleanup()
	fixture.StartAndGetSession()

	fixture.ExpectMessage("fix the tests").
		MockTool("searchInFiles").Returns(map[string]any{"matches": 8}).
		MockTool("bash").Returns(map[string]any{"exit": 0}).
		RespondWith("done")

	require.NoError(t, fixture.StartChat("fix the tests"))
	response := fixture.WaitForResponseOrFail(2 * time.Second)

	require.Len(t, response.Activities, 2)
	assert.Equal(t, "searchInFiles", response.Activities[0].Tool)
	assert.Equal(t, "bash", response.Activities[1].Tool)
	assert.True(t, response.Activities[0].Success)
	assert.NotEmpty(t, response.Activities[0].Summary)
}

// Draining at turn end must remove the request's bucket: a later turn
// carries only its own activities.
func TestChatActivitiesDoNotLeakAcrossTurns(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	defer fixture.Cleanup()
	fixture.StartAndGetSession()

	fixture.ExpectMessage("first").
		MockTool("bash").Returns(map[string]any{"exit": 0}).
		RespondWith("one")
	fixture.ExpectMessage("second").
		MockTool("readFile").Returns(map[string]any{"results": "x"}).
		RespondWith("two")

	require.NoError(t, fixture.StartChat("first"))
	first := fixture.WaitForResponseOrFail(2 * time.Second)
	require.Len(t, first.Activities, 1)

	require.NoError(t, fixture.StartChat("second"))
	second := fixture.WaitForResponseOrFail(2 * time.Second)
	require.Len(t, second.Activities, 1)
	assert.Equal(t, "readFile", second.Activities[0].Tool)
}

// A turn with no tool calls carries no activities.
func TestChatWithoutToolsHasNoActivities(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	defer fixture.Cleanup()
	fixture.StartAndGetSession()

	fixture.ExpectSimpleMessage("hello", "hi")

	require.NoError(t, fixture.StartChat("hello"))
	response := fixture.WaitForResponseOrFail(2 * time.Second)

	assert.Empty(t, response.Activities)
}
