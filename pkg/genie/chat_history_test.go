package genie_test

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/genie/genietest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// History must be recorded by the time the response event is observable:
// it is correctness state for the next turn and must not depend on
// asynchronous event delivery.
func TestChatRecordsHistoryBeforeResponseEvent(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	fixture.StartAndGetSession()

	fixture.ExpectSimpleMessage("what is genie?", "a coding assistant")
	require.NoError(t, fixture.StartChat("what is genie?"))
	fixture.WaitForResponseOrFail(2 * time.Second)

	contextMap, err := fixture.Genie.GetContext(context.Background())
	require.NoError(t, err)
	assert.Contains(t, contextMap["chat"], "what is genie?")
	assert.Contains(t, contextMap["chat"], "a coding assistant")
}

// A turn that fails must leave no trace in history.
func TestChatDoesNotRecordFailedTurns(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	fixture.StartAndGetSession()

	// No expectation configured: the prompt runner errors for this message.
	require.NoError(t, fixture.StartChat("unanswerable question"))
	response := fixture.WaitForResponseOrFail(2 * time.Second)
	require.Error(t, response.Error)

	contextMap, err := fixture.Genie.GetContext(context.Background())
	require.NoError(t, err)
	assert.NotContains(t, contextMap["chat"], "unanswerable question",
		"failed turns must not be recorded in history")
}
