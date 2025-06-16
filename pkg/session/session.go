package session

import (
	"github.com/kcaldas/genie/pkg/context"
	"github.com/kcaldas/genie/pkg/history"
)

// Session represents a conversation session
type Session interface {
	GetID() string
	AddInteraction(userMessage, assistantResponse string) error
	GetContext() []string
	GetHistory() []string
}

// InMemorySession implements Session with context and history managers
type InMemorySession struct {
	id             string
	contextManager context.ContextManager
	historyManager history.HistoryManager
}

// NewSessionWithManagers creates a new session with context and history managers
func NewSessionWithManagers(id string, contextManager context.ContextManager, historyManager history.HistoryManager) Session {
	return &InMemorySession{
		id:             id,
		contextManager: contextManager,
		historyManager: historyManager,
	}
}

// NewSessionWithContext creates a new session with a context manager (backward compatibility)
func NewSessionWithContext(id string, contextManager context.ContextManager) Session {
	return NewSessionWithManagers(id, contextManager, history.NewHistoryManager())
}

// GetID returns the session ID
func (s *InMemorySession) GetID() string {
	return s.id
}

// AddInteraction adds a user message and assistant response to both context and history
func (s *InMemorySession) AddInteraction(userMessage, assistantResponse string) error {
	// Broadcast to both managers
	err := s.contextManager.AddInteraction(s.id, userMessage, assistantResponse)
	if err != nil {
		return err
	}
	
	err = s.historyManager.AddInteraction(s.id, userMessage, assistantResponse)
	if err != nil {
		return err
	}
	
	return nil
}

// GetContext returns the current conversation context
func (s *InMemorySession) GetContext() []string {
	context, err := s.contextManager.GetContext(s.id)
	if err != nil {
		// Return empty slice if no context exists yet
		return []string{}
	}
	return context
}

// GetHistory returns the complete conversation history
func (s *InMemorySession) GetHistory() []string {
	history, err := s.historyManager.GetHistory(s.id)
	if err != nil {
		// Return empty slice if no history exists yet
		return []string{}
	}
	return history
}