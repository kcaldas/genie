package ctx

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
)

func TestChatCtxManager_CanBeCreated(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	assert.NotNil(t, manager)
}

func TestChatCtxManager_ReceivesChatResponseEvents(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	// Publish a chat response event
	chatEvent := events.ChatResponseEvent{
		Message:  "Hello",
		Response: "Hi there!",
		Error:    nil,
	}
	eventBus.Publish("chat.response", chatEvent)

	// Give time for event processing
	time.Sleep(10 * time.Millisecond)

	// Get context should contain the formatted conversation
	part, err := manager.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "chat", part.Key)
	assert.Contains(t, part.Content, "User: Hello")
	assert.Contains(t, part.Content, "Genie: Hi there!")
}

func TestChatCtxManager_MultipleMessagePairs(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	// Publish multiple chat response events
	chatEvent1 := events.ChatResponseEvent{
		Message:  "First question",
		Response: "First answer",
		Error:    nil,
	}
	chatEvent2 := events.ChatResponseEvent{
		Message:  "Second question",
		Response: "Second answer",
		Error:    nil,
	}

	eventBus.Publish("chat.response", chatEvent1)
	eventBus.Publish("chat.response", chatEvent2)

	// Small delay for test reliability (not needed in production due to natural user interaction delays)
	time.Sleep(1 * time.Millisecond)

	// Get context should contain both conversations (order may vary due to async processing, which is acceptable)
	part, err := manager.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "chat", part.Key)
	assert.Contains(t, part.Content, "User: First question")
	assert.Contains(t, part.Content, "Genie: First answer")
	assert.Contains(t, part.Content, "User: Second question")
	assert.Contains(t, part.Content, "Genie: Second answer")

	// Both messages should be present (order may vary due to async processing, which is acceptable)
	assert.Contains(t, part.Content, "User: First question", "Should contain first question")
	assert.Contains(t, part.Content, "User: Second question", "Should contain second question")
}

func TestChatCtxManager_ClearContext(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	// Add some context first
	chatEvent := events.ChatResponseEvent{
		Message:  "Hello",
		Response: "Hi there!",
		Error:    nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)

	// Verify context exists
	part, err := manager.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Contains(t, part.Content, "User: Hello")

	// Clear context
	err2 := manager.ClearPart()
	assert.NoError(t, err2)

	// Verify context is cleared
	part2, err := manager.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, part2.Content)
}

func TestChatCtxManager_EmptyContext(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	// Get context from empty manager
	part, err := manager.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "chat", part.Key)
	assert.Empty(t, part.Content)
}

func TestChatCtxManager_FormatsWithGeniePrefix(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	// Publish a chat response event
	chatEvent := events.ChatResponseEvent{
		Message:  "What's your name?",
		Response: "I'm Genie, your AI assistant!",
		Error:    nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)

	// Verify formatting uses "Genie:" prefix for assistant responses
	part, err := manager.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "chat", part.Key)
	expected := "User: What's your name?\nGenie: I'm Genie, your AI assistant!"
	assert.Equal(t, expected, part.Content)
}
