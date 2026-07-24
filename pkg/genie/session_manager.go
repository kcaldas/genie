package genie

import (
	"fmt"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/session"
)

// InMemoryManager implements SessionManager with in-memory storage
type InMemoryManager struct {
	session   Session
	publisher events.Publisher
	recorder  *session.Recorder
}

// NewSessionManager creates a new session manager. The recorder may be nil
// (recording disabled); sessions use it to record persona changes.
func NewSessionManager(publisher events.Publisher, recorder *session.Recorder) SessionManager {
	return &InMemoryManager{
		publisher: publisher,
		recorder:  recorder,
	}
}

// CreateSession creates a new session with the given genie home directory, working directory, allowed dirs, and persona
func (m *InMemoryManager) CreateSession(genieHomeDir string, workingDir string, allowedDirs []string, persona Persona) (Session, error) {
	m.session = NewSession(genieHomeDir, workingDir, allowedDirs, persona, m.publisher, m.recorder)
	return m.session, nil
}

// GetSession retrieves an existing session by ID
func (m *InMemoryManager) GetSession() (Session, error) {
	if m.session == nil {
		return nil, fmt.Errorf("no session exists, please create one first")
	}
	return m.session, nil
}
