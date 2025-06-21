package genie

// ChatResponseEvent is published when AI generates a response
type ChatResponseEvent struct {
	SessionID string
	Message   string
	Response  string
	Error     error
}

// Topic returns the event topic for chat responses
func (e ChatResponseEvent) Topic() string {
	return "chat.response"
}

// ChatStartedEvent is published when chat processing begins
type ChatStartedEvent struct {
	SessionID string
	Message   string
}

// Topic returns the event topic for chat started events
func (e ChatStartedEvent) Topic() string {
	return "chat.started"
}