package genie

import (
	"context"

	"github.com/kcaldas/genie/pkg/events"
)

// Genie is the core AI assistant interface
type Genie interface {
	// Lifecycle management - returns initial session, must be called first
	Start(workingDir *string) (*Session, error)

	// Chat operations - async, response via events (only work after Start)
	Chat(ctx context.Context, message string) error

	// Context management - returns structured context parts by key
	GetContext(ctx context.Context) (map[string]string, error)

	// Event communication - get the event bus for async responses
	GetEventBus() events.EventBus
}

// Session represents a conversation session
type Session struct {
	ID               string
	WorkingDirectory string
	CreatedAt        string
	Interactions     []Interaction
}

// Interaction represents a single message-response pair
type Interaction struct {
	Message  string
	Response string
	Time     string
}

