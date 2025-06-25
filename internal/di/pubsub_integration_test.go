package di

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPubsubIntegration_ManagersReceiveEvents(t *testing.T) {
	// Create managers using Wire DI (should create singletons with shared channels)
	contextManager := ProvideContextManager()
	sessionManager := ProvideSessionManager()
	session, err := sessionManager.CreateSession("integration-test-session", ".")
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
	ctx := context.Background()
	contextData, err := contextManager.GetContext(ctx, "integration-test-session")
	if err != nil {
		t.Logf("Error getting context: %v", err)

		// Let's try a different session ID to see if any data exists
		_, err2 := contextManager.GetContext(ctx, "different-session")
		t.Logf("Different session error: %v", err2)

		// Let's see if we can add data directly to verify the manager works
		directErr := contextManager.AddInteraction("direct-test", "direct-user", "direct-response")
		t.Logf("Direct add error: %v", directErr)

		if directErr == nil {
			directData, directGetErr := contextManager.GetContext(ctx, "direct-test")
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

	// Context manager serves the same purpose as history manager
	// Test that context manager received the events properly
	t.Logf("✅ Context manager integration test passed")
}

func TestContextManager_ConversationContext(t *testing.T) {
	// Create managers using Wire DI
	contextManager := ProvideContextManager()
	sessionManager := ProvideSessionManager()
	
	// Create a test session
	session, err := sessionManager.CreateSession("context-test-session", ".")
	require.NoError(t, err)
	
	// Add first interaction
	err = session.AddInteraction("Hello", "Hi there!")
	require.NoError(t, err)
	
	// Give time for event propagation
	time.Sleep(100 * time.Millisecond)
	
	// Test conversation context with one interaction
	ctx := context.Background()
	context1, err := contextManager.GetConversationContext(ctx, "context-test-session", 5)
	require.NoError(t, err)
	
	expected1 := "User: Hello\nAssistant: Hi there!"
	assert.Equal(t, expected1, context1)
	
	// Add second interaction
	err = session.AddInteraction("How are you?", "I'm doing well!")
	require.NoError(t, err)
	
	// Give time for event propagation  
	time.Sleep(100 * time.Millisecond)
	
	// Test conversation context with two interactions
	context2, err := contextManager.GetConversationContext(ctx, "context-test-session", 5)
	require.NoError(t, err)
	
	expected2 := "User: Hello\nAssistant: Hi there!\nUser: How are you?\nAssistant: I'm doing well!"
	assert.Equal(t, expected2, context2)
	
	// Test with limited pairs
	contextLimited, err := contextManager.GetConversationContext(ctx, "context-test-session", 1)
	require.NoError(t, err)
	
	expectedLimited := "User: How are you?\nAssistant: I'm doing well!"
	assert.Equal(t, expectedLimited, contextLimited)
	
	t.Logf("✅ Conversation context test passed")
}
