package genie

import (
	"context"

	"github.com/kcaldas/genie/pkg/events"
)

// Genie is the core AI assistant interface
type Genie interface {
	// Lifecycle management - returns initial session, must be called first
	Start(workingDir *string, persona *string, opts ...StartOption) (Session, error)

	// Chat operations - async, response via events (only work after Start)
	// Optional ChatOption values can be provided to attach images or tweak behaviour.
	Chat(ctx context.Context, message string, opts ...ChatOption) error

	// Context management - returns structured context parts by key
	GetContext(ctx context.Context) (map[string]string, error)

	// Status - returns the current status of the AI backend
	GetStatus() *Status

	// Event communication - get the event bus for async responses
	GetEventBus() events.EventBus

	// ListPersonas returns all available personas
	ListPersonas(ctx context.Context) ([]Persona, error)

	// GetSession returns the current session
	GetSession() (Session, error)
}

// Persona represents a discovered persona
type Persona interface {
	GetID() string
	GetName() string
	GetSource() string
}

// Session represents a conversation session
type Session interface {
	GetID() string
	GetWorkingDirectory() string        // CWD for file operations (--cwd parameter)
	GetGenieHomeDirectory() string      // Where .genie/ config lives (where genie was started)
	GetCreatedAt() string
	GetPersona() Persona
	SetPersona(persona Persona)
}

// SessionManager manages multiple sessions
type SessionManager interface {
	CreateSession(genieHomeDir string, workingDir string, persona Persona) (Session, error)
	GetSession() (Session, error)
}

// Status represents the current status of the AI backend
type Status struct {
	Connected bool
	Model     string
	Backend   string
	Message   string
}
