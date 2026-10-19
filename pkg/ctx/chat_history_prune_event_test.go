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

func TestChatHistory_PublishesContextPrunedEvent(t *testing.T) {
	bus := events.NewEventBus()
	collector := &pruneEventCollector{}
	bus.Subscribe(events.ContextPrunedEvent{}.Topic(), collector.collect)

	chat := NewChatHistory(bus).(*InMemoryChatHistory)
	chat.SetBudgetStrategy(NewSlidingWindowStrategy())
	chat.SetTokenBudget(30)

	for i := 0; i < 20; i++ {
		chat.AddTurn(fmt.Sprintf("question number %d with some padding", i),
			fmt.Sprintf("answer number %d with some padding", i))
	}

	_ = chat.History(context.Background())

	got := collector.snapshot()
	require.Len(t, got, 1, "prune must publish exactly one event")
	assert.Equal(t, "sliding_window", got[0].Strategy)
	assert.Equal(t, 20, got[0].Total)
	assert.Greater(t, got[0].Dropped, 0)
	assert.Equal(t, got[0].Total-got[0].Kept, got[0].Dropped)
	assert.Equal(t, 30, got[0].BudgetTokens)
}

func TestChatHistory_PrunePublishesOncePerOverflow(t *testing.T) {
	bus := events.NewEventBus()
	collector := &pruneEventCollector{}
	bus.Subscribe(events.ContextPrunedEvent{}.Topic(), collector.collect)

	chat := NewChatHistory(bus).(*InMemoryChatHistory)
	chat.SetBudgetStrategy(NewSlidingWindowStrategy())
	chat.SetTokenBudget(30)

	for i := 0; i < 20; i++ {
		chat.AddTurn(fmt.Sprintf("question number %d with some padding", i),
			fmt.Sprintf("answer number %d with some padding", i))
	}

	// The prune is in place: the second read has nothing left to drop.
	_ = chat.History(context.Background())
	_ = chat.History(context.Background())
	assert.Len(t, collector.snapshot(), 1, "an in-place prune must publish once")

	// The next overflow publishes again.
	chat.AddTurn("one more question with padding", "one more answer with padding")
	_ = chat.History(context.Background())
	assert.Len(t, collector.snapshot(), 2)
}

func TestChatHistory_NoPruneEventWithoutDrop(t *testing.T) {
	bus := events.NewEventBus()
	collector := &pruneEventCollector{}
	bus.Subscribe(events.ContextPrunedEvent{}.Topic(), collector.collect)

	chat := NewChatHistory(bus).(*InMemoryChatHistory)
	chat.SetBudgetStrategy(NewSlidingWindowStrategy())
	chat.SetTokenBudget(100000)

	chat.AddTurn("short question", "short answer")

	_ = chat.History(context.Background())
	assert.Empty(t, collector.snapshot())
}
