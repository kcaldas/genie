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
	GetContext(ctx context.Context) (ContextPart, error)
	ClearContext() error
}

// ContextRegistry manages multiple context providers
type ContextRegistry struct {
	providers []ContextPartProvider
}

// NewContextRegistry creates a new context registry
func NewContextRegistry() *ContextRegistry {
	return &ContextRegistry{
		providers: make([]ContextPartProvider, 0),
	}
}

// Register adds a provider to the registry
func (r *ContextRegistry) Register(provider ContextPartProvider) {
	r.providers = append(r.providers, provider)
}

// GetProviders returns all registered providers
func (r *ContextRegistry) GetProviders() []ContextPartProvider {
	return r.providers
}

// ContextManager manages conversation context
type ContextManager interface {
	GetLLMContext(ctx context.Context) (string, error) // Returns formatted context for LLM
	ClearContext() error
}

// InMemoryManager implements ContextManager with registry-based providers
type InMemoryManager struct {
	registry *ContextRegistry
}

// NewContextManager creates a new context manager with registry
func NewContextManager(registry *ContextRegistry) ContextManager {
	return &InMemoryManager{
		registry: registry,
	}
}

// GetLLMContext combines all registered contexts
func (m *InMemoryManager) GetLLMContext(ctx context.Context) (string, error) {
	var result []string
	var hasProjectContext bool

	// Process all providers
	for _, provider := range m.registry.GetProviders() {
		part, err := provider.GetContext(ctx)
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
		part, err := provider.GetContext(context.Background())
		if err != nil {
			continue
		}
		// Only clear chat context
		if part.Key == "chat" {
			return provider.ClearContext()
		}
	}
	return nil
}
