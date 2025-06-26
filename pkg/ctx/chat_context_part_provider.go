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
	eventChan chan events.ChatResponseEvent
	done chan bool
}

// NewChatCtxManager creates a new chat context manager
func NewChatCtxManager(eventBus events.EventBus) ChatContextPartProvider {
	manager := &InMemoryChatContextPartProvider{
		messages:  make([]Message, 0),
		eventChan: make(chan events.ChatResponseEvent, 100), // Buffered channel
		done:      make(chan bool),
	}

	// Start sequential event processor
	go manager.processEvents()

	// Subscribe to chat.response events
	eventBus.Subscribe("chat.response", func(event any) {
		if chatEvent, ok := event.(events.ChatResponseEvent); ok {
			manager.eventChan <- chatEvent
		}
	})

	return manager
}

// processEvents processes events sequentially to maintain order
func (m *InMemoryChatContextPartProvider) processEvents() {
	for {
		select {
		case chatEvent := <-m.eventChan:
			// Process event sequentially
			message := Message{
				User:      chatEvent.Message,
				Assistant: chatEvent.Response,
			}
			
			m.mu.Lock()
			m.messages = append(m.messages, message)
			m.mu.Unlock()
			
		case <-m.done:
			return
		}
	}
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
