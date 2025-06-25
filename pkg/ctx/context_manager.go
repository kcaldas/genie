package ctx

import (
	"context"
	"strings"
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
	GetLLMContext(ctx context.Context) (string, error)              // Returns formatted context for LLM
	ClearContext() error
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

// GetLLMContext combines all registered contexts
func (m *InMemoryManager) GetLLMContext(ctx context.Context) (string, error) {
	var result []string
	var hasProjectContext bool

	// Process all providers
	for _, provider := range m.registry.GetProviders() {
		part, err := provider.GetPart(ctx)
		if err != nil {
			return "", err
		}

		if part.Content != "" {
			// Add delimiter for chat context if there's previous content
			if part.Key == "chat" && hasProjectContext {
				result = append(result, "\n# Recent Conversation History\n")
			}
			result = append(result, part.Content)

			if part.Key == "project" {
				hasProjectContext = true
			}
		}
	}

	return strings.Join(result, "\n"), nil
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
