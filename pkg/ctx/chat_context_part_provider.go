package ctx

import (
	"context"
	"strings"

	"github.com/kcaldas/genie/pkg/events"
)

// Message represents a user/assistant conversation pair
type Message struct {
	User      string
	Assistant string
}

// ChatContextPartProvider manages chat context for a single session
type ChatContextPartProvider interface {
	ContextPartProvider
}

// InMemoryChatContextPartProvider implements ChatCtxManager with in-memory storage
type InMemoryChatContextPartProvider struct {
	messages []Message
}

// NewChatCtxManager creates a new chat context manager
func NewChatCtxManager(eventBus events.EventBus) ChatContextPartProvider {
	manager := &InMemoryChatContextPartProvider{
		messages: make([]Message, 0),
	}

	// Subscribe to chat.response events
	eventBus.Subscribe("chat.response", manager.handleChatResponseEvent)

	return manager
}

// handleChatResponseEvent handles chat response events from the event bus
func (m *InMemoryChatContextPartProvider) handleChatResponseEvent(event any) {
	if chatEvent, ok := event.(events.ChatResponseEvent); ok {
		// Create a new message pair
		message := Message{
			User:      chatEvent.Message,
			Assistant: chatEvent.Response,
		}
		m.messages = append(m.messages, message)
	}
}

// GetPart returns the formatted conversation context
func (m *InMemoryChatContextPartProvider) GetPart(ctx context.Context) (ContextPart, error) {
	var parts []string

	for _, msg := range m.messages {
		parts = append(parts, "User: "+msg.User)
		parts = append(parts, "Genie: "+msg.Assistant)
	}

	return ContextPart{
		Key:     "chat",
		Content: strings.Join(parts, "\n"),
	}, nil
}

// ClearPart removes all context
func (m *InMemoryChatContextPartProvider) ClearPart() error {
	m.messages = make([]Message, 0)
	return nil
}
