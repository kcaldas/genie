package session

import (
	"github.com/kcaldas/genie/pkg/events"
)

// Session represents a conversation session
type Session interface {
	GetWorkingDirectory() string
}

// InMemorySession implements Session with event bus publishing
type InMemorySession struct {
	workingDir string
	publisher  events.Publisher
}

// NewSession creates a new session with working directory and publisher for broadcasting
func NewSession(workingDir string, publisher events.Publisher) Session {
	return &InMemorySession{
		workingDir: workingDir,
		publisher:  publisher,
	}
}

// GetWorkingDirectory returns the session's working directory
func (s *InMemorySession) GetWorkingDirectory() string {
	return s.workingDir
}
