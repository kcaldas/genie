package events

// ToolExecutedEvent represents a tool that has been executed
type ToolExecutedEvent struct {
	ExecutionID string
	ToolName    string
	Parameters  map[string]any
	Message     string
	Result      map[string]any // The actual result returned by the tool
}

// Topic returns the event topic for tool execution
func (e ToolExecutedEvent) Topic() string {
	return "tool.executed"
}

// ToolConfirmationRequest represents a request for user confirmation before executing a tool
type ToolConfirmationRequest struct {
	ExecutionID string
	ToolName    string
	Command     string
	Message     string
}

// Topic returns the event topic for tool confirmation requests
func (e ToolConfirmationRequest) Topic() string {
	return "tool.confirmation.request"
}

// ToolConfirmationResponse represents a user's response to a confirmation request
type ToolConfirmationResponse struct {
	ExecutionID string
	Confirmed   bool
}

// Topic returns the event topic for tool confirmation responses
func (e ToolConfirmationResponse) Topic() string {
	return "tool.confirmation.response"
}

// UserConfirmationRequest represents a generic request for user confirmation with content preview
type UserConfirmationRequest struct {
	ExecutionID string
	Title       string // Title of the confirmation dialog
	Content     string // Content to display (diff, plan, etc.)
	ContentType string // "diff", "plan", etc. for rendering hints
	FilePath    string // Optional: for file-specific confirmations
	Message     string // Optional: custom message
	ConfirmText string // Optional: custom confirm button text
	CancelText  string // Optional: custom cancel button text
}

// Topic returns the event topic for user confirmation requests
func (e UserConfirmationRequest) Topic() string {
	return "user.confirmation.request"
}

// UserConfirmationResponse represents a user's response to a confirmation request
type UserConfirmationResponse struct {
	ExecutionID string
	Confirmed   bool
}

// Topic returns the event topic for user confirmation responses
func (e UserConfirmationResponse) Topic() string {
	return "user.confirmation.response"
}

// ChatResponseEvent is published when AI generates a response
type ChatResponseEvent struct {
	Message   string
	Response  string
	Error     error
	UserInput string // Add UserInput field
}

// Topic returns the event topic for chat responses
func (e ChatResponseEvent) Topic() string {
	return "chat.response"
}

// ChatStartedEvent is published when chat processing begins
type ChatStartedEvent struct {
	Message string
}

// Topic returns the event topic for chat started events
func (e ChatStartedEvent) Topic() string {
	return "chat.started"
}

// ToolCallMessageEvent is published when a tool call wants to display a message to the user
type ToolCallMessageEvent struct {
	ToolName string
	Message  string
}

// Topic returns the event topic for tool call messages
func (e ToolCallMessageEvent) Topic() string {
	return "tool.call.message"
}

// NotificationEvent is published to give user intermediary feedback. Reasoning, errors
type NotificationEvent struct {
	Message string
	Error   error
}

// Topic returns the event topic for notification events
func (e NotificationEvent) Topic() string {
	return "chat.notification"
}

// NoOpPublisher is a publisher that does nothing (for testing or when events are not needed)
type NoOpPublisher struct{}

// Publish does nothing
func (n *NoOpPublisher) Publish(topic string, event interface{}) {
	// No-op
}

// NoOpEventBus is an event bus that does nothing (for testing)
type NoOpEventBus struct{}

// Publish does nothing
func (n *NoOpEventBus) Publish(topic string, event interface{}) {
	// No-op
}

// Subscribe does nothing
func (n *NoOpEventBus) Subscribe(topic string, handler EventHandler) {
	// No-op
}
