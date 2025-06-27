package ctx

import (
	"context"
	"strings"
	"sync"

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
	mu       sync.RWMutex
	messages []Message
}

// NewChatCtxManager creates a new chat context manager
func NewChatCtxManager(eventBus events.EventBus) ChatContextPartProvider {
	manager := &InMemoryChatContextPartProvider{
		messages: make([]Message, 0),
	}

	// Subscribe to chat.response events with direct processing
	eventBus.Subscribe("chat.response", func(event any) {
		if chatEvent, ok := event.(events.ChatResponseEvent); ok {
			message := Message{
				User:      chatEvent.Message,
				Assistant: chatEvent.Response,
			}
			
			manager.mu.Lock()
			manager.messages = append(manager.messages, message)
			manager.mu.Unlock()
		}
	})

	return manager
}


// GetPart returns the formatted conversation context
func (m *InMemoryChatContextPartProvider) GetPart(ctx context.Context) (ContextPart, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
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
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.messages = make([]Message, 0)
	return nil
}
