package bridge

import (
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

// recorder collects dispatched messages. Bus handlers run concurrently,
// so it locks.
type recorder struct {
	mu   sync.Mutex
	msgs []any
}

func (r *recorder) Dispatch(m any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, m)
}

func TestDispatcherDeliversInOrderThroughNext(t *testing.T) {
	d := NewDispatcher()
	d.Dispatch(uimsg.ChatChanged{})
	d.Dispatch(uimsg.Quit{})
	first := d.Next()()
	if _, ok := first.(Envelope).Msg.(uimsg.ChatChanged); !ok {
		t.Fatalf("first = %#v, want ChatChanged", first)
	}
	if _, ok := d.Next()().(Envelope).Msg.(uimsg.Quit); !ok {
		t.Fatal("second must be Quit")
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- d.Next()() }()
	select {
	case <-done:
		t.Fatal("Next must block while the queue is empty")
	case <-time.After(50 * time.Millisecond):
	}
	d.Dispatch(uimsg.Quit{})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Next did not wake on dispatch")
	}
}

func TestSubscribeTranslatesBusTopics(t *testing.T) {
	bus := events.NewCommandEventBus()
	r := &recorder{}
	Subscribe(bus, r)
	cfg := &types.Config{Theme: "nord"}
	bus.Emit("persona.changed", map[string]interface{}{"name": "engineer"})
	bus.Emit("theme.changed", map[string]interface{}{"config": cfg})
	bus.Emit("request.started", 1)
	bus.Emit("request.finished", true)
	bus.Emit("token.count", int32(42))
	bus.Emit("ui.cursor.changed", false)
	bus.Emit("ui.mouse.changed", true)
	bus.Emit("app.exit", nil)
	bus.Emit("paste.multiline", "a\nb")
	bus.Emit("vim.mode.changed", true)
	bus.Emit("persona.changed", "not a map") // ignored
	bus.WaitForPendingEvents()

	want := map[string]bool{}
	for _, m := range r.msgs {
		switch v := m.(type) {
		case uimsg.PersonaChanged:
			want["persona"] = v.Name == "engineer"
		case uimsg.ThemeChanged:
			want["theme"] = v.Config == cfg
		case uimsg.RequestStarted:
			want["started"] = v.Active == 1
		case uimsg.RequestFinished:
			want["finished"] = v.Last
		case uimsg.TokenCount:
			want["tokens"] = v.Tokens == 42
		case uimsg.CursorChanged:
			want["cursor"] = !v.Visible
		case uimsg.MouseChanged:
			want["mouse"] = v.Enabled
		case uimsg.Quit:
			want["quit"] = true
		case uimsg.OpenWrite:
			want["write"] = v.Content == "a\nb"
		case uimsg.VimModeChanged:
			want["vim"] = v.Enabled
		}
	}
	for _, k := range []string{"persona", "theme", "started", "finished", "tokens", "cursor", "mouse", "quit", "write", "vim"} {
		if !want[k] {
			t.Errorf("%s message missing or wrong; got %#v", k, r.msgs)
		}
	}
	if len(r.msgs) != 10 {
		t.Errorf("dispatched %d messages, want 10", len(r.msgs))
	}
}
