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

// ToolExecutedEvent represents a tool that has been executed
type ToolExecutedEvent struct {
	SessionID string
	ToolName  string
	Message   string
}

// Topic returns the event topic for tool execution
func (e ToolExecutedEvent) Topic() string {
	return "tool.executed"
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

// NoOpPublisher is a publisher that does nothing (for testing or when events are not needed)
type NoOpPublisher struct{}

// Publish does nothing
func (n *NoOpPublisher) Publish(topic string, event interface{}) {
	// No-op
}
