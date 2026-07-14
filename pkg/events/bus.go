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
	// PublishSync delivers synchronously, blocking until all handlers complete.
	PublishSync(eventType string, event interface{})
}

// Subscriber allows subscribing to events
type Subscriber interface {
	// Subscribe adds a handler for a topic and returns a function that
	// detaches it. Callers with a bounded lifetime (dialogs, per-request
	// waiters) must call the returned function to avoid handler leaks.
	Subscribe(eventType string, handler EventHandler) func()
}

// EventBus provides both publishing and subscribing
type EventBus interface {
	Publisher
	Subscriber
}

// InMemoryBus implements EventBus with in-memory storage.
//
// Delivery contract: events are delivered asynchronously, in publish
// order per topic, and are never dropped. Publish never blocks the
// caller; each topic has a dedicated worker goroutine draining an
// unbounded queue.
type InMemoryBus struct {
	mu          sync.RWMutex
	subscribers map[string][]subscriberEntry
	workers     map[string]*topicWorker
	nextID      int
}

type subscriberEntry struct {
	id      int
	handler EventHandler
}

// NewEventBus creates a new event bus.
func NewEventBus() EventBus {
	return &InMemoryBus{
		subscribers: make(map[string][]subscriberEntry),
		workers:     make(map[string]*topicWorker),
	}
}

// Subscribe adds a handler for a specific event type and returns an
// unsubscribe function.
func (b *InMemoryBus) Subscribe(eventType string, handler EventHandler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := b.nextID
	b.subscribers[eventType] = append(b.subscribers[eventType], subscriberEntry{id: id, handler: handler})

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		entries := b.subscribers[eventType]
		for i, entry := range entries {
			if entry.id == id {
				b.subscribers[eventType] = append(entries[:i], entries[i+1:]...)
				break
			}
		}
	}
}

// SubscriberCount returns the number of handlers attached to a topic.
// Useful for asserting the absence of handler leaks in tests.
func (b *InMemoryBus) SubscriberCount(eventType string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers[eventType])
}

// Publish sends an event to all subscribers of that event type.
// Events are delivered in-order per topic via a dedicated worker
// goroutine. Publishing never blocks and never drops: the per-topic
// queue grows as needed.
func (b *InMemoryBus) Publish(eventType string, event interface{}) {
	handlers := b.handlersFor(eventType)
	if len(handlers) == 0 {
		return
	}

	worker := b.getOrCreateWorker(eventType)
	worker.enqueue(eventEnvelope{event: event, handlers: handlers})
}

// PublishSync delivers an event to all subscribers synchronously on the
// caller's goroutine, blocking until all handlers complete. Use this when
// the caller must wait for handlers before proceeding (e.g. tool events).
func (b *InMemoryBus) PublishSync(eventType string, event interface{}) {
	handlers := b.handlersFor(eventType)
	for _, handler := range handlers {
		invokeHandler(handler, event)
	}
}

// Shutdown stops all topic workers after draining their queues.
// Primarily useful for tests and short-lived child buses.
func (b *InMemoryBus) Shutdown() {
	b.mu.Lock()
	workers := make([]*topicWorker, 0, len(b.workers))
	for _, w := range b.workers {
		workers = append(workers, w)
	}
	b.mu.Unlock()

	for _, w := range workers {
		w.stop()
	}
}

// handlersFor snapshots handlers for the topic.
func (b *InMemoryBus) handlersFor(eventType string) []EventHandler {
	b.mu.RLock()
	defer b.mu.RUnlock()
	entries := b.subscribers[eventType]
	if len(entries) == 0 {
		return nil
	}
	handlers := make([]EventHandler, len(entries))
	for i, entry := range entries {
		handlers[i] = entry.handler
	}
	return handlers
}

// getOrCreateWorker returns the per-topic worker, creating it if needed.
func (b *InMemoryBus) getOrCreateWorker(eventType string) *topicWorker {
	b.mu.Lock()
	defer b.mu.Unlock()

	if worker, ok := b.workers[eventType]; ok {
		return worker
	}

	worker := newTopicWorker()
	b.workers[eventType] = worker
	return worker
}

func invokeHandler(h EventHandler, e interface{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Event handler panicked: %v", r)
		}
	}()
	h(e)
}

type eventEnvelope struct {
	event    interface{}
	handlers []EventHandler
}

// topicWorker drains an unbounded FIFO queue on a dedicated goroutine,
// preserving per-topic ordering without ever blocking publishers or
// dropping events.
type topicWorker struct {
	mu       sync.Mutex
	cond     *sync.Cond
	queue    []eventEnvelope
	stopped  bool
	done     chan struct{}
	stopOnce sync.Once
}

func newTopicWorker() *topicWorker {
	w := &topicWorker{done: make(chan struct{})}
	w.cond = sync.NewCond(&w.mu)
	go w.run()
	return w
}

func (w *topicWorker) enqueue(env eventEnvelope) {
	w.mu.Lock()
	if w.stopped {
		w.mu.Unlock()
		return
	}
	w.queue = append(w.queue, env)
	w.mu.Unlock()
	w.cond.Signal()
}

func (w *topicWorker) run() {
	defer close(w.done)
	for {
		w.mu.Lock()
		for len(w.queue) == 0 && !w.stopped {
			w.cond.Wait()
		}
		if len(w.queue) == 0 && w.stopped {
			w.mu.Unlock()
			return
		}
		env := w.queue[0]
		w.queue = w.queue[1:]
		w.mu.Unlock()

		for _, handler := range env.handlers {
			invokeHandler(handler, env.event)
		}
	}
}

// stop drains the remaining queue and waits for the worker to exit.
func (w *topicWorker) stop() {
	w.stopOnce.Do(func() {
		w.mu.Lock()
		w.stopped = true
		w.mu.Unlock()
		w.cond.Signal()
		<-w.done
	})
}
