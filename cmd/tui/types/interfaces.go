package types

// IStateAccessor is the view controllers have of chat and UI state.
type IStateAccessor interface {
	GetMessages() []Message
	// AddMessage appends a message and returns its stable ID.
	AddMessage(msg Message) int64
	ClearMessages()
	GetMessageCount() int
	// UpdateMessageByID mutates the message with the given stable ID,
	// returning false if it has been evicted.
	UpdateMessageByID(id int64, update func(*Message)) bool
	GetLastMessage() *Message

	// Confirmation state
	SetWaitingConfirmation(waiting bool)
	IsWaitingConfirmation() bool

	// UI state management
	IsContextViewerActive() bool
	SetContextViewerActive(active bool)
}
