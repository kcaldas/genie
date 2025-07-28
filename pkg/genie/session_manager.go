package genie

import (
	"fmt"

	"github.com/kcaldas/genie/pkg/events"
)


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

// CreateSession creates a new session with the given working directory and persona
func (m *InMemoryManager) CreateSession(workingDir string, persona Persona) (Session, error) {
	m.session = NewSession(workingDir, persona, m.publisher)
	return m.session, nil
}

// GetSession retrieves an existing session by ID
func (m *InMemoryManager) GetSession() (Session, error) {
	if m.session == nil {
		return nil, fmt.Errorf("no session exists, please create one first")
	}
	return m.session, nil
}
