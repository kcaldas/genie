package events

import (
	"log"
	"sync"
	"sync/atomic"
)

const defaultTopicBuffer = 256

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
	workers     map[string]*topicWorker
	bufferSize  int
	dropped     atomic.Int64
}

// NewEventBus creates a new event bus with the default buffer size.
func NewEventBus() EventBus {
	return NewEventBusWithBuffer(defaultTopicBuffer)
}

// NewEventBusWithBuffer allows configuring the per-topic worker queue size.
// A buffer of at least 1 is enforced to avoid unbuffered sends.
func NewEventBusWithBuffer(buffer int) EventBus {
	if buffer < 1 {
		buffer = 1
	}
	return &InMemoryBus{
		subscribers: make(map[string][]EventHandler),
		workers:     make(map[string]*topicWorker),
		bufferSize:  buffer,
	}
}

// Subscribe adds a handler for a specific event type.
func (b *InMemoryBus) Subscribe(eventType string, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscribers[eventType] = append(b.subscribers[eventType], handler)
}

// Publish sends an event to all subscribers of that event type.
// Events are delivered in-order per topic via a dedicated worker goroutine.
// Publishing is non-blocking: if the queue is full, the event is dropped.
func (b *InMemoryBus) Publish(eventType string, event interface{}) {
	handlers := b.handlersFor(eventType)
	if len(handlers) == 0 {
		return
	}

	worker := b.getOrCreateWorker(eventType)
	env := eventEnvelope{
		event:    event,
		handlers: handlers,
	}

	select {
	case worker.ch <- env:
	default:
		// Preserve non-blocking semantics; drop if the topic queue is full.
		b.dropped.Add(1)
		log.Printf("Event bus queue full for topic %s; dropping event", eventType)
	}
}

// DroppedCount returns the number of events dropped due to full queues.
func (b *InMemoryBus) DroppedCount() int64 {
	return b.dropped.Load()
}

// Shutdown stops all topic workers. Primarily useful for tests.
func (b *InMemoryBus) Shutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, w := range b.workers {
		w.stop()
	}
}

// handlersFor snapshots handlers for the topic.
func (b *InMemoryBus) handlersFor(eventType string) []EventHandler {
	b.mu.RLock()
	defer b.mu.RUnlock()
	handlers := make([]EventHandler, len(b.subscribers[eventType]))
	copy(handlers, b.subscribers[eventType])
	return handlers
}

// getOrCreateWorker returns the per-topic worker, creating it if needed.
func (b *InMemoryBus) getOrCreateWorker(eventType string) *topicWorker {
	b.mu.Lock()
	defer b.mu.Unlock()

	if worker, ok := b.workers[eventType]; ok {
		return worker
	}

	worker := newTopicWorker(b.bufferSize)
	b.workers[eventType] = worker
	return worker
}

type eventEnvelope struct {
	event    interface{}
	handlers []EventHandler
}

type topicWorker struct {
	ch       chan eventEnvelope
	wg       sync.WaitGroup
	stopOnce sync.Once
}

func newTopicWorker(buffer int) *topicWorker {
	w := &topicWorker{
		ch: make(chan eventEnvelope, buffer),
	}
	w.wg.Add(1)
	go w.run()
	return w
}

func (w *topicWorker) run() {
	defer w.wg.Done()
	for env := range w.ch {
		for _, handler := range env.handlers {
			func(h EventHandler, e interface{}) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Event handler panicked: %v", r)
					}
				}()
				h(e)
			}(handler, env.event)
		}
	}
}

func (w *topicWorker) stop() {
	w.stopOnce.Do(func() {
		close(w.ch)
		w.wg.Wait()
	})
}
