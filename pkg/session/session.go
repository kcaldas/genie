package session

import (
	"github.com/kcaldas/genie/pkg/events"
)

// Session represents a conversation session
type Session interface {
	GetID() string
	AddInteraction(userMessage, assistantResponse string) error
}

// InMemorySession implements Session with channel broadcasting
type InMemorySession struct {
	id        string
	historyCh chan<- events.SessionInteractionEvent
	contextCh chan<- events.SessionInteractionEvent
}

// NewSession creates a new session with channels for broadcasting
func NewSession(id string, historyCh, contextCh chan<- events.SessionInteractionEvent) Session {
	return &InMemorySession{
		id:        id,
		historyCh: historyCh,
		contextCh: contextCh,
	}
}

// GetID returns the session ID
func (s *InMemorySession) GetID() string {
	return s.id
}

// AddInteraction adds a user message and assistant response by broadcasting to channels
func (s *InMemorySession) AddInteraction(userMessage, assistantResponse string) error {
	// Create event
	event := events.SessionInteractionEvent{
		SessionID:         s.id,
		UserMessage:       userMessage,
		AssistantResponse: assistantResponse,
	}

	// Broadcast to both channels (non-blocking)
	select {
	case s.historyCh <- event:
	default:
		// Skip if history channel is full
	}

	select {
	case s.contextCh <- event:
	default:
		// Skip if context channel is full
	}

	return nil
}
