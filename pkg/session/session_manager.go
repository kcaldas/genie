package session

import (
	"fmt"
	
	"github.com/kcaldas/genie/pkg/context"
	"github.com/kcaldas/genie/pkg/history"
)

// SessionManager manages multiple sessions
type SessionManager interface {
	CreateSession(id string) (Session, error)
	GetSession(id string) (Session, error)
}

// InMemoryManager implements SessionManager with in-memory storage
type InMemoryManager struct {
	sessions       map[string]Session
	contextManager context.ContextManager
	historyManager history.HistoryManager
}

// NewSessionManagerWithManagers creates a new session manager with both context and history managers
func NewSessionManagerWithManagers(contextManager context.ContextManager, historyManager history.HistoryManager) SessionManager {
	return &InMemoryManager{
		sessions:       make(map[string]Session),
		contextManager: contextManager,
		historyManager: historyManager,
	}
}

// NewSessionManagerWithContext creates a new session manager with context manager (backward compatibility)
func NewSessionManagerWithContext(contextManager context.ContextManager) SessionManager {
	return NewSessionManagerWithManagers(contextManager, history.NewHistoryManager())
}

// CreateSession creates a new session with the given ID
func (m *InMemoryManager) CreateSession(id string) (Session, error) {
	session := NewSessionWithManagers(id, m.contextManager, m.historyManager)
	m.sessions[id] = session
	return session, nil
}

// GetSession retrieves an existing session by ID
func (m *InMemoryManager) GetSession(id string) (Session, error) {
	session, exists := m.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return session, nil
}