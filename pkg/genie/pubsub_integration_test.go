package genie

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/ctx"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestContextManager creates a context manager and event bus that share
// the same bus instance — the exact pattern enforced by the per-instance DI fix.
func newTestContextManager(t *testing.T) (ctx.ContextManager, events.EventBus) {
	t.Helper()

	eventBus := events.NewEventBus()

	registry := ctx.NewContextPartProviderRegistry()
	chatManager := ctx.NewChatHistory(eventBus)
	chatManager.SetBudgetStrategy(ctx.NewSlidingWindowStrategy())
	registry.RegisterChat(chatManager, 0.7)

	return ctx.NewContextManager(registry), eventBus
}

// History is recorded synchronously via RecordChatTurn (what the core
// does after each successful turn) — no event delivery, no sleeps.
func TestContextManager_RecordsChatTurns(t *testing.T) {
	contextManager, _ := newTestContextManager(t)

	contextManager.RecordChatTurn("Hello world", "Hi there, how can I help?")

	history, err := contextManager.ChatHistory(context.Background())
	require.NoError(t, err)
	chatContext := ctx.FormatHistory(history)

	assert.Contains(t, chatContext, "User: Hello world")
	assert.Contains(t, chatContext, "Assistant: Hi there, how can I help?")
}

func TestContextManager_MultipleTurnsAndClear(t *testing.T) {
	contextManager, _ := newTestContextManager(t)

	contextManager.RecordChatTurn("Hello", "Hi there!")
	contextManager.RecordChatTurn("How are you?", "I'm doing well!")

	history, err := contextManager.ChatHistory(context.Background())
	require.NoError(t, err)
	chatContext := ctx.FormatHistory(history)

	assert.Contains(t, chatContext, "User: Hello")
	assert.Contains(t, chatContext, "Assistant: Hi there!")
	assert.Contains(t, chatContext, "User: How are you?")
	assert.Contains(t, chatContext, "Assistant: I'm doing well!")

	// Test clear context
	err = contextManager.ClearContext()
	require.NoError(t, err)

	// Should be empty after clearing
	cleared, err := contextManager.ChatHistory(context.Background())
	require.NoError(t, err)
	assert.Empty(t, cleared)
}
