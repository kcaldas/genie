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

// translators maps a bus topic to the typed message it becomes. A
// translator returns false for a payload of the wrong shape, which is
// then ignored.
var translators = map[string]func(e interface{}) (tea.Msg, bool){
	uimsg.TopicPersonaChanged: func(e interface{}) (tea.Msg, bool) {
		name, ok := mapField[string](e, "name")
		return uimsg.PersonaChanged{Name: name}, ok
	},
	uimsg.TopicModelChanged: func(e interface{}) (tea.Msg, bool) {
		provider, ok := mapField[string](e, "provider")
		model, ok2 := mapField[string](e, "model")
		return uimsg.ModelChanged{Provider: provider, Model: model}, ok && ok2
	},
	uimsg.TopicPaletteOpen: func(e interface{}) (tea.Msg, bool) {
		kind, ok := e.(string)
		return uimsg.OpenPalette{Kind: kind}, ok
	},
	uimsg.TopicThemeChanged: func(e interface{}) (tea.Msg, bool) {
		cfg, ok := mapField[*types.Config](e, "config")
		return uimsg.ThemeChanged{Config: cfg}, ok
	},
	uimsg.TopicRequestStarted: func(e interface{}) (tea.Msg, bool) {
		n, ok := e.(int)
		return uimsg.RequestStarted{Active: n}, ok
	},
	uimsg.TopicRequestFinished: func(e interface{}) (tea.Msg, bool) {
		last, ok := e.(bool)
		return uimsg.RequestFinished{Last: last}, ok
	},
	uimsg.TopicTokenCount: func(e interface{}) (tea.Msg, bool) {
		n, ok := e.(int32)
		return uimsg.TokenCount{Tokens: n}, ok
	},
	uimsg.TopicCursorChanged: func(e interface{}) (tea.Msg, bool) {
		on, ok := e.(bool)
		return uimsg.CursorChanged{Visible: on}, ok
	},
	uimsg.TopicMouseChanged: func(e interface{}) (tea.Msg, bool) {
		on, ok := e.(bool)
		return uimsg.MouseChanged{Enabled: on}, ok
	},
	uimsg.TopicPasteMultiline: func(e interface{}) (tea.Msg, bool) {
		content, ok := e.(string)
		return uimsg.ExpandPrompt{Content: content}, ok
	},
	uimsg.TopicVimModeChanged: func(e interface{}) (tea.Msg, bool) {
		on, ok := e.(bool)
		return uimsg.VimModeChanged{Enabled: on}, ok
	},
	uimsg.TopicAppExit: func(interface{}) (tea.Msg, bool) {
		return uimsg.Quit{}, true
	},
}

// mapField reads a typed value out of a map[string]interface{} payload.
func mapField[T any](e interface{}, key string) (T, bool) {
	var zero T
	data, ok := e.(map[string]interface{})
	if !ok {
		return zero, false
	}
	v, ok := data[key].(T)
	return v, ok
}

// Subscribe forwards the UI-relevant command-bus topics to the dispatcher
// as typed uimsg values.
func Subscribe(bus *events.CommandEventBus, d types.Dispatcher) {
	for topic, translate := range translators {
		translate := translate
		bus.Subscribe(topic, func(e interface{}) {
			if msg, ok := translate(e); ok {
				d.Dispatch(msg)
			}
		})
	}
}
