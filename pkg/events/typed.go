package events

// Event is implemented by every event payload that knows its topic.
type Event interface {
	Topic() string
}

// SubscribeTo attaches a typed handler for T's topic, eliminating the
// type-assertion boilerplate at every subscription site. Events on the
// topic that are not of type T are ignored. It returns the unsubscribe
// function from the underlying bus.
func SubscribeTo[T Event](bus Subscriber, handler func(T)) func() {
	var zero T
	return bus.Subscribe(zero.Topic(), func(event interface{}) {
		if typed, ok := event.(T); ok {
			handler(typed)
		}
	})
}
