package input

import (
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/history"
	"github.com/kcaldas/genie/cmd/tui/ui/editor"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

type fakeClipboard string

func (c fakeClipboard) Paste() (string, error) { return string(c), nil }

type suggester []string

func (s suggester) Suggest(in string) string {
	for _, c := range s {
		if in != "" && strings.HasPrefix(c, in) {
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
	for _, topic := range []string{uimsg.TopicInputText, uimsg.TopicInputCommand, uimsg.TopicInputSlashCommand, uimsg.TopicInputCancel} {
		topic := topic
		bus.Subscribe(topic, func(e interface{}) {
			got.mu.Lock()
			defer got.mu.Unlock()
			got.byTo[topic] = append(got.byTo[topic], e.(string))
		})
	}
	prompt := editor.NewPrompt(suggester{":write", ":yank"}, history.NewChatHistory("", false))
	m := New(prompt, bus, fakeClipboard("from clip"))
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
	case "shift+enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift}
	case "alt+enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt}
	case "ctrl+j":
		return tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "f4":
		return tea.KeyPressMsg{Code: tea.KeyF4}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "ctrl+v":
		return tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl}
	case "ctrl+shift+v":
		return tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl | tea.ModShift}
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
	if got := got.of(uimsg.TopicInputText); len(got) != 1 || got[0] != "hello there" {
		t.Fatalf("text events = %v", got)
	}
	if got := got.of(uimsg.TopicInputCommand); len(got) != 1 || got[0] != ":help" {
		t.Fatalf("command events = %v", got)
	}
	if got := got.of(uimsg.TopicInputSlashCommand); len(got) != 1 || got[0] != "/compact now" {
		t.Fatalf("slash events = %v", got)
	}
	if m.Text() != "" {
		t.Fatalf("prompt not cleared: %q", m.Text())
	}
}

func TestNewlineKeysAndMultiLineSubmit(t *testing.T) {
	m, bus, got := newTestModel(t)
	typeText(m, "one")
	press(m, "shift+enter")
	typeText(m, "two")
	press(m, "alt+enter")
	typeText(m, "three")
	press(m, "ctrl+j")
	typeText(m, "four")
	if m.Rows() != 4 {
		t.Fatalf("rows = %d, want 4", m.Rows())
	}
	press(m, "enter")
	bus.WaitForPendingEvents()
	if got := got.of(uimsg.TopicInputText); len(got) != 1 || got[0] != "one\ntwo\nthree\nfour" {
		t.Fatalf("multi-line submit = %q", got)
	}
	if m.Rows() != 1 {
		t.Fatal("prompt shrinks back after sending")
	}
}

func TestUpDownAreHistoryOnlyAtTheEdges(t *testing.T) {
	m, bus, _ := newTestModel(t)
	typeText(m, "earlier")
	press(m, "enter")
	bus.WaitForPendingEvents()
	typeText(m, "a")
	press(m, "shift+enter")
	typeText(m, "b")
	press(m, "shift+enter")
	typeText(m, "c")
	press(m, "up") // from the last line: moves up, no history
	if m.Text() != "a\nb\nc" {
		t.Fatalf("up in the middle replaced the text: %q", m.Text())
	}
	press(m, "up", "up") // now on the first line: history
	if m.Text() != "earlier" {
		t.Fatalf("up on the first line = %q", m.Text())
	}
	press(m, "down")
	if m.Text() != "a\nb\nc" {
		t.Fatalf("down restores the draft: %q", m.Text())
	}
}

func TestEscEmitsCancel(t *testing.T) {
	m, bus, got := newTestModel(t)
	press(m, "esc")
	bus.WaitForPendingEvents()
	if len(got.of(uimsg.TopicInputCancel)) != 1 {
		t.Fatalf("cancel events = %v", got.of(uimsg.TopicInputCancel))
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
	if v := ansi.Strip(m.View(th)); v != ":write" {
		t.Fatalf("ghost text missing from view %q", v)
	}
	press(m, "tab")
	m.View(th)
	if x, y := m.CursorPos(); m.Text() != ":write" || x != 6 || y != 0 {
		t.Fatalf("tab: text=%q cursor=(%d,%d)", m.Text(), x, y)
	}
}

func TestLargePasteBecomesAChip(t *testing.T) {
	m, bus, got := newTestModel(t)
	typeText(m, "look: ")
	m.Update(tea.PasteMsg{Content: "l1\nl2\nl3\nl4"})
	th := style.New("default", 1)
	view := ansi.Strip(m.View(th))
	if view != "look: [Pasted 4 lines]" {
		t.Fatalf("view = %q", view)
	}
	if m.Rows() != 1 {
		t.Fatal("a chip takes one row")
	}
	press(m, "enter")
	bus.WaitForPendingEvents()
	if got := got.of(uimsg.TopicInputText); len(got) != 1 || got[0] != "look: l1\nl2\nl3\nl4" {
		t.Fatalf("chip not expanded on send: %q", got)
	}
}

func TestSmallPasteAndClipboardKeys(t *testing.T) {
	m, _, _ := newTestModel(t)
	m.Update(tea.PasteMsg{Content: "pasted text"})
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

func TestF4AndForcedPasteOpenWriteMode(t *testing.T) {
	m, _, _ := newTestModel(t)
	typeText(m, "draft")
	cmd := press(m, "f4")
	if msg, ok := cmd().(uimsg.OpenWrite); !ok || msg.Content != "draft" || m.Text() != "" {
		t.Fatalf("F4 = %#v, text=%q", cmd(), m.Text())
	}
	typeText(m, "note ")
	cmd = press(m, "ctrl+shift+v")
	if msg, ok := cmd().(uimsg.OpenWrite); !ok || msg.Content != "note from clip" {
		t.Fatalf("ctrl+shift+v = %#v", cmd())
	}
}

func TestPromptScrollsPastMaxRows(t *testing.T) {
	m, _, _ := newTestModel(t)
	for i := 0; i < MaxRows+3; i++ {
		typeText(m, "x")
		press(m, "shift+enter")
	}
	if m.Rows() != MaxRows {
		t.Fatalf("rows capped at %d, got %d", MaxRows, m.Rows())
	}
	th := style.New("default", 1)
	view := m.View(th)
	if n := strings.Count(view, "\n") + 1; n != MaxRows {
		t.Fatalf("view has %d rows", n)
	}
	if _, y := m.CursorPos(); y != MaxRows-1 {
		t.Fatalf("cursor row = %d, want the last visible row", y)
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
