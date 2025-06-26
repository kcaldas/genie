package session

import (
	"github.com/kcaldas/genie/pkg/events"
)

// Session represents a conversation session
type Session interface {
	GetID() string
	GetWorkingDirectory() string
}

// InMemorySession implements Session with event bus publishing
type InMemorySession struct {
	id         string
	workingDir string
	publisher  events.Publisher
}

// NewSession creates a new session with working directory and publisher for broadcasting
func NewSession(id string, workingDir string, publisher events.Publisher) Session {
	return &InMemorySession{
		id:         id,
		workingDir: workingDir,
		publisher:  publisher,
	}
}

// GetID returns the session ID
func (s *InMemorySession) GetID() string {
	return s.id
}

// GetWorkingDirectory returns the session's working directory
func (s *InMemorySession) GetWorkingDirectory() string {
	return s.workingDir
}
