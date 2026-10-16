package types

// Dispatcher is the one-way channel from controllers, which run on
// event-bus goroutines, to the UI, which owns all rendering state. A
// controller never touches the UI directly; it dispatches a message
// describing what changed and the UI applies it on its own loop.
//
// Messages are the plain structs in cmd/tui/ui/uimsg.
type Dispatcher interface {
	Dispatch(msg any)
}
