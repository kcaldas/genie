package context

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/events"
)

// ContextManager manages conversation context for sessions
type ContextManager interface {
	AddInteraction(sessionID, userMessage, assistantResponse string) error
	GetContext(sessionID string) ([]string, error)
	GetConversationContext(sessionID string, maxPairs int) (string, error)
	ClearContext(sessionID string) error
}

// InMemoryManager implements ContextManager with in-memory storage
type InMemoryManager struct {
	contexts map[string][]string
}

// NewContextManager creates a new context manager that subscribes to events from a bus
func NewContextManager(subscriber events.Subscriber) ContextManager {
	manager := &InMemoryManager{
		contexts: make(map[string][]string),
	}

	// Subscribe to session interaction events
	subscriber.Subscribe("session.interaction", manager.handleEvent)

	return manager
}

// NewInMemoryContextManager creates a new in-memory context manager without event subscription (for testing)
func NewInMemoryContextManager() ContextManager {
	return &InMemoryManager{
		contexts: make(map[string][]string),
	}
}

// handleEvent handles session interaction events from the event bus
func (m *InMemoryManager) handleEvent(event interface{}) {
	if sessionEvent, ok := event.(events.SessionInteractionEvent); ok {
		// Use the existing AddInteraction method
		m.AddInteraction(sessionEvent.SessionID, sessionEvent.UserMessage, sessionEvent.AssistantResponse)
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

// GetConversationContext builds a formatted conversation context string for AI prompts
func (m *InMemoryManager) GetConversationContext(sessionID string, maxPairs int) (string, error) {
	context, err := m.GetContext(sessionID)
	if err != nil {
		return "", err
	}
	
	// Context comes as alternating user/assistant messages
	// Only include complete pairs
	totalMessages := len(context)
	if totalMessages%2 != 0 {
		totalMessages-- // Remove incomplete pair
	}
	
	if totalMessages == 0 {
		return "", nil
	}
	
	pairsToInclude := maxPairs
	if totalMessages/2 < maxPairs {
		pairsToInclude = totalMessages / 2
	}
	
	startIdx := totalMessages - (pairsToInclude * 2)
	
	var contextBuilder strings.Builder
	for i := startIdx; i < totalMessages; i += 2 {
		contextBuilder.WriteString(fmt.Sprintf("User: %s\n", context[i]))
		contextBuilder.WriteString(fmt.Sprintf("Assistant: %s\n", context[i+1]))
	}
	
	return strings.TrimSpace(contextBuilder.String()), nil
}

// ClearContext removes all context for a session
func (m *InMemoryManager) ClearContext(sessionID string) error {
	delete(m.contexts, sessionID)
	return nil
}
