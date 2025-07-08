package events

import (
	"sync"
)

// CommandEventBus is an event bus for command-level internal communication.
// It is separate from the backend event bus and handles command-specific events.
// Unlike the backend EventBus, this supports unsubscribe, subscribe-once, and ID-based management.
type CommandEventBus struct {
	subscribers map[string][]subscriberInfo
	mu          sync.RWMutex
	nextID      int
}

type subscriberInfo struct {
	id      int
	handler func(interface{})
	once    bool
}

// NewCommandEventBus creates a new command-level event bus
func NewCommandEventBus() *CommandEventBus {
	return &CommandEventBus{
		subscribers: make(map[string][]subscriberInfo),
		nextID:      1,
	}
}

// Subscribe registers a handler for a specific event type.
// Returns an unsubscribe function.
func (bus *CommandEventBus) Subscribe(eventType string, handler func(interface{})) func() {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	id := bus.nextID
	bus.nextID++

	info := subscriberInfo{
		id:      id,
		handler: handler,
		once:    false,
	}

	bus.subscribers[eventType] = append(bus.subscribers[eventType], info)

	// Return unsubscribe function
	return func() {
		bus.unsubscribe(eventType, id)
	}
}

// SubscribeOnce registers a handler that will only be called once
func (bus *CommandEventBus) SubscribeOnce(eventType string, handler func(interface{})) func() {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	id := bus.nextID
	bus.nextID++

	info := subscriberInfo{
		id:      id,
		handler: handler,
		once:    true,
	}

	bus.subscribers[eventType] = append(bus.subscribers[eventType], info)

	// Return unsubscribe function
	return func() {
		bus.unsubscribe(eventType, id)
	}
}

// Emit sends an event to all subscribers of the given event type.
// Handlers are called asynchronously in separate goroutines.
func (bus *CommandEventBus) Emit(eventType string, event interface{}) {
	bus.mu.RLock()
	subscribers := bus.subscribers[eventType]
	// Make a copy to avoid holding the lock during handler execution
	handlersCopy := make([]subscriberInfo, len(subscribers))
	copy(handlersCopy, subscribers)
	bus.mu.RUnlock()

	// Track which once handlers were called
	var onceHandlerIDs []int

	// Call handlers asynchronously
	for _, sub := range handlersCopy {
		if sub.once {
			onceHandlerIDs = append(onceHandlerIDs, sub.id)
		}
		
		// Run handler in goroutine for async execution
		go sub.handler(event)
	}

	// Remove once handlers that were called
	if len(onceHandlerIDs) > 0 {
		bus.mu.Lock()
		for _, id := range onceHandlerIDs {
			bus.removeSubscriber(eventType, id)
		}
		bus.mu.Unlock()
	}
}

// Clear removes all subscribers
func (bus *CommandEventBus) Clear() {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	bus.subscribers = make(map[string][]subscriberInfo)
}

// unsubscribe removes a specific subscriber
func (bus *CommandEventBus) unsubscribe(eventType string, id int) {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	bus.removeSubscriber(eventType, id)
}

// removeSubscriber removes a subscriber by ID (must be called with lock held)
func (bus *CommandEventBus) removeSubscriber(eventType string, id int) {
	subscribers := bus.subscribers[eventType]
	
	// Find and remove the subscriber with matching ID
	for i, sub := range subscribers {
		if sub.id == id {
			// Remove by swapping with last element and truncating
			subscribers[i] = subscribers[len(subscribers)-1]
			bus.subscribers[eventType] = subscribers[:len(subscribers)-1]
			
			// If no more subscribers for this event type, remove the key
			if len(bus.subscribers[eventType]) == 0 {
				delete(bus.subscribers, eventType)
			}
			break
		}
	}
}