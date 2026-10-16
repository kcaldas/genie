// Package bridge connects the goroutine world of controllers and the
// command bus to the single-threaded Bubble Tea loop.
package bridge

import (
	tea "charm.land/bubbletea/v2"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

// Dispatcher implements types.Dispatcher with a buffered channel the root
// model drains through Next. It needs no reference to the program, so
// controllers may dispatch before the UI runs and the order is kept.
type Dispatcher struct {
	ch chan tea.Msg
}

// queueSize bounds how many messages may wait before Dispatch blocks the
// dispatching goroutine.
const queueSize = 1024

// NewDispatcher returns a ready dispatcher.
func NewDispatcher() *Dispatcher { return &Dispatcher{ch: make(chan tea.Msg, queueSize)} }

// Dispatch queues msg for the UI.
func (d *Dispatcher) Dispatch(msg any) { d.ch <- msg }

// Envelope wraps a dispatched message so the root model can tell it apart
// from messages sent by other means and re-arm Next exactly once.
type Envelope struct{ Msg tea.Msg }

// Next returns a command that waits for the next dispatched message. The
// root model returns it from Init and again after every Envelope.
func (d *Dispatcher) Next() tea.Cmd {
	return func() tea.Msg { return Envelope{Msg: <-d.ch} }
}

var _ types.Dispatcher = (*Dispatcher)(nil)

// Subscribe forwards the UI-relevant command-bus topics to the dispatcher
// as typed uimsg values.
func Subscribe(bus *events.CommandEventBus, d types.Dispatcher) {
	bus.Subscribe("persona.changed", func(e interface{}) {
		if data, ok := e.(map[string]interface{}); ok {
			if name, ok := data["name"].(string); ok {
				d.Dispatch(uimsg.PersonaChanged{Name: name})
			}
		}
	})
	bus.Subscribe("theme.changed", func(e interface{}) {
		if data, ok := e.(map[string]interface{}); ok {
			if cfg, ok := data["config"].(*types.Config); ok {
				d.Dispatch(uimsg.ThemeChanged{Config: cfg})
			}
		}
	})
	bus.Subscribe("request.started", func(e interface{}) {
		if n, ok := e.(int); ok {
			d.Dispatch(uimsg.RequestStarted{Active: n})
		}
	})
	bus.Subscribe("request.finished", func(e interface{}) {
		if last, ok := e.(bool); ok {
			d.Dispatch(uimsg.RequestFinished{Last: last})
		}
	})
	bus.Subscribe("token.count", func(e interface{}) {
		if n, ok := e.(int32); ok {
			d.Dispatch(uimsg.TokenCount{Tokens: n})
		}
	})
	bus.Subscribe("ui.cursor.changed", func(e interface{}) {
		if on, ok := e.(bool); ok {
			d.Dispatch(uimsg.CursorChanged{Visible: on})
		}
	})
	bus.Subscribe("ui.mouse.changed", func(e interface{}) {
		if on, ok := e.(bool); ok {
			d.Dispatch(uimsg.MouseChanged{Enabled: on})
		}
	})
	bus.Subscribe("paste.multiline", func(e interface{}) {
		if content, ok := e.(string); ok {
			d.Dispatch(uimsg.OpenWrite{Content: content})
		}
	})
	bus.Subscribe("vim.mode.changed", func(e interface{}) {
		if on, ok := e.(bool); ok {
			d.Dispatch(uimsg.VimModeChanged{Enabled: on})
		}
	})
	bus.Subscribe("app.exit", func(interface{}) {
		d.Dispatch(uimsg.Quit{})
	})
}
