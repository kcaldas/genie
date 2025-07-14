package genie

import (
	"context"

	"github.com/kcaldas/genie/pkg/events"
)

// Genie is the core AI assistant interface
type Genie interface {
	// Lifecycle management - returns initial session, must be called first
	Start(workingDir *string, persona *string) (*Session, error)

	// Chat operations - async, response via events (only work after Start)
	Chat(ctx context.Context, message string) error

	// Context management - returns structured context parts by key
	GetContext(ctx context.Context) (map[string]string, error)

	// Status - returns the current status of the AI backend
	GetStatus() *Status

	// Event communication - get the event bus for async responses
	GetEventBus() events.EventBus
}

// Session represents a conversation session
type Session struct {
	ID               string
	WorkingDirectory string
	CreatedAt        string
}

// Status represents the current status of the AI backend
type Status struct {
	Connected bool
	Backend   string
	Message   string
}


