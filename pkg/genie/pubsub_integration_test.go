package genie

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPubsubIntegration_ManagersReceiveEvents(t *testing.T) {
	// Create managers using Wire DI (should create singletons with shared channels)
	contextManager := ProvideContextManager()
	eventBus := ProvideEventBus()

	// Simulate what Genie core does: publish chat.response events
	// (ContextManager now listens to chat.response events instead of session.interaction)
	chatEvent := events.ChatResponseEvent{
		Message:  "Hello world",
		Response: "Hi there, how can I help?",
		Error:    nil,
	}
	eventBus.Publish("chat.response", chatEvent)

	// Give some time for async event processing
	time.Sleep(100 * time.Millisecond)

	// Test that context manager received the chat.response events
	ctx := context.Background()
	parts, err := contextManager.GetContextParts(ctx)
	require.NoError(t, err)
	chatContext := parts["chat"]

	// Should contain the conversation
	assert.Contains(t, chatContext, "User: Hello world")
	assert.Contains(t, chatContext, "Assistant: Hi there, how can I help?")

	t.Logf("Chat Context: %s", chatContext)
	t.Logf("✅ Context manager integration test passed")
}

func TestContextManager_WithChatResponseEvents(t *testing.T) {
	// Create managers using Wire DI
	contextManager := ProvideContextManager()
	eventBus := ProvideEventBus()

	// Simulate chat response events directly (as they would come from genie core)
	chatEvent1 := events.ChatResponseEvent{
		Message:  "Hello",
		Response: "Hi there!",
		Error:    nil,
	}

	chatEvent2 := events.ChatResponseEvent{
		Message:  "How are you?",
		Response: "I'm doing well!",
		Error:    nil,
	}

	// Publish events
	eventBus.Publish("chat.response", chatEvent1)
	eventBus.Publish("chat.response", chatEvent2)

	// Give time for event propagation
	time.Sleep(100 * time.Millisecond)

	// Test LLM context includes both interactions
	ctx := context.Background()
	parts, err := contextManager.GetContextParts(ctx)
	require.NoError(t, err)
	chatContext := parts["chat"]

	// Should contain both conversations
	assert.Contains(t, chatContext, "User: Hello")
	assert.Contains(t, chatContext, "Assistant: Hi there!")
	assert.Contains(t, chatContext, "User: How are you?")
	assert.Contains(t, chatContext, "Assistant: I'm doing well!")

	// Test clear context
	err = contextManager.ClearContext()
	require.NoError(t, err)

	// Should be empty after clearing
	clearedParts, err := contextManager.GetContextParts(ctx)
	require.NoError(t, err)
	assert.Empty(t, clearedParts["chat"])

	t.Logf("✅ Chat response events test passed")
}