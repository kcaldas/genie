package session

import (
	"github.com/kcaldas/genie/pkg/events"
)

// Session represents a conversation session
type Session interface {
	GetWorkingDirectory() string
	GetPersona() string
	SetPersona(persona string)
	GetID() string
	GetCreatedAt() string
}

// InMemorySession implements Session with event bus publishing
type InMemorySession struct {
	id         string
	workingDir string
	persona    string
	publisher  events.Publisher
	createdAt  string
}

// NewSession creates a new session with working directory, persona, and publisher for broadcasting
func NewSession(workingDir string, persona string, publisher events.Publisher) Session {
	return &InMemorySession{
		id:         "session-1", // TODO: generate unique IDs
		workingDir: workingDir,
		persona:    persona,
		publisher:  publisher,
		createdAt:  "TODO", // TODO: add actual timestamp
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

// SetPersona sets the session's selected persona
func (s *InMemorySession) SetPersona(persona string) {
	s.persona = persona
}

// GetID returns the session's unique identifier
func (s *InMemorySession) GetID() string {
	return s.id
}

// GetCreatedAt returns the session's creation timestamp
func (s *InMemorySession) GetCreatedAt() string {
	return s.createdAt
}
