package genie

import "context"

// Genie is the core AI assistant interface
type Genie interface {
	// Chat operations - async, response via events
	Chat(ctx context.Context, sessionID string, message string) error
	
	// Session management
	CreateSession() (string, error)
	GetSession(sessionID string) (*Session, error)
}

// Session represents a conversation session
type Session struct {
	ID           string
	CreatedAt    string
	Interactions []Interaction
}

// Interaction represents a single message-response pair
type Interaction struct {
	Message  string
	Response string
	Time     string
}