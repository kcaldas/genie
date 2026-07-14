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
	chatManager := ctx.NewChatCtxManager(eventBus)
	chatManager.SetBudgetStrategy(ctx.NewSlidingWindowStrategy())
	registry.Register(chatManager, 0.7)

	return ctx.NewContextManager(registry), eventBus
}

// History is recorded synchronously via RecordChatTurn (what the core
// does after each successful turn) — no event delivery, no sleeps.
func TestContextManager_RecordsChatTurns(t *testing.T) {
	contextManager, _ := newTestContextManager(t)

	contextManager.RecordChatTurn("Hello world", "Hi there, how can I help?")

	parts, err := contextManager.GetContextParts(context.Background())
	require.NoError(t, err)
	chatContext := parts["chat"]

	assert.Contains(t, chatContext, "User: Hello world")
	assert.Contains(t, chatContext, "Assistant: Hi there, how can I help?")
}

func TestContextManager_MultipleTurnsAndClear(t *testing.T) {
	contextManager, _ := newTestContextManager(t)

	contextManager.RecordChatTurn("Hello", "Hi there!")
	contextManager.RecordChatTurn("How are you?", "I'm doing well!")

	parts, err := contextManager.GetContextParts(context.Background())
	require.NoError(t, err)
	chatContext := parts["chat"]

	assert.Contains(t, chatContext, "User: Hello")
	assert.Contains(t, chatContext, "Assistant: Hi there!")
	assert.Contains(t, chatContext, "User: How are you?")
	assert.Contains(t, chatContext, "Assistant: I'm doing well!")

	// Test clear context
	err = contextManager.ClearContext()
	require.NoError(t, err)

	// Should be empty after clearing
	clearedParts, err := contextManager.GetContextParts(context.Background())
	require.NoError(t, err)
	assert.Empty(t, clearedParts["chat"])
}
