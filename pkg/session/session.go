package session

import (
	"github.com/kcaldas/genie/pkg/events"
)

// Session represents a conversation session
type Session interface {
	GetWorkingDirectory() string
	GetPersona() string
}

// InMemorySession implements Session with event bus publishing
type InMemorySession struct {
	workingDir string
	persona    string
	publisher  events.Publisher
}

// NewSession creates a new session with working directory, persona, and publisher for broadcasting
func NewSession(workingDir string, persona string, publisher events.Publisher) Session {
	return &InMemorySession{
		workingDir: workingDir,
		persona:    persona,
		publisher:  publisher,
	}
}


// GetWorkingDirectory returns the session's working directory
func (s *InMemorySession) GetWorkingDirectory() string {
	return s.workingDir
}

// GetPersona returns the session's selected persona
func (s *InMemorySession) GetPersona() string {
	return s.persona
}
