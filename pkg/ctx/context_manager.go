package ctx

import (
	"context"
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

// ContextPartProviderRegistry manages multiple context providers
type ContextPartProviderRegistry struct {
	providers    []ContextPartProvider
	budgetShares []float64
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

// GetProviders returns all registered providers
func (r *ContextPartProviderRegistry) GetProviders() []ContextPartProvider {
	return r.providers
}

// ContextManager manages conversation context
type ContextManager interface {
	GetContextParts(ctx context.Context) (map[string]string, error)
	ClearContext() error
	SeedChatHistory(history []Message)
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

// SetContextBudget distributes the token budget across providers based on their registered shares.
func (m *InMemoryManager) SetContextBudget(totalTokens int) {
	totalShare := 0.0
	for _, s := range m.registry.budgetShares {
		totalShare += s
	}

	for i, provider := range m.registry.providers {
		share := m.registry.budgetShares[i]
		if totalShare > 0 && share > 0 {
			provider.SetTokenBudget(int(float64(totalTokens) * share / totalShare))
		} else {
			provider.SetTokenBudget(0)
		}
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

// ClearContext clears the chat context only (maintains current behavior)
func (m *InMemoryManager) ClearContext() error {
	for _, provider := range m.registry.GetProviders() {
		part, err := provider.GetPart(context.Background())
		if err != nil {
			continue
		}
		if part.Key == "chat" {
			return provider.ClearPart()
		}
	}
	return nil
}

func (m *InMemoryManager) SeedChatHistory(history []Message) {
	if len(history) == 0 {
		return
	}
	for _, provider := range m.registry.GetProviders() {
		if seedable, ok := provider.(interface{ SeedHistory([]Message) }); ok {
			seedable.SeedHistory(history)
			return
		}
	}
}
