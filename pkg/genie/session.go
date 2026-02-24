package genie

import (
	"github.com/kcaldas/genie/pkg/events"
)

// DefaultPersona is a simple implementation of the Persona interface
type DefaultPersona struct {
	ID     string
	Name   string
	Source string
}

func (p *DefaultPersona) GetID() string     { return p.ID }
func (p *DefaultPersona) GetName() string   { return p.Name }
func (p *DefaultPersona) GetSource() string { return p.Source }

// InMemorySession implements Session with event bus publishing
type InMemorySession struct {
	id            string
	genieHomeDir  string   // Where .genie/ config lives
	workingDir    string   // CWD for file operations
	allowedDirs   []string // Extra directories tools may access
	persona       Persona
	publisher     events.Publisher
	createdAt     string
}

// NewSession creates a new session with genie home directory, working directory, allowed dirs, persona, and publisher for broadcasting
func NewSession(genieHomeDir string, workingDir string, allowedDirs []string, persona Persona, publisher events.Publisher) Session {
	return &InMemorySession{
		id:           "session-1", // TODO: generate unique IDs
		genieHomeDir: genieHomeDir,
		workingDir:   workingDir,
		allowedDirs:  allowedDirs,
		persona:      persona,
		publisher:    publisher,
		createdAt:    "TODO", // TODO: add actual timestamp
	}
}


// GetWorkingDirectory returns the session's working directory (CWD for file operations)
func (s *InMemorySession) GetWorkingDirectory() string {
	return s.workingDir
}

// GetAllowedDirectories returns the extra directories that tools may access
func (s *InMemorySession) GetAllowedDirectories() []string {
	return s.allowedDirs
}

// GetGenieHomeDirectory returns the directory where .genie/ config lives
func (s *InMemorySession) GetGenieHomeDirectory() string {
	return s.genieHomeDir
}

// GetPersona returns the session's selected persona
func (s *InMemorySession) GetPersona() Persona {
	return s.persona
}

// SetPersona sets the session's selected persona
func (s *InMemorySession) SetPersona(persona Persona) {
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
