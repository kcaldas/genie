package ctx

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
)

func TestChatCtxManager_CanBeCreated(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	assert.NotNil(t, manager)
}

func TestChatCtxManager_RecordsTurns(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	// Record a completed exchange synchronously
	manager.AddTurn("Hello", "Hi there!")

	// Get context should contain the formatted conversation
	part, err := manager.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "chat", part.Key)
	assert.Contains(t, part.Content, "User: Hello")
	assert.Contains(t, part.Content, "Assistant: Hi there!")
}

func TestChatCtxManager_MultipleMessagePairs(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	// Record multiple exchanges
	manager.AddTurn("First question", "First answer")
	manager.AddTurn("Second question", "Second answer")

	// Get context should contain both conversations in order
	part, err := manager.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "chat", part.Key)
	assert.Contains(t, part.Content, "User: First question")
	assert.Contains(t, part.Content, "Assistant: First answer")
	assert.Contains(t, part.Content, "User: Second question")
	assert.Contains(t, part.Content, "Assistant: Second answer")
}

func TestChatCtxManager_ClearContext(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	// Add some context first
	manager.AddTurn("Hello", "Hi there!")

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

	// Record an exchange
	manager.AddTurn("What's your name?", "I'm Genie, your AI assistant!")

	// Verify formatting uses "Assistant:" prefix for assistant responses
	part, err := manager.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "chat", part.Key)
	expected := "User: What's your name?\nAssistant: I'm Genie, your AI assistant!"
	assert.Equal(t, expected, part.Content)
}

func TestChatCtxManager_SeedHistory(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatCtxManager(eventBus)

	manager.SeedHistory([]Message{
		{User: "Hello", Assistant: "Hi!"},
		{User: "How are you?", Assistant: "Doing great."},
	})

	part, err := manager.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Contains(t, part.Content, "User: Hello")
	assert.Contains(t, part.Content, "Assistant: Hi!")
	assert.Contains(t, part.Content, "User: How are you?")
	assert.Contains(t, part.Content, "Assistant: Doing great.")
}
