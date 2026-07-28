package ctx

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pruneEventCollector struct {
	mu     sync.Mutex
	events []events.ContextPrunedEvent
}

func (c *pruneEventCollector) collect(event interface{}) {
	if e, ok := event.(events.ContextPrunedEvent); ok {
		c.mu.Lock()
		c.events = append(c.events, e)
		c.mu.Unlock()
	}
}

func (c *pruneEventCollector) snapshot() []events.ContextPrunedEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]events.ContextPrunedEvent(nil), c.events...)
}

func TestChatProvider_PublishesContextPrunedEvent(t *testing.T) {
	bus := events.NewEventBus()
	collector := &pruneEventCollector{}
	bus.Subscribe(events.ContextPrunedEvent{}.Topic(), collector.collect)

	provider := NewChatCtxManager(bus).(*InMemoryChatContextPartProvider)
	provider.SetBudgetStrategy(NewSlidingWindowStrategy())
	provider.SetTokenBudget(30)

	for i := 0; i < 20; i++ {
		provider.AddTurn(fmt.Sprintf("question number %d with some padding", i),
			fmt.Sprintf("answer number %d with some padding", i))
	}

	_, err := provider.GetPart(context.Background())
	require.NoError(t, err)

	got := collector.snapshot()
	require.Len(t, got, 1, "prune must publish exactly one event")
	assert.Equal(t, "sliding_window", got[0].Strategy)
	assert.Equal(t, 20, got[0].Total)
	assert.Greater(t, got[0].Dropped, 0)
	assert.Equal(t, got[0].Total-got[0].Kept, got[0].Dropped)
	assert.Equal(t, 30, got[0].BudgetTokens)
}

func TestChatProvider_PruneEventDedupedAcrossReads(t *testing.T) {
	bus := events.NewEventBus()
	collector := &pruneEventCollector{}
	bus.Subscribe(events.ContextPrunedEvent{}.Topic(), collector.collect)

	provider := NewChatCtxManager(bus).(*InMemoryChatContextPartProvider)
	provider.SetBudgetStrategy(NewSlidingWindowStrategy())
	provider.SetTokenBudget(30)

	for i := 0; i < 20; i++ {
		provider.AddTurn(fmt.Sprintf("question number %d with some padding", i),
			fmt.Sprintf("answer number %d with some padding", i))
	}

	// Prune recomputes per read; the same outcome must publish only once.
	_, err := provider.GetPart(context.Background())
	require.NoError(t, err)
	_, err = provider.GetPart(context.Background())
	require.NoError(t, err)
	assert.Len(t, collector.snapshot(), 1, "identical prune outcome must be deduped")

	// A new turn changes the prune outcome: a fresh event is published.
	provider.AddTurn("one more question with padding", "one more answer with padding")
	_, err = provider.GetPart(context.Background())
	require.NoError(t, err)
	assert.Len(t, collector.snapshot(), 2)
}

func TestChatProvider_NoPruneEventWithoutDrop(t *testing.T) {
	bus := events.NewEventBus()
	collector := &pruneEventCollector{}
	bus.Subscribe(events.ContextPrunedEvent{}.Topic(), collector.collect)

	provider := NewChatCtxManager(bus).(*InMemoryChatContextPartProvider)
	provider.SetBudgetStrategy(NewSlidingWindowStrategy())
	provider.SetTokenBudget(100000)

	provider.AddTurn("short question", "short answer")

	_, err := provider.GetPart(context.Background())
	require.NoError(t, err)
	assert.Empty(t, collector.snapshot())
}
