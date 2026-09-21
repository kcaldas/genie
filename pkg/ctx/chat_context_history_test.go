package ctx

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newHistoryProvider(budget int) *InMemoryChatContextPartProvider {
	p := NewChatCtxManager(nil).(*InMemoryChatContextPartProvider)
	p.SetBudgetStrategy(NewSlidingWindowStrategy())
	p.SetTokenBudget(budget)
	return p
}

func TestChatProvider_HistoryReturnsKeptTurnsInOrder(t *testing.T) {
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

func TestChatProvider_HistoryIsACopy(t *testing.T) {
	p := newHistoryProvider(100000)
	p.AddTurn("one", "uno")

	history := p.History(context.Background())
	history[0].User = "mutated"

	assert.Equal(t, "one", p.History(context.Background())[0].User)
}

func TestChatProvider_GetPartRendersExactlyTheHistory(t *testing.T) {
	p := newHistoryProvider(100000)
	p.AddTurn("one", "uno")
	p.AddTurn("two", "dos", events.ToolActivity{Tool: "bash", Args: "ls"})

	part, err := p.GetPart(context.Background())
	require.NoError(t, err)

	var want []string
	for _, m := range p.History(context.Background()) {
		want = append(want, formatMessageForContext(m))
	}
	assert.Equal(t, strings.Join(want, "\n"), part.Content)
}

// Each turn below is ~25 estimated tokens (100 bytes). A budget of 200
// tokens holds 8 of them; the low-water mark keeps the prune from
// recurring on every turn once the history is full.
func addFixedSizeTurns(p *InMemoryChatContextPartProvider, from, to int) {
	for i := from; i < to; i++ {
		user := fmt.Sprintf("u%02d", i)
		p.AddTurn(user, strings.Repeat("x", 100-len("User: \nAssistant: ")-len(user)))
	}
}

func TestChatProvider_PrunesInBlocksNotOneMessagePerTurn(t *testing.T) {
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

func TestChatProvider_PruneDropsToLowWaterMark(t *testing.T) {
	p := newHistoryProvider(200)
	addFixedSizeTurns(p, 0, 12)

	kept := p.History(context.Background())

	// 60% of a 200-token budget is 120 tokens: four 25-token turns.
	assert.Len(t, kept, 4)
	assert.Equal(t, "u08", kept[0].User)
	assert.Equal(t, "u11", kept[3].User)
}

func TestChatProvider_PruneIsPermanent(t *testing.T) {
	p := newHistoryProvider(200)
	addFixedSizeTurns(p, 0, 12)
	_ = p.History(context.Background())

	p.SetTokenBudget(100000)

	assert.Len(t, p.History(context.Background()), 4, "pruned turns must not come back when the budget grows")
}

func TestChatProvider_NoBudgetKeepsEverything(t *testing.T) {
	p := NewChatCtxManager(nil).(*InMemoryChatContextPartProvider)
	addFixedSizeTurns(p, 0, 50)

	assert.Len(t, p.History(context.Background()), 50)
}

func TestInMemoryManager_ChatHistoryComesFromTheChatProvider(t *testing.T) {
	registry := NewContextPartProviderRegistry()
	chat := NewChatCtxManager(nil)
	registry.Register(chat, 1)
	manager := NewContextManager(registry)
	manager.RecordChatTurn("hello", "hi")

	history, err := manager.ChatHistory(context.Background())

	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "hello", history[0].User)
}

func TestChatProvider_PruneKeepsNewestTurnWhenLowWaterIsTooSmall(t *testing.T) {
	// Each turn is ~25 tokens; a 30-token budget has an 18-token low-water
	// mark that no single turn fits under. The newest turn must survive.
	p := newHistoryProvider(30)
	addFixedSizeTurns(p, 0, 3)

	kept := p.History(context.Background())

	require.Len(t, kept, 1)
	assert.Equal(t, "u02", kept[0].User)
}
