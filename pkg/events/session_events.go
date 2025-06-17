package events

// SessionInteractionEvent represents a new interaction in a session
type SessionInteractionEvent struct {
	SessionID         string
	UserMessage       string
	AssistantResponse string
}

// Topic returns the event topic for session interactions
func (e SessionInteractionEvent) Topic() string {
	return "session.interaction"
}
