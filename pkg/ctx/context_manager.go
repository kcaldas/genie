package ctx

import (
	"context"
)

// ContextPart represents a part of the context with its key
type ContextPart struct {
	Key     string
	Content string
}

// ContextPartProvider provides context from various sources
type ContextPartProvider interface {
	GetPart(ctx context.Context) (ContextPart, error)
	ClearPart() error
}

// ContextPartProviderRegistry manages multiple context providers
type ContextPartProviderRegistry struct {
	providers []ContextPartProvider
}

// NewContextPartProviderRegistry creates a new context registry
func NewContextPartProviderRegistry() *ContextPartProviderRegistry {
	return &ContextPartProviderRegistry{
		providers: make([]ContextPartProvider, 0),
	}
}

// Register adds a provider to the registry
func (r *ContextPartProviderRegistry) Register(provider ContextPartProvider) {
	r.providers = append(r.providers, provider)
}

// GetProviders returns all registered providers
func (r *ContextPartProviderRegistry) GetProviders() []ContextPartProvider {
	return r.providers
}

// ContextManager manages conversation context
type ContextManager interface {
	GetContextParts(ctx context.Context) (map[string]string, error) // Returns all context parts
	ClearContext() error
	SeedChatHistory(history []Message)
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

// GetContextParts retrieves all context parts from registered providers
func (m *InMemoryManager) GetContextParts(ctx context.Context) (map[string]string, error) {
	parts := make(map[string]string)

	// Process all providers
	for _, provider := range m.registry.GetProviders() {
		part, err := provider.GetPart(ctx)
		if err != nil {
			return nil, err
		}

		if part.Content != "" {
			// Add the part to the map
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
		// Only clear chat context
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
