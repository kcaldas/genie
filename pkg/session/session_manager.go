package session

import (
	"fmt"

	"github.com/kcaldas/genie/pkg/events"
)

// SessionManager manages multiple sessions
type SessionManager interface {
	CreateSession(id string) (Session, error)
	GetSession(id string) (Session, error)
}

// InMemoryManager implements SessionManager with in-memory storage
type InMemoryManager struct {
	sessions  map[string]Session
	publisher events.Publisher
}

// NewSessionManager creates a new session manager
func NewSessionManager(publisher events.Publisher) SessionManager {
	return &InMemoryManager{
		sessions:  make(map[string]Session),
		publisher: publisher,
	}
}

// CreateSession creates a new session with the given ID
func (m *InMemoryManager) CreateSession(id string) (Session, error) {
	var session Session
	session = NewSession(id, m.publisher)
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
