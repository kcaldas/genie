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

// HistoryChannel is a typed channel for history events
type HistoryChannel chan SessionInteractionEvent

// ContextChannel is a typed channel for context events  
type ContextChannel chan SessionInteractionEvent

// NewHistoryChannel creates a new channel for history events
func NewHistoryChannel() HistoryChannel {
	return make(chan SessionInteractionEvent, 10)
}

// NewContextChannel creates a new channel for context events
func NewContextChannel() ContextChannel {
	return make(chan SessionInteractionEvent, 10)
}
