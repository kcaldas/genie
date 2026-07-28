package ctx

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/kcaldas/genie/pkg/events"
)

// Message represents a user/assistant conversation pair
type Message struct {
	User      string
	Assistant string
}

// ChatContextPartProvider manages chat context for a single session
type ChatContextPartProvider interface {
	ContextPartProvider
	SeedHistory(history []Message)
	SetBudgetStrategy(strategy CollectionBudgetStrategy[Message])
	// AddTurn records one completed exchange. Empty user or assistant
	// sides are allowed (ephemeral modes); a fully empty turn is ignored.
	AddTurn(user, assistant string)
}

// InMemoryChatContextPartProvider implements ChatCtxManager with in-memory storage
type InMemoryChatContextPartProvider struct {
	mu             sync.RWMutex
	messages       []Message
	budgetStrategy CollectionBudgetStrategy[Message]
	tokenBudget    int

	publisher events.Publisher
	pruneMu   sync.Mutex
	lastPrune *events.ContextPrunedEvent
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
func (p *InMemoryChatContextPartProvider) AddTurn(user, assistant string) {
	if user == "" && assistant == "" {
		return
	}
	p.addMessage(user, assistant)
}

func (p *InMemoryChatContextPartProvider) addMessage(user, assistant string) {
	message := Message{}

	if user != "" {
		message.User = user
	}

	if assistant != "" {
		message.Assistant = assistant
	}

	p.mu.Lock()
	p.messages = append(p.messages, message)
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

// formatMessage formats a single message for context output.
func formatMessageForContext(msg Message) string {
	var parts []string
	if msg.User != "" {
		parts = append(parts, "User: "+msg.User)
	}
	if msg.Assistant != "" {
		parts = append(parts, "Assistant: "+msg.Assistant)
	}
	return strings.Join(parts, "\n")
}

// GetPart returns the formatted conversation context
func (m *InMemoryChatContextPartProvider) GetPart(ctx context.Context) (ContextPart, error) {
	m.mu.RLock()
	messages := m.messages
	strategy := m.budgetStrategy
	tokenBudget := m.tokenBudget
	m.mu.RUnlock()

	// Apply sliding window if strategy and budget are set
	var pruneEvent *events.ContextPrunedEvent
	if strategy != nil && tokenBudget > 0 {
		kept, tokensUsed := strategy.ApplyToCollection(messages, tokenBudget, formatMessageForContext)
		if dropped := len(messages) - len(kept); dropped > 0 {
			slog.Info("chat history pruned",
				"strategy", strategy.Name(),
				"total", len(messages),
				"kept", len(kept),
				"dropped", dropped,
				"kept_tokens", tokensUsed,
				"budget_tokens", tokenBudget,
			)
			pruneEvent = &events.ContextPrunedEvent{
				Strategy:     strategy.Name(),
				Total:        len(messages),
				Kept:         len(kept),
				Dropped:      dropped,
				KeptTokens:   tokensUsed,
				BudgetTokens: tokenBudget,
			}
		}
		messages = kept
	}

	var parts []string
	for _, msg := range messages {
		if msg.User != "" {
			parts = append(parts, "User: "+msg.User)
		}
		if msg.Assistant != "" {
			parts = append(parts, "Assistant: "+msg.Assistant)
		}
	}

	// Publish outside any lock: prune recomputes on every read, so an
	// unchanged outcome is deduped against the last published event.
	if pruneEvent != nil {
		m.publishPrune(*pruneEvent)
	}

	return ContextPart{
		Key:     "chat",
		Content: strings.Join(parts, "\n"),
	}, nil
}

// publishPrune publishes a prune outcome unless it matches the previously
// published one.
func (m *InMemoryChatContextPartProvider) publishPrune(event events.ContextPrunedEvent) {
	if m.publisher == nil {
		return
	}
	m.pruneMu.Lock()
	if m.lastPrune != nil && *m.lastPrune == event {
		m.pruneMu.Unlock()
		return
	}
	m.lastPrune = &event
	m.pruneMu.Unlock()
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
			User:      msg.User,
			Assistant: msg.Assistant,
		})
	}
}
