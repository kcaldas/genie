package ctx

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newHistoryProvider(budget int) *InMemoryChatHistory {
	p := NewChatHistory(nil).(*InMemoryChatHistory)
	p.SetBudgetStrategy(NewSlidingWindowStrategy())
	p.SetTokenBudget(budget)
	return p
}

func TestChatHistory_HistoryReturnsKeptTurnsInOrder(t *testing.T) {
	p := newHistoryProvider(100000)
	p.AddTurn("one", "uno")
	p.AddTurn("two", "dos", events.ToolActivity{Tool: "readFile", Args: "a.md", Summary: "12 lines"})
	p.AddTurn("three", "tres")

	history := p.History(context.Background())

	require.Len(t, history, 3)
	assert.Equal(t, "one", history[0].User)
	assert.Equal(t, "dos", history[1].Assistant)
	assert.Equal(t, "readFile", history[1].Activities[0].Tool)
	assert.Equal(t, "three", history[2].User)
}

func TestChatHistory_HistoryIsACopy(t *testing.T) {
	p := newHistoryProvider(100000)
	p.AddTurn("one", "uno")

	history := p.History(context.Background())
	history[0].User = "mutated"

	assert.Equal(t, "one", p.History(context.Background())[0].User)
}

func TestFormatHistory_RendersEveryTurnWithFormatMessage(t *testing.T) {
	p := newHistoryProvider(100000)
	p.AddTurn("one", "uno")
	p.AddTurn("two", "dos", events.ToolActivity{Tool: "bash", Args: "ls"})
	history := p.History(context.Background())

	var want []string
	for _, m := range history {
		want = append(want, FormatMessage(m))
	}
	assert.Equal(t, strings.Join(want, "\n"), FormatHistory(history))
}

func TestFormatHistory_SkipsTurnsThatRenderEmpty(t *testing.T) {
	assert.Equal(t, "User: a\nAssistant: b", FormatHistory([]Message{{}, {User: "a"}, {}, {Assistant: "b"}}))
}

// Each turn below is ~25 estimated tokens (100 bytes). A budget of 200
// tokens holds 8 of them; the low-water mark keeps the prune from
// recurring on every turn once the history is full.
func addFixedSizeTurns(p *InMemoryChatHistory, from, to int) {
	for i := from; i < to; i++ {
		user := fmt.Sprintf("u%02d", i)
		p.AddTurn(user, strings.Repeat("x", 100-len("User: \nAssistant: ")-len(user)))
	}
}

func TestChatHistory_PrunesInBlocksNotOneMessagePerTurn(t *testing.T) {
	p := newHistoryProvider(200)
	addFixedSizeTurns(p, 0, 9) // 9 turns > budget of 8

	first := p.History(context.Background())
	require.NotEmpty(t, first)
	assert.Less(t, len(first), 8, "prune must cut below the budget, not just to it")
	oldest := first[0].User

	// Adding turns while the history fits again must not move the window start.
	addFixedSizeTurns(p, 9, 11)
	second := p.History(context.Background())
	assert.Equal(t, oldest, second[0].User, "window start moved although history still fit the budget")
	assert.Len(t, second, len(first)+2)
}

func TestChatHistory_PruneDropsToLowWaterMark(t *testing.T) {
	p := newHistoryProvider(200)
	addFixedSizeTurns(p, 0, 12)

	kept := p.History(context.Background())

	// 60% of a 200-token budget is 120 tokens: four 25-token turns.
	assert.Len(t, kept, 4)
	assert.Equal(t, "u08", kept[0].User)
	assert.Equal(t, "u11", kept[3].User)
}

func TestChatHistory_PruneIsPermanent(t *testing.T) {
	p := newHistoryProvider(200)
	addFixedSizeTurns(p, 0, 12)
	_ = p.History(context.Background())

	p.SetTokenBudget(100000)

	assert.Len(t, p.History(context.Background()), 4, "pruned turns must not come back when the budget grows")
}

func TestChatHistory_NoBudgetKeepsEverything(t *testing.T) {
	p := NewChatHistory(nil).(*InMemoryChatHistory)
	addFixedSizeTurns(p, 0, 50)

	assert.Len(t, p.History(context.Background()), 50)
}

func TestInMemoryManager_ChatHistoryComesFromTheRegisteredChat(t *testing.T) {
	registry := NewContextPartProviderRegistry()
	chat := NewChatHistory(nil)
	registry.RegisterChat(chat, 1)
	manager := NewContextManager(registry)
	manager.RecordChatTurn("hello", "hi")

	history, err := manager.ChatHistory(context.Background())

	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "hello", history[0].User)
}

func TestChatHistory_PruneKeepsNewestTurnWhenLowWaterIsTooSmall(t *testing.T) {
	// Each turn is ~25 tokens; a 30-token budget has an 18-token low-water
	// mark that no single turn fits under. The newest turn must survive.
	p := newHistoryProvider(30)
	addFixedSizeTurns(p, 0, 3)

	kept := p.History(context.Background())

	require.Len(t, kept, 1)
	assert.Equal(t, "u02", kept[0].User)
}

func TestChatHistory_PublishesEveryPruneEvenWithIdenticalCounts(t *testing.T) {
	bus := events.NewEventBus()
	var mu sync.Mutex
	var got []events.ContextPrunedEvent
	bus.Subscribe(events.ContextPrunedEvent{}.Topic(), func(e interface{}) {
		if ev, ok := e.(events.ContextPrunedEvent); ok {
			mu.Lock()
			got = append(got, ev)
			mu.Unlock()
		}
	})
	p := NewChatHistory(bus).(*InMemoryChatHistory)
	p.SetBudgetStrategy(NewSlidingWindowStrategy())
	p.SetTokenBudget(200)

	addFixedSizeTurns(p, 0, 12)
	_ = p.History(context.Background()) // 12 -> 4
	addFixedSizeTurns(p, 12, 20)
	_ = p.History(context.Background()) // 12 -> 4 again, same counts

	require.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(got) == 2 }, time.Second, 10*time.Millisecond,
		"each real prune must publish, however similar its counts")
}
