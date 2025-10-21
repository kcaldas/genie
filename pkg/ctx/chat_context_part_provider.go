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
	SeedHistory(history []Message)
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
			manager.addMessage(chatEvent.Message, chatEvent.Response)
		}
	})

	return manager
}

func (p *InMemoryChatContextPartProvider) addMessage(user, assistant string) {
	message := Message{}

	if user != "" {
		message.User = user
	}

	if assistant != "" {
		message.Assistant = assistant
	}

	p.mu.Lock()
	p.messages = append(p.messages, message)
	p.mu.Unlock()
}

// GetPart returns the formatted conversation context
func (m *InMemoryChatContextPartProvider) GetPart(ctx context.Context) (ContextPart, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var parts []string

	for _, msg := range m.messages {
		if msg.User != "" {
			parts = append(parts, "User: "+msg.User)
		}
		if msg.Assistant != "" {
			parts = append(parts, "Assistant: "+msg.Assistant)
		}
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

// SeedHistory replaces the current chat history with the provided messages.
func (m *InMemoryChatContextPartProvider) SeedHistory(history []Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(history) == 0 {
		m.messages = make([]Message, 0)
		return
	}

	m.messages = make([]Message, 0, len(history))
	for _, msg := range history {
		if msg.User == "" && msg.Assistant == "" {
			continue
		}
		m.messages = append(m.messages, Message{
			User:      msg.User,
			Assistant: msg.Assistant,
		})
	}
}
