package session

import (
	"fmt"

	"github.com/kcaldas/genie/pkg/events"
)

// SessionManager manages multiple sessions
type SessionManager interface {
	CreateSession(workingDir string) (Session, error)
	GetSession() (Session, error)
}

// InMemoryManager implements SessionManager with in-memory storage
type InMemoryManager struct {
	session   Session
	publisher events.Publisher
}

// NewSessionManager creates a new session manager
func NewSessionManager(publisher events.Publisher) SessionManager {
	return &InMemoryManager{
		publisher: publisher,
	}
}

// CreateSession creates a new session with the given ID and working directory
func (m *InMemoryManager) CreateSession(workingDir string) (Session, error) {
	m.session = NewSession(workingDir, m.publisher)
	return m.session, nil
}

// GetSession retrieves an existing session by ID
func (m *InMemoryManager) GetSession() (Session, error) {
	if m.session == nil {
		return nil, fmt.Errorf("no session exists, please create one first")
	}
	return m.session, nil
}
