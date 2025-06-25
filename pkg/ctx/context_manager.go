package ctx

import (
	"context"
	"strings"
)

// ContextManager manages conversation context
type ContextManager interface {
	GetLLMContext(ctx context.Context) (string, error) // Returns formatted context for LLM
	ClearContext() error
}

// InMemoryManager implements ContextManager with ChatCtxManager integration
type InMemoryManager struct {
	projectCtxManager ProjectCtxManager
	chatCtxManager    ChatCtxManager
}

// NewContextManager creates a new context manager with ChatCtxManager integration
func NewContextManager(projectCtxManager ProjectCtxManager, chatCtxManager ChatCtxManager) ContextManager {
	return &InMemoryManager{
		projectCtxManager: projectCtxManager,
		chatCtxManager:    chatCtxManager,
	}
}

// GetLLMContext combines project and chat context
func (m *InMemoryManager) GetLLMContext(ctx context.Context) (string, error) {
	var result []string

	// Add project context at the top
	projectContext, err := m.projectCtxManager.GetContext(ctx)
	if err != nil {
		return "", err
	}
	if projectContext != "" {
		result = append(result, projectContext)
	}

	// Add chat context from ChatCtxManager with delimiter
	chatContext := m.chatCtxManager.GetContext()
	if chatContext != "" {
		result = append(result, "\n# Recent Conversation History\n")
		result = append(result, chatContext)
	}

	return strings.Join(result, "\n"), nil
}

// ClearContext delegates to ChatCtxManager
func (m *InMemoryManager) ClearContext() error {
	return m.chatCtxManager.ClearContext()
}
