package di

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/kcaldas/genie/pkg/events"
)

func TestPubsubIntegration_ManagersReceiveEvents(t *testing.T) {
	// Create managers using Wire DI (should create singletons with shared channels)
	contextManager := ProvideContextManager()
	eventBus := ProvideEventBus()

	// Simulate what Genie core does: publish chat.response events
	// (ContextManager now listens to chat.response events instead of session.interaction)
	chatEvent := events.ChatResponseEvent{
		SessionID: "integration-test-session",
		Message:   "Hello world",
		Response:  "Hi there, how can I help?",
		Error:     nil,
	}
	eventBus.Publish("chat.response", chatEvent)

	// Give some time for async event processing
	time.Sleep(100 * time.Millisecond)

	// Test that context manager received the chat.response events
	ctx := context.Background()
	llmContext, err := contextManager.GetLLMContext(ctx, "integration-test-session")
	require.NoError(t, err)
	
	// Should contain the conversation
	assert.Contains(t, llmContext, "User: Hello world")
	assert.Contains(t, llmContext, "Assistant: Hi there, how can I help?")

	t.Logf("LLM Context: %s", llmContext)
	t.Logf("✅ Context manager integration test passed")
}

func TestContextManager_WithChatResponseEvents(t *testing.T) {
	// Create managers using Wire DI
	contextManager := ProvideContextManager()
	eventBus := ProvideEventBus()
	
	// Simulate chat response events directly (as they would come from genie core)
	chatEvent1 := events.ChatResponseEvent{
		SessionID: "chat-test-session",
		Message:   "Hello",
		Response:  "Hi there!",
		Error:     nil,
	}
	
	chatEvent2 := events.ChatResponseEvent{
		SessionID: "chat-test-session", 
		Message:   "How are you?",
		Response:  "I'm doing well!",
		Error:     nil,
	}
	
	// Publish events
	eventBus.Publish("chat.response", chatEvent1)
	eventBus.Publish("chat.response", chatEvent2)
	
	// Give time for event propagation
	time.Sleep(100 * time.Millisecond)
	
	// Test LLM context includes both interactions
	ctx := context.Background()
	llmContext, err := contextManager.GetLLMContext(ctx, "chat-test-session")
	require.NoError(t, err)
	
	// Should contain both conversations
	assert.Contains(t, llmContext, "User: Hello")
	assert.Contains(t, llmContext, "Assistant: Hi there!")
	assert.Contains(t, llmContext, "User: How are you?")
	assert.Contains(t, llmContext, "Assistant: I'm doing well!")
	
	// Test clear context
	err = contextManager.ClearContext("chat-test-session")
	require.NoError(t, err)
	
	// Should be empty after clearing
	clearedContext, err := contextManager.GetLLMContext(ctx, "chat-test-session")
	require.NoError(t, err)
	assert.Empty(t, clearedContext)
	
	t.Logf("✅ Chat response events test passed")
}
