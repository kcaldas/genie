package genie_test

import (
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/genie"
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

// The next turn's prompt context carries the previous turn's Actions
// block: the model is no longer procedurally blind between turns.
func TestNextTurnSeesPreviousActivities(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	defer fixture.Cleanup()
	fixture.StartAndGetSession()

	fixture.ExpectMessage("fix it").
		MockTool("bash").Returns(map[string]any{"exit": 1}).
		RespondWith("fixed")
	fixture.ExpectSimpleMessage("thanks", "welcome")

	require.NoError(t, fixture.StartChat("fix it"))
	fixture.WaitForResponseOrFail(2 * time.Second)

	require.NoError(t, fixture.StartChat("thanks"))
	fixture.WaitForResponseOrFail(2 * time.Second)

	captured := fixture.MockPromptRunner.CapturedData()
	require.Len(t, captured, 2)
	chat := captured[1]["chat"]
	assert.Contains(t, chat, "User: fix it")
	assert.Contains(t, chat, "Assistant Actions:\n- bash → Executed (mocked)")
	assert.Contains(t, chat, "Assistant: fixed")
}

// Ephemeral turns must not leak tool activity into history: activity
// args can carry the very content the ephemeral mode is hiding.
func TestEphemeralTurnRecordsNoActivities(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	defer fixture.Cleanup()
	fixture.StartAndGetSession()

	fixture.ExpectMessage("secret request").
		MockTool("bash").Returns(map[string]any{"exit": 0}).
		RespondWith("done quietly")
	fixture.ExpectSimpleMessage("next", "ok")

	require.NoError(t, fixture.StartChat("secret request", genie.WithEphemeral(genie.EphemeralInput)))
	fixture.WaitForResponseOrFail(2 * time.Second)

	require.NoError(t, fixture.StartChat("next"))
	fixture.WaitForResponseOrFail(2 * time.Second)

	captured := fixture.MockPromptRunner.CapturedData()
	require.Len(t, captured, 2)
	assert.NotContains(t, captured[1]["chat"], "Assistant Actions:")
}
