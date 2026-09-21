package ctx

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/events"
)

// Message represents a user/assistant conversation pair, with the tool
// activities that produced the assistant side.
type Message struct {
	User       string
	Activities []events.ToolActivity
	Assistant  string
}

// ChatContextPartProvider manages chat context for a single session
type ChatContextPartProvider interface {
	ContextPartProvider
	SeedHistory(history []Message)
	SetBudgetStrategy(strategy CollectionBudgetStrategy[Message])
	// AddTurn records one completed exchange, optionally with the tool
	// activities that produced it. Empty user or assistant sides are
	// allowed (ephemeral modes); a fully empty turn is ignored.
	AddTurn(user, assistant string, activities ...events.ToolActivity)
}

// InMemoryChatContextPartProvider implements ChatCtxManager with in-memory storage
type InMemoryChatContextPartProvider struct {
	mu             sync.RWMutex
	messages       []Message
	budgetStrategy CollectionBudgetStrategy[Message]
	tokenBudget    int

	publisher events.Publisher
}

// NewChatCtxManager creates a new chat context manager.
//
// History is recorded synchronously by the core via AddTurn after each
// successful turn — never from bus events, whose delivery order and
// timing must not influence what the model remembers.
//
// The event bus is used for outbound observability only: prune outcomes
// are published as ContextPrunedEvent for observers such as the session
// recorder.
func NewChatCtxManager(eventBus events.EventBus) ChatContextPartProvider {
	return &InMemoryChatContextPartProvider{
		messages:  make([]Message, 0),
		publisher: eventBus,
	}
}

// AddTurn records one completed exchange in conversation history.
func (p *InMemoryChatContextPartProvider) AddTurn(user, assistant string, activities ...events.ToolActivity) {
	if user == "" && assistant == "" {
		return
	}

	p.mu.Lock()
	p.messages = append(p.messages, Message{
		User:       user,
		Activities: activities,
		Assistant:  assistant,
	})
	p.mu.Unlock()
}

// SetBudgetStrategy sets the collection budget strategy for chat context.
func (m *InMemoryChatContextPartProvider) SetBudgetStrategy(strategy CollectionBudgetStrategy[Message]) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.budgetStrategy = strategy
}

// SetTokenBudget sets the token budget for chat context trimming.
func (m *InMemoryChatContextPartProvider) SetTokenBudget(tokens int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokenBudget = tokens
}

// formatMessageForContext formats a single message for context output.
// It is both the renderer and the budget-counting formatter, so
// activities are counted in their turn's token cost by construction.
func formatMessageForContext(msg Message) string {
	var parts []string
	if msg.User != "" {
		parts = append(parts, "User: "+msg.User)
	}
	if len(msg.Activities) > 0 {
		lines := make([]string, 0, len(msg.Activities)+1)
		// "Assistant Actions" names the actor explicitly, matching the
		// User:/Assistant: speaker labels around it.
		lines = append(lines, "Assistant Actions:")
		for _, activity := range msg.Activities {
			line := "- " + activity.Tool
			if activity.Args != "" {
				line += " " + activity.Args
			}
			if activity.Summary != "" {
				line += " → " + activity.Summary
			}
			lines = append(lines, line)
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}
	if msg.Assistant != "" {
		parts = append(parts, "Assistant: "+msg.Assistant)
	}
	return strings.Join(parts, "\n")
}

// GetPart returns the formatted conversation context
// pruneLowWater is the fraction of the token budget the history is cut
// down to when it overflows. Cutting below the budget, not just to it,
// is what keeps the prune from recurring on every turn: a history that
// overflows by one message would otherwise drop its oldest message on
// every read, and each drop shifts the whole prefix the provider's
// prompt cache had matched.
const pruneLowWater = 0.6

// History returns the turns the model will see this read, oldest first,
// after any budget prune. The prune is in place and permanent: turns that
// no longer fit are gone, so the window start stays fixed until the
// history overflows again. Callers get a copy.
func (m *InMemoryChatContextPartProvider) History(ctx context.Context) []Message {
	if pruneEvent := m.pruneToBudget(ctx); pruneEvent != nil {
		m.publishPrune(*pruneEvent)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	history := make([]Message, len(m.messages))
	copy(history, m.messages)
	return history
}

// pruneToBudget drops the oldest turns once the history overflows the
// token budget, keeping only what fits under the low-water mark. It
// reports the prune, or nil when nothing was dropped.
func (m *InMemoryChatContextPartProvider) pruneToBudget(ctx context.Context) *events.ContextPrunedEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.budgetStrategy == nil || m.tokenBudget <= 0 || historyTokens(m.messages) <= m.tokenBudget {
		return nil
	}
	lowWater := int(float64(m.tokenBudget) * pruneLowWater)
	kept, keptTokens := m.budgetStrategy.ApplyToCollection(m.messages, lowWater, formatMessageForContext)
	if len(kept) == 0 {
		// No single turn fits under the low-water mark; keep what fits the
		// full budget rather than emptying the conversation.
		kept, keptTokens = m.budgetStrategy.ApplyToCollection(m.messages, m.tokenBudget, formatMessageForContext)
	}
	dropped := len(m.messages) - len(kept)
	if dropped <= 0 {
		return nil
	}
	slog.InfoContext(ctx, "chat history pruned",
		"strategy", m.budgetStrategy.Name(),
		"total", len(m.messages),
		"kept", len(kept),
		"dropped", dropped,
		"kept_tokens", keptTokens,
		"budget_tokens", m.tokenBudget,
	)
	event := &events.ContextPrunedEvent{
		Strategy:     m.budgetStrategy.Name(),
		Total:        len(m.messages),
		Kept:         len(kept),
		Dropped:      dropped,
		KeptTokens:   keptTokens,
		BudgetTokens: m.tokenBudget,
	}
	m.messages = kept
	return event
}

func historyTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += EstimateTokens(formatMessageForContext(msg))
	}
	return total
}

// GetPart renders the history as one text part with the same formatter
// the budget is counted with, so what the model sees is exactly what was
// budgeted. Providers that consume History directly get the same turns.
func (m *InMemoryChatContextPartProvider) GetPart(ctx context.Context) (ContextPart, error) {
	var parts []string
	for _, msg := range m.History(ctx) {
		if formatted := formatMessageForContext(msg); formatted != "" {
			parts = append(parts, formatted)
		}
	}
	return ContextPart{
		Key:     "chat",
		Content: strings.Join(parts, "\n"),
	}, nil
}

// publishPrune announces a prune. Every prune is a real change to the
// history now that pruning is in place, so none is deduplicated.
func (m *InMemoryChatContextPartProvider) publishPrune(event events.ContextPrunedEvent) {
	if m.publisher == nil {
		return
	}
	m.publisher.PublishSync(event.Topic(), event)
}

// ClearPart removes all context
func (m *InMemoryChatContextPartProvider) ClearPart() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = make([]Message, 0)
	return nil
}

// SeedHistory replaces the current chat history with the provided messages.
func (m *InMemoryChatContextPartProvider) SeedHistory(history []Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(history) == 0 {
		m.messages = make([]Message, 0)
		return
	}

	m.messages = make([]Message, 0, len(history))
	for _, msg := range history {
		if msg.User == "" && msg.Assistant == "" {
			continue
		}
		m.messages = append(m.messages, Message{
			User:       msg.User,
			Activities: msg.Activities,
			Assistant:  msg.Assistant,
		})
	}
}
