package session

import (
	"github.com/kcaldas/genie/pkg/events"
)

// Session represents a conversation session
type Session interface {
	GetID() string
	AddInteraction(userMessage, assistantResponse string) error
}

// InMemorySession implements Session with event bus publishing
type InMemorySession struct {
	id        string
	publisher events.Publisher
}

// NewSession creates a new session with publisher for broadcasting
func NewSession(id string, publisher events.Publisher) Session {
	return &InMemorySession{
		id:        id,
		publisher: publisher,
	}
}

// GetID returns the session ID
func (s *InMemorySession) GetID() string {
	return s.id
}

// AddInteraction adds a user message and assistant response by publishing to event bus
func (s *InMemorySession) AddInteraction(userMessage, assistantResponse string) error {
	// Create event
	event := events.SessionInteractionEvent{
		SessionID:         s.id,
		UserMessage:       userMessage,
		AssistantResponse: assistantResponse,
	}

	// Publish to event bus (synchronous)
	s.publisher.Publish("session.interaction", event)

	return nil
}
