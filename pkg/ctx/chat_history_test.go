package ctx

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
)

func TestChatHistory_CanBeCreated(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatHistory(eventBus)

	assert.NotNil(t, manager)
}

func TestChatHistory_RecordsTurns(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatHistory(eventBus)

	// Record a completed exchange synchronously
	manager.AddTurn("Hello", "Hi there!")

	// The rendered history should contain the conversation
	part := FormatHistory(manager.History(context.Background()))
	assert.Contains(t, part, "User: Hello")
	assert.Contains(t, part, "Assistant: Hi there!")
}

func TestChatHistory_MultipleMessagePairs(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatHistory(eventBus)

	// Record multiple exchanges
	manager.AddTurn("First question", "First answer")
	manager.AddTurn("Second question", "Second answer")

	// The rendered history should contain both conversations in order
	part := FormatHistory(manager.History(context.Background()))
	assert.Contains(t, part, "User: First question")
	assert.Contains(t, part, "Assistant: First answer")
	assert.Contains(t, part, "User: Second question")
	assert.Contains(t, part, "Assistant: Second answer")
}

func TestChatHistory_ClearContext(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatHistory(eventBus)

	// Add some context first
	manager.AddTurn("Hello", "Hi there!")

	// Verify context exists
	part := FormatHistory(manager.History(context.Background()))
	assert.Contains(t, part, "User: Hello")

	// Clear context
	manager.Clear()

	// Verify context is cleared
	part2 := FormatHistory(manager.History(context.Background()))
	assert.Empty(t, part2)
}

func TestChatHistory_EmptyContext(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatHistory(eventBus)

	// Render an empty history
	part := FormatHistory(manager.History(context.Background()))
	assert.Empty(t, part)
}

func TestChatHistory_FormatsWithGeniePrefix(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatHistory(eventBus)

	// Record an exchange
	manager.AddTurn("What's your name?", "I'm Genie, your AI assistant!")

	// Verify formatting uses "Assistant:" prefix for assistant responses
	part := FormatHistory(manager.History(context.Background()))
	expected := "User: What's your name?\nAssistant: I'm Genie, your AI assistant!"
	assert.Equal(t, expected, part)
}

func TestChatHistory_SeedHistory(t *testing.T) {
	eventBus := events.NewEventBus()
	manager := NewChatHistory(eventBus)

	manager.SeedHistory([]Message{
		{User: "Hello", Assistant: "Hi!"},
		{User: "How are you?", Assistant: "Doing great."},
	})

	part := FormatHistory(manager.History(context.Background()))
	assert.Contains(t, part, "User: Hello")
	assert.Contains(t, part, "Assistant: Hi!")
	assert.Contains(t, part, "User: How are you?")
	assert.Contains(t, part, "Assistant: Doing great.")
}
