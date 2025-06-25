package ctx

import (
	"context"
	"strings"

	"github.com/kcaldas/genie/pkg/events"
)

// ContextManager manages conversation context for sessions
type ContextManager interface {
	GetLLMContext(ctx context.Context, sessionID string) (string, error) // Returns formatted context for LLM with internal window size
	ClearContext(sessionID string) error
}

// InMemoryManager implements ContextManager with in-memory storage
type InMemoryManager struct {
	contexts          map[string][]string
	projectCtxManager ProjectCtxManager
}

// InMemoryManagerWithChatCtx implements ContextManager with ChatCtxManager integration
type InMemoryManagerWithChatCtx struct {
	projectCtxManager ProjectCtxManager
	chatCtxManager    ChatCtxManager
}

// NewContextManager creates a new context manager that subscribes to events from a bus
func NewContextManager(subscriber events.Subscriber, projectCtxManager ProjectCtxManager) ContextManager {
	manager := &InMemoryManager{
		contexts:          make(map[string][]string),
		projectCtxManager: projectCtxManager,
	}

	// Subscribe to chat response events
	subscriber.Subscribe("chat.response", manager.handleChatResponseEvent)

	return manager
}

// NewContextManagerWithChatCtx creates a new context manager with ChatCtxManager integration
func NewContextManagerWithChatCtx(projectCtxManager ProjectCtxManager, chatCtxManager ChatCtxManager) ContextManager {
	return &InMemoryManagerWithChatCtx{
		projectCtxManager: projectCtxManager,
		chatCtxManager:    chatCtxManager,
	}
}

// NewInMemoryContextManager creates a new in-memory context manager without event subscription (for testing)
func NewInMemoryContextManager() ContextManager {
	// Create a minimal project context manager for testing
	projectCtxManager := NewProjectCtxManager(nil)
	return &InMemoryManager{
		contexts:          make(map[string][]string),
		projectCtxManager: projectCtxManager,
	}
}

// handleChatResponseEvent handles chat response events from the event bus
func (m *InMemoryManager) handleChatResponseEvent(event any) {
	if chatEvent, ok := event.(events.ChatResponseEvent); ok {
		// Add the user message and assistant response to context
		m.addInteraction(chatEvent.SessionID, chatEvent.Message, chatEvent.Response)
	}
}

// addInteraction adds a user message and assistant response to the session context
func (m *InMemoryManager) addInteraction(sessionID, userMessage, assistantResponse string) {
	if _, exists := m.contexts[sessionID]; !exists {
		m.contexts[sessionID] = make([]string, 0)
	}

	m.contexts[sessionID] = append(m.contexts[sessionID], userMessage, assistantResponse)
}

// GetLLMContext returns formatted context for LLM with internal default window size
func (m *InMemoryManager) GetLLMContext(ctx context.Context, sessionID string) (string, error) {
	var result []string

	// Add project context at the top
	projectContext, err := m.projectCtxManager.GetContext(ctx)
	if err != nil {
		return "", err
	}
	if projectContext != "" {
		result = append(result, projectContext)
	}

	// Add session context
	sessionContext, exists := m.contexts[sessionID]
	if exists {
		result = append(result, sessionContext...)
	}

	resultString := strings.Join(result, "\n")

	return resultString, nil
}

// ClearContext removes all context for a session
func (m *InMemoryManager) ClearContext(sessionID string) error {
	delete(m.contexts, sessionID)
	return nil
}

// GetLLMContext for InMemoryManagerWithChatCtx combines project and chat context
func (m *InMemoryManagerWithChatCtx) GetLLMContext(ctx context.Context, sessionID string) (string, error) {
	var result []string

	// Add project context at the top
	projectContext, err := m.projectCtxManager.GetContext(ctx)
	if err != nil {
		return "", err
	}
	if projectContext != "" {
		result = append(result, projectContext)
	}

	// Add chat context from ChatCtxManager
	chatContext := m.chatCtxManager.GetContext()
	if chatContext != "" {
		result = append(result, chatContext)
	}

	return strings.Join(result, "\n"), nil
}

// ClearContext for InMemoryManagerWithChatCtx delegates to ChatCtxManager
func (m *InMemoryManagerWithChatCtx) ClearContext(sessionID string) error {
	return m.chatCtxManager.ClearContext()
}
