package input

import (
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/history"
	"github.com/kcaldas/genie/cmd/tui/ui/editor"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
)

type fakeClipboard string

func (c fakeClipboard) Paste() (string, error) { return string(c), nil }

type suggester []string

func (s suggester) Suggest(in string) string {
	for _, c := range s {
		if in != "" && len(c) >= len(in) && c[:len(in)] == in {
			return c
		}
	}
	return ""
}

// emitted records bus events per topic. Handlers run on their own
// goroutines, so it locks.
type emitted struct {
	mu   sync.Mutex
	byTo map[string][]string
}

func (e *emitted) of(topic string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.byTo[topic]...)
}

func newTestModel(t *testing.T) (*Model, *events.CommandEventBus, *emitted) {
	t.Helper()
	bus := events.NewCommandEventBus()
	got := &emitted{byTo: map[string][]string{}}
	for _, topic := range []string{TopicText, TopicCommand, TopicSlashCommand, TopicCancel} {
		topic := topic
		bus.Subscribe(topic, func(e interface{}) {
			got.mu.Lock()
			defer got.mu.Unlock()
			got.byTo[topic] = append(got.byTo[topic], e.(string))
		})
	}
	line := editor.NewLine(suggester{":write", ":yank"}, history.NewChatHistory("", false))
	m := New(line, bus, fakeClipboard("from clip"))
	m.SetWidth(40)
	return m, bus, got
}

func press(m *Model, keys ...string) tea.Cmd {
	var last tea.Cmd
	for _, k := range keys {
		last = m.Update(keyFor(k))
	}
	return last
}

func keyFor(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "ctrl+v":
		return tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl}
	case "ctrl+w":
		return tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func typeText(m *Model, s string) {
	for _, r := range s {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func TestSubmitRoutesByPrefix(t *testing.T) {
	m, bus, got := newTestModel(t)
	typeText(m, "hello there")
	press(m, "enter")
	typeText(m, ":help")
	press(m, "enter")
	typeText(m, "/compact now")
	press(m, "enter")
	press(m, "enter") // empty: nothing emitted
	bus.WaitForPendingEvents()
	if len(got.of(TopicText)) != 1 || got.of(TopicText)[0] != "hello there" {
		t.Fatalf("text events = %v", got.of(TopicText))
	}
	if len(got.of(TopicCommand)) != 1 || got.of(TopicCommand)[0] != ":help" {
		t.Fatalf("command events = %v", got.of(TopicCommand))
	}
	if len(got.of(TopicSlashCommand)) != 1 || got.of(TopicSlashCommand)[0] != "/compact now" {
		t.Fatalf("slash events = %v", got.of(TopicSlashCommand))
	}
	if m.Text() != "" {
		t.Fatalf("prompt not cleared: %q", m.Text())
	}
}

func TestEscEmitsCancel(t *testing.T) {
	m, bus, got := newTestModel(t)
	press(m, "esc")
	bus.WaitForPendingEvents()
	if len(got.of(TopicCancel)) != 1 {
		t.Fatalf("cancel events = %v", got.of(TopicCancel))
	}
}

func TestCtrlCQuitsOnlyWhenEmpty(t *testing.T) {
	m, _, _ := newTestModel(t)
	typeText(m, "draft")
	if cmd := press(m, "ctrl+c"); cmd != nil || m.Text() != "" {
		t.Fatalf("ctrl+c with text should clear, not quit: cmd=%v text=%q", cmd, m.Text())
	}
	cmd := press(m, "ctrl+c")
	if cmd == nil {
		t.Fatal("ctrl+c on empty prompt should return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestTabCompletesAndGhostRenders(t *testing.T) {
	m, _, _ := newTestModel(t)
	typeText(m, ":w")
	th := style.New("default", 1)
	if v := m.View(th); v == ":w" {
		t.Fatalf("ghost text missing from view %q", v)
	}
	press(m, "tab")
	if m.Text() != ":write" || m.CursorX() != 6 {
		t.Fatalf("tab: text=%q cursor=%d", m.Text(), m.CursorX())
	}
}

func TestHistoryKeys(t *testing.T) {
	m, bus, _ := newTestModel(t)
	typeText(m, "first")
	press(m, "enter")
	bus.WaitForPendingEvents()
	press(m, "up")
	if m.Text() != "first" {
		t.Fatalf("up = %q", m.Text())
	}
	press(m, "down")
	if m.Text() != "" {
		t.Fatalf("down = %q", m.Text())
	}
}

func TestPasteMessageAndClipboardKeys(t *testing.T) {
	m, _, _ := newTestModel(t)
	m.Update(tea.PasteMsg{Content: "pasted\ntext"})
	if m.Text() != "pasted text" {
		t.Fatalf("bracketed paste = %q", m.Text())
	}
	press(m, "ctrl+c")
	press(m, "ctrl+v")
	if m.Text() != "from clip" {
		t.Fatalf("clipboard paste = %q", m.Text())
	}
	press(m, "ctrl+w")
	if m.Text() != "from " {
		t.Fatalf("ctrl+w = %q", m.Text())
	}
	press(m, "backspace")
	if m.Text() != "from" {
		t.Fatalf("backspace = %q", m.Text())
	}
}

func TestUnknownSpecialKeysAreIgnored(t *testing.T) {
	m, _, _ := newTestModel(t)
	m.Update(tea.KeyPressMsg{Code: tea.KeyF1})
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	if m.Text() != "" {
		t.Fatalf("special keys typed text: %q", m.Text())
	}
}
