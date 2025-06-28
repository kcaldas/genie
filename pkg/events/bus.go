package events

import (
	"log"
	"sync"
)

// EventHandler is a function that handles an event
type EventHandler func(event interface{})

// Publisher allows publishing events
type Publisher interface {
	Publish(eventType string, event interface{})
}

// Subscriber allows subscribing to events
type Subscriber interface {
	Subscribe(eventType string, handler EventHandler)
}

// EventBus provides both publishing and subscribing
type EventBus interface {
	Publisher
	Subscriber
}

// InMemoryBus implements EventBus with in-memory storage
type InMemoryBus struct {
	mu          sync.RWMutex
	subscribers map[string][]EventHandler
}

// NewEventBus creates a new event bus
func NewEventBus() EventBus {
	return &InMemoryBus{
		subscribers: make(map[string][]EventHandler),
	}
}

// Subscribe adds a handler for a specific event type
func (b *InMemoryBus) Subscribe(eventType string, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscribers[eventType] = append(b.subscribers[eventType], handler)
}

// Publish sends an event to all subscribers of that event type
func (b *InMemoryBus) Publish(eventType string, event interface{}) {
	b.mu.RLock()
	handlers := make([]EventHandler, len(b.subscribers[eventType]))
	copy(handlers, b.subscribers[eventType])
	b.mu.RUnlock()

	// Call all handlers asynchronously with panic recovery
	for _, handler := range handlers {
		go func(h EventHandler, e interface{}) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Event handler panicked: %v", r)
				}
			}()
			h(e)
		}(handler, event)
	}
}
