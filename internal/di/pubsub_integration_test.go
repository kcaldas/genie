package di

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPubsubIntegration_ManagersReceiveEvents(t *testing.T) {
	// Create separate channels for managers
	historyCh := ProvideHistoryChannel()
	contextCh := ProvideContextChannel()

	// Create managers using the provider functions
	contextManager := ProvideContextManager(contextCh)
	historyManager := ProvideHistoryManager(historyCh)

	// Create session manager
	sessionManager := ProvideSessionManager(historyCh, contextCh)
	session, err := sessionManager.CreateSession("integration-test-session")
	require.NoError(t, err)

	// Add an interaction (should trigger pubsub events)
	t.Logf("Session type: %T", session)
	err = session.AddInteraction("Hello world", "Hi there, how can I help?")
	require.NoError(t, err)
	t.Logf("AddInteraction completed")

	// Give some time for async event processing
	time.Sleep(100 * time.Millisecond)

	// No need to track events - we'll check the managers directly

	// Debug: Let's see what's actually in the context manager
	contextData, err := contextManager.GetContext("integration-test-session")
	if err != nil {
		t.Logf("Error getting context: %v", err)

		// Let's try a different session ID to see if any data exists
		_, err2 := contextManager.GetContext("different-session")
		t.Logf("Different session error: %v", err2)

		// Let's see if we can add data directly to verify the manager works
		directErr := contextManager.AddInteraction("direct-test", "direct-user", "direct-response")
		t.Logf("Direct add error: %v", directErr)

		if directErr == nil {
			directData, directGetErr := contextManager.GetContext("direct-test")
			t.Logf("Direct data: %v, error: %v", directData, directGetErr)
		}

		// Fail the test here since we couldn't get the expected data
		t.Fatalf("Context manager didn't receive events: %v", err)
	} else {
		t.Logf("Context data: %v", contextData)
		assert.Len(t, contextData, 2)
		assert.Equal(t, "Hello world", contextData[0])
		assert.Equal(t, "Hi there, how can I help?", contextData[1])
	}

	historyData, err := historyManager.GetHistory("integration-test-session")
	require.NoError(t, err)
	assert.Len(t, historyData, 2)
	assert.Equal(t, "Hello world", historyData[0])
	assert.Equal(t, "Hi there, how can I help?", historyData[1])
}
