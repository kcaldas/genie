package ctx

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/kcaldas/genie/pkg/events"
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
		SessionID: "session-1",
		Message:   "Hello",
		Response:  "Hi there!",
		Error:     nil,
	}
	eventBus.Publish("chat.response", chatEvent)

	// Give time for event processing
	time.Sleep(10 * time.Millisecond)

	// Get context should contain the formatted conversation
	result := manager.GetContext()
	assert.Contains(t, result, "User: Hello")
	assert.Contains(t, result, "Genie: Hi there!")
}

func TestChatCtxManager_MultipleMessagePairs(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	// Publish multiple chat response events
	chatEvent1 := events.ChatResponseEvent{
		SessionID: "session-1",
		Message:   "First question",
		Response:  "First answer",
		Error:     nil,
	}
	chatEvent2 := events.ChatResponseEvent{
		SessionID: "session-1",
		Message:   "Second question",
		Response:  "Second answer",
		Error:     nil,
	}
	
	eventBus.Publish("chat.response", chatEvent1)
	eventBus.Publish("chat.response", chatEvent2)

	// Give time for event processing
	time.Sleep(10 * time.Millisecond)

	// Get context should contain both conversations in order with formatting
	result := manager.GetContext()
	assert.Contains(t, result, "User: First question")
	assert.Contains(t, result, "Genie: First answer")
	assert.Contains(t, result, "User: Second question")
	assert.Contains(t, result, "Genie: Second answer")
	
	// Verify the order - first question should come before second question
	firstIndex := strings.Index(result, "User: First question")
	secondIndex := strings.Index(result, "User: Second question")
	assert.True(t, firstIndex < secondIndex, "Messages should be in chronological order")
}

func TestChatCtxManager_ClearContext(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	// Add some context first
	chatEvent := events.ChatResponseEvent{
		SessionID: "session-1",
		Message:   "Hello",
		Response:  "Hi there!",
		Error:     nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)

	// Verify context exists
	result := manager.GetContext()
	assert.Contains(t, result, "User: Hello")

	// Clear context
	err := manager.ClearContext()
	assert.NoError(t, err)

	// Verify context is cleared
	result = manager.GetContext()
	assert.Empty(t, result)
}

func TestChatCtxManager_EmptyContext(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	// Get context from empty manager
	result := manager.GetContext()
	assert.Empty(t, result)
}

func TestChatCtxManager_FormatsWithGeniePrefix(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	// Publish a chat response event
	chatEvent := events.ChatResponseEvent{
		SessionID: "session-1",
		Message:   "What's your name?",
		Response:  "I'm Genie, your AI assistant!",
		Error:     nil,
	}
	eventBus.Publish("chat.response", chatEvent)
	time.Sleep(10 * time.Millisecond)

	// Verify formatting uses "Genie:" prefix for assistant responses
	result := manager.GetContext()
	expected := "User: What's your name?\nGenie: I'm Genie, your AI assistant!"
	assert.Equal(t, expected, result)
}