package ctx

import (
	"context"

	"github.com/kcaldas/genie/pkg/events"
)

// ContextPart represents a part of the context with its key
type ContextPart struct {
	Key     string
	Content string
}

// ContextPartProvider provides context from various sources.
// All providers are budget-aware — SetTokenBudget is called before GetPart
// during context assembly. Providers that don't need budgets can ignore it.
type ContextPartProvider interface {
	SetTokenBudget(tokens int)
	GetPart(ctx context.Context) (ContextPart, error)
	ClearPart() error
}

// ContextPartProviderRegistry holds the text context providers and the
// chat history, each with a share of the context token budget.
type ContextPartProviderRegistry struct {
	providers    []ContextPartProvider
	budgetShares []float64
	chat         ChatHistory
	chatShare    float64
}

// NewContextPartProviderRegistry creates a new context registry
func NewContextPartProviderRegistry() *ContextPartProviderRegistry {
	return &ContextPartProviderRegistry{
		providers:    make([]ContextPartProvider, 0),
		budgetShares: make([]float64, 0),
	}
}

// Register adds a provider to the registry with a budget share.
// Share is a relative weight (e.g., 0.7 and 0.3). Providers with share 0 get no budget.
func (r *ContextPartProviderRegistry) Register(provider ContextPartProvider, budgetShare float64) {
	r.providers = append(r.providers, provider)
	r.budgetShares = append(r.budgetShares, budgetShare)
}

// RegisterChat sets the chat history and its budget share. The history is
// not a context part: it reaches the model as structured turns, never as
// template text.
func (r *ContextPartProviderRegistry) RegisterChat(chat ChatHistory, budgetShare float64) {
	r.chat = chat
	r.chatShare = budgetShare
}

// GetProviders returns all registered providers
func (r *ContextPartProviderRegistry) GetProviders() []ContextPartProvider {
	return r.providers
}

// Chat returns the registered chat history, or nil.
func (r *ContextPartProviderRegistry) Chat() ChatHistory {
	return r.chat
}

// ContextManager manages conversation context
type ContextManager interface {
	// GetContextParts renders the text context parts by key. The chat
	// history is not among them; see ChatHistory.
	GetContextParts(ctx context.Context) (map[string]string, error)
	// ChatHistory returns the conversation turns the model will see this
	// turn, oldest first, after the chat history's budget prune. The
	// error is reserved for stores that can fail to read; the in-memory
	// one never does.
	ChatHistory(ctx context.Context) ([]Message, error)
	ClearContext() error
	SeedChatHistory(history []Message)
	// RecordChatTurn synchronously appends a completed exchange to the
	// conversation history, optionally with the tool activities that
	// produced it. The core calls this after each successful turn;
	// history must never depend on asynchronous event delivery.
	RecordChatTurn(user, assistant string, activities ...events.ToolActivity)
	SetContextBudget(totalTokens int)
}

// InMemoryManager implements ContextManager with registry-based providers
type InMemoryManager struct {
	registry *ContextPartProviderRegistry
}

// NewContextManager creates a new context manager with registry
func NewContextManager(registry *ContextPartProviderRegistry) ContextManager {
	return &InMemoryManager{
		registry: registry,
	}
}

// SetContextBudget distributes the token budget across the providers and
// the chat history based on their registered shares.
func (m *InMemoryManager) SetContextBudget(totalTokens int) {
	totalShare := 0.0
	for _, s := range m.registry.budgetShares {
		totalShare += s
	}
	if m.registry.chat != nil {
		totalShare += m.registry.chatShare
	}
	budgetFor := func(share float64) int {
		if totalShare > 0 && share > 0 {
			return int(float64(totalTokens) * share / totalShare)
		}
		return 0
	}

	for i, provider := range m.registry.providers {
		provider.SetTokenBudget(budgetFor(m.registry.budgetShares[i]))
	}
	if m.registry.chat != nil {
		m.registry.chat.SetTokenBudget(budgetFor(m.registry.chatShare))
	}
}

// GetContextParts retrieves all context parts from registered providers.
func (m *InMemoryManager) GetContextParts(ctx context.Context) (map[string]string, error) {
	parts := make(map[string]string)
	for _, provider := range m.registry.GetProviders() {
		part, err := provider.GetPart(ctx)
		if err != nil {
			return nil, err
		}
		if part.Content != "" {
			parts[part.Key] = part.Content
		}
	}
	return parts, nil
}

// ChatHistory returns the chat history's kept turns, or nil without one.
func (m *InMemoryManager) ChatHistory(ctx context.Context) ([]Message, error) {
	if m.registry.chat == nil {
		return nil, nil
	}
	return m.registry.chat.History(ctx), nil
}

// ClearContext forgets the chat history; other context parts are kept.
func (m *InMemoryManager) ClearContext() error {
	if m.registry.chat != nil {
		m.registry.chat.Clear()
	}
	return nil
}

// SeedChatHistory replaces the chat history with restored turns.
func (m *InMemoryManager) SeedChatHistory(history []Message) {
	if len(history) == 0 || m.registry.chat == nil {
		return
	}
	m.registry.chat.SeedHistory(history)
}

// RecordChatTurn appends a completed exchange to the chat history.
func (m *InMemoryManager) RecordChatTurn(user, assistant string, activities ...events.ToolActivity) {
	if m.registry.chat == nil {
		return
	}
	m.registry.chat.AddTurn(user, assistant, activities...)
}
