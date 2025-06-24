package session

import (
	"github.com/kcaldas/genie/pkg/events"
)

// Session represents a conversation session
type Session interface {
	GetID() string
	GetWorkingDirectory() string
	AddInteraction(userMessage, assistantResponse string) error
}

// InMemorySession implements Session with event bus publishing
type InMemorySession struct {
	id           string
	workingDir   string
	publisher    events.Publisher
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
