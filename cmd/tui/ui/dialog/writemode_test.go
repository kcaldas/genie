package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/history"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
)

type fakeBus struct{ topic, data string }

func (b *fakeBus) Emit(topic string, data interface{}) {
	b.topic, _ = topic, data
	b.data = data.(string)
}

func typeInto(w *WriteMode, s string) {
	for _, r := range s {
		w.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func TestWriteModeSubmitsAndRecordsHistory(t *testing.T) {
	bus := &fakeBus{}
	hist := history.NewChatHistory("", false)
	w := NewWriteMode("", false, bus, hist)
	w.SetSize(60, 12)
	typeInto(w, "first line")
	w.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	typeInto(w, "second")
	_, done := w.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if !done || bus.topic != "user.input.text" || bus.data != "first line\nsecond" {
		t.Fatalf("submit: done=%v topic=%q data=%q", done, bus.topic, bus.data)
	}
	if h := hist.GetHistory(); len(h) != 1 || h[0] != "first line\nsecond" {
		t.Fatalf("history = %v", h)
	}
}

func TestWriteModeEscCancelsWithoutSending(t *testing.T) {
	bus := &fakeBus{}
	w := NewWriteMode("draft", false, bus, nil)
	_, done := w.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !done || bus.topic != "" {
		t.Fatal("esc must close without emitting")
	}
}

func TestWriteModeViewTitleAndWrap(t *testing.T) {
	w := NewWriteMode(strings.Repeat("x", 25)+"\nend", true, &fakeBus{}, nil)
	w.SetSize(14, 8) // inner width 12: the 25 x's wrap to 3 rows
	th := style.New("default", 1)
	out := ansi.Strip(w.View(th))
	lines := strings.Split(out, "\n")
	if len(lines) != 7 {
		t.Fatalf("frame leaves one row for the status bar: got %d rows", len(lines))
	}
	if !strings.Contains(lines[0], "[INSERT]") {
		t.Fatalf("title = %q", lines[0])
	}
	if lines[1] != "│xxxxxxxxxxxx│" || lines[3] != "│x           │" || lines[4] != "│end         │" {
		t.Fatalf("wrapped rows:\n%s", out)
	}
	c := w.Cursor()
	if c == nil || c.X != 1+3 || c.Y != 1+3 || c.Shape != tea.CursorBar {
		t.Fatalf("cursor = %+v", c)
	}
	w.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(ansi.Strip(w.View(th)), "[NORMAL]") || w.Cursor().Shape != tea.CursorBlock {
		t.Fatal("normal mode shows in the title with a block cursor")
	}
	w.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	typeInto(w, "q")
	if !strings.Contains(ansi.Strip(w.View(th)), " :q ") {
		t.Fatal("command line shows in the title")
	}
	if _, done := w.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); !done {
		t.Fatal(":q closes")
	}
}

func TestWriteModeScrollsToKeepCursorVisible(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		sb.WriteString("line\n")
	}
	w := NewWriteMode(sb.String(), false, &fakeBus{}, nil)
	w.SetSize(20, 8) // inner height 5, cursor on row 30
	th := style.New("default", 1)
	w.View(th)
	if c := w.Cursor(); c.Y != 1+4 {
		t.Fatalf("cursor row = %d, want bottom row 5", c.Y)
	}
	for i := 0; i < 30; i++ {
		w.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	}
	w.View(th)
	if w.offsetY != 0 || w.Cursor().Y != 1 {
		t.Fatalf("scrolled to top: offset=%d cursorY=%d", w.offsetY, w.Cursor().Y)
	}
}

func TestWriteModePasteAndVimSwitch(t *testing.T) {
	w := NewWriteMode("", false, &fakeBus{}, nil)
	w.Update(tea.PasteMsg{Content: "a\nb"})
	if w.Text() != "a\nb" {
		t.Fatalf("paste = %q", w.Text())
	}
	w.SetVim(true)
	if w.ModeLabel() != "[INSERT]" || !w.ShowsStatus() || !w.Fullscreen() {
		t.Fatal("vim on, fullscreen with status")
	}
}
