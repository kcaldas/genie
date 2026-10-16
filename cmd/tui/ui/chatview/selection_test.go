package chatview

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

func selModel(t *testing.T) *Model {
	t.Helper()
	chat := state.NewChatState(100)
	m := New(chat, &countingFormatter{})
	m.SetSize(40, 10)
	chat.AddMessage(types.Message{Role: "user", Content: "hello wide world"})
	chat.AddMessage(types.Message{Role: "assistant", Content: "日本語 text here"})
	m.Update(uimsg.ChatChanged{})
	flush(m)
	return m
}

func TestDragSelectionAcrossLines(t *testing.T) {
	m := selModel(t)
	// line 0: "user: hello wide world", line 1: "", line 2: "assistant: 日本語 text here"
	m.SelectStart(0, 6)
	m.SelectExtend(2, 17) // through "日本語"
	got := m.SelectedText()
	if got != "hello wide world\n\nassistant: 日本語" {
		t.Fatalf("selected = %q", got)
	}
	v := m.View()
	if !strings.Contains(v, "\x1b[7mhello wide world\x1b[27m") {
		t.Fatalf("selection not highlighted:\n%q", v)
	}
	if !strings.Contains(ansi.Strip(v), "user: hello wide world") {
		t.Fatal("highlighting must not change the text")
	}
	m.ClearSelection()
	if m.HasSelection() || m.SelectedText() != "" || strings.Contains(m.View(), "\x1b[7m") {
		t.Fatal("clear removes selection and highlight")
	}
}

func TestBackwardsDragAndEmptySelection(t *testing.T) {
	m := selModel(t)
	m.SelectStart(0, 11)
	m.SelectExtend(0, 6)
	if got := m.SelectedText(); got != "hello" {
		t.Fatalf("backwards drag = %q", got)
	}
	m.SelectStart(0, 3)
	if m.SelectedText() != "" {
		t.Fatal("a click without drag selects nothing")
	}
}

func TestWordAndLineSelection(t *testing.T) {
	m := selModel(t)
	m.SelectWord(0, 8) // inside "hello"
	if got := m.SelectedText(); got != "hello" {
		t.Fatalf("word = %q", got)
	}
	m.SelectWord(2, 13) // inside 日本語 (cells 11-16)
	if got := m.SelectedText(); got != "日本語" {
		t.Fatalf("wide word = %q", got)
	}
	m.SelectWord(1, 0) // empty line
	if m.SelectedText() != "" {
		t.Fatal("word on an empty line is empty")
	}
	m.SelectLine(2)
	if got := m.SelectedText(); got != "assistant: 日本語 text here" {
		t.Fatalf("line = %q", got)
	}
}

func TestSelectionFollowsScrollOffset(t *testing.T) {
	chat := state.NewChatState(100)
	m := New(chat, &countingFormatter{})
	m.SetSize(40, 3)
	for i := 0; i < 10; i++ {
		chat.AddMessage(types.Message{Role: "user", Content: "line" + string(rune('A'+i))})
	}
	m.Update(uimsg.ChatChanged{})
	flush(m)
	m.Top()
	m.ScrollBy(4) // first visible absolute line is 4: "user: lineC"
	m.SelectLine(0)
	if got := m.SelectedText(); got != "user: lineC" {
		t.Fatalf("selection must use absolute lines: %q", got)
	}
	m.SelectStart(99, 0) // beyond the end clamps
	if !m.HasSelection() {
		t.Fatal("clamped start still selects")
	}
}
