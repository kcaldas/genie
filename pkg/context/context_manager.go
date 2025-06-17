package context

import (
	"fmt"

	"github.com/kcaldas/genie/pkg/events"
)

// ContextManager manages conversation context for sessions
type ContextManager interface {
	AddInteraction(sessionID, userMessage, assistantResponse string) error
	GetContext(sessionID string) ([]string, error)
	ClearContext(sessionID string) error
}

// InMemoryManager implements ContextManager with in-memory storage
type InMemoryManager struct {
	contexts map[string][]string
}

// NewContextManager creates a new context manager that listens to events from a channel
func NewContextManager(eventCh <-chan events.SessionInteractionEvent) ContextManager {
	manager := &InMemoryManager{
		contexts: make(map[string][]string),
	}

	// Start listening for events from the channel
	go manager.listenForEvents(eventCh)

	return manager
}

// NewInMemoryContextManager creates a new in-memory context manager without event subscription (for testing)
func NewInMemoryContextManager() ContextManager {
	return &InMemoryManager{
		contexts: make(map[string][]string),
	}
}

// listenForEvents handles session interaction events from the channel
func (m *InMemoryManager) listenForEvents(eventCh <-chan events.SessionInteractionEvent) {
	for event := range eventCh {
		// Use the existing AddInteraction method
		m.AddInteraction(event.SessionID, event.UserMessage, event.AssistantResponse)
	}
}

// AddInteraction adds a user message and assistant response to the session context
func (m *InMemoryManager) AddInteraction(sessionID, userMessage, assistantResponse string) error {
	if _, exists := m.contexts[sessionID]; !exists {
		m.contexts[sessionID] = make([]string, 0)
	}

	m.contexts[sessionID] = append(m.contexts[sessionID], userMessage, assistantResponse)
	return nil
}

// GetContext retrieves the conversation context for a session
func (m *InMemoryManager) GetContext(sessionID string) ([]string, error) {
	context, exists := m.contexts[sessionID]
	if !exists {
		return nil, fmt.Errorf("context for session %s not found", sessionID)
	}

	// Return a copy to prevent external modification
	result := make([]string, len(context))
	copy(result, context)
	return result, nil
}

// ClearContext removes all context for a session
func (m *InMemoryManager) ClearContext(sessionID string) error {
	delete(m.contexts, sessionID)
	return nil
}
