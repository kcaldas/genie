package editor

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func k(s string) tea.KeyPressMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "ctrl+l":
		return tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl}
	case "alt+left":
		return tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt}
	case "alt+right":
		return tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModAlt}
	}
	r := []rune(s)
	return tea.KeyPressMsg{Code: r[0], Text: s}
}

// keys feeds a sequence; a token longer than one rune is a named key.
func keys(m *Multi, seq ...string) Action {
	last := ActionNone
	for _, s := range seq {
		last = m.Key(k(s))
	}
	return last
}

func typed(m *Multi, s string) {
	for _, r := range s {
		m.Key(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func wantText(t *testing.T, m *Multi, text string) {
	t.Helper()
	if m.Text() != text {
		t.Fatalf("text = %q, want %q", m.Text(), text)
	}
}

func wantCursor(t *testing.T, m *Multi, row, col int) {
	t.Helper()
	r, c := m.Buf.Cursor()
	if r != row || c != col {
		t.Fatalf("cursor = (%d,%d), want (%d,%d) in %q", r, c, row, col, m.Text())
	}
}

func TestPlainEditorEditsAndEscCloses(t *testing.T) {
	m := NewMulti("", false)
	typed(m, "hello")
	keys(m, "enter")
	typed(m, "world")
	wantText(t, m, "hello\nworld")
	keys(m, "alt+left")
	wantCursor(t, m, 1, 0)
	keys(m, "alt+right")
	wantCursor(t, m, 1, 5)
	keys(m, "up")
	wantCursor(t, m, 0, 5)
	keys(m, "left", "left")
	keys(m, "backspace")
	wantText(t, m, "helo\nworld")
	if m.ModeLabel() != "" {
		t.Fatal("plain editor shows no mode")
	}
	if keys(m, "esc") != ActionClose {
		t.Fatal("esc closes the plain editor")
	}
	if keys(m, "ctrl+s") != ActionSubmit {
		t.Fatal("ctrl+s submits")
	}
	m.Buf.Clear()
	if keys(m, "ctrl+s") != ActionClose {
		t.Fatal("ctrl+s on empty text closes")
	}
}

func TestClearKeysClearThenClose(t *testing.T) {
	m := NewMulti("draft", false)
	if keys(m, "ctrl+c") != ActionNone || m.Text() != "" {
		t.Fatal("ctrl+c with text clears")
	}
	if keys(m, "ctrl+l") != ActionClose {
		t.Fatal("ctrl+l on empty text closes")
	}
	v := NewMulti("", true)
	if keys(v, "ctrl+c") != ActionNone || v.Mode() != ModeNormal {
		t.Fatal("in vim, ctrl+c on empty insert goes to normal first")
	}
	if keys(v, "ctrl+c") != ActionClose {
		t.Fatal("ctrl+c again in normal mode closes")
	}
}

func TestViModeSwitching(t *testing.T) {
	m := NewMulti("", true)
	if m.Mode() != ModeInsert || m.ModeLabel() != "[INSERT]" {
		t.Fatal("vim starts in insert mode")
	}
	typed(m, "abc")
	keys(m, "esc")
	if m.Mode() != ModeNormal || m.ModeLabel() != "[NORMAL]" {
		t.Fatal("esc enters normal mode")
	}
	wantCursor(t, m, 0, 2) // esc steps back one, like vi
	if keys(m, "esc") != ActionNone {
		t.Fatal("esc in normal mode never closes")
	}
	keys(m, "i")
	if m.Mode() != ModeInsert {
		t.Fatal("i enters insert")
	}
	keys(m, "esc", "A")
	wantCursor(t, m, 0, 3)
	keys(m, "esc", "0", "a")
	wantCursor(t, m, 0, 1)
	keys(m, "esc", "o")
	typed(m, "second")
	wantText(t, m, "abc\nsecond")
	keys(m, "esc", "O")
	typed(m, "middle")
	wantText(t, m, "abc\nmiddle\nsecond")
	wantCursor(t, m, 1, 6)
}

func TestViMotions(t *testing.T) {
	m := NewMulti("foo.bar baz\n  qux", true)
	keys(m, "esc")
	keys(m, "g", "g")
	wantCursor(t, m, 0, 0)
	keys(m, "w")
	wantCursor(t, m, 0, 3) // the '.'
	keys(m, "w")
	wantCursor(t, m, 0, 4) // bar
	keys(m, "W")
	wantCursor(t, m, 0, 8) // baz
	keys(m, "w")
	wantCursor(t, m, 1, 2) // first word of the next line
	keys(m, "b")
	wantCursor(t, m, 0, 8)
	keys(m, "B")
	wantCursor(t, m, 0, 0) // "foo.bar" is one WORD
	keys(m, "$")
	wantCursor(t, m, 0, 10)
	keys(m, "h", "h")
	wantCursor(t, m, 0, 8)
	keys(m, "l")
	wantCursor(t, m, 0, 9)
	keys(m, "j")
	wantCursor(t, m, 1, 5)
	keys(m, "k")
	wantCursor(t, m, 0, 5)
	keys(m, "G")
	wantCursor(t, m, 1, 0)
	keys(m, "enter")
	wantCursor(t, m, 1, 0) // no next line
	keys(m, "g", "x")      // pending g cancelled by another key
	if m.pending != opNone {
		t.Fatal("unrecognised key cancels pending g")
	}
}

func TestViDeleteAndChange(t *testing.T) {
	m := NewMulti("one two three\nfour", true)
	keys(m, "esc", "g", "g", "w")
	keys(m, "x")
	wantText(t, m, "one wo three\nfour")
	keys(m, "d", "$")
	wantText(t, m, "one \nfour")
	keys(m, "$", "d", "0") // d0 keeps the character under the cursor, as vi does
	wantText(t, m, " \nfour")
	keys(m, "d", "d")
	wantText(t, m, "four")
	keys(m, "d", "x") // d then a non-motion cancels
	wantText(t, m, "four")
	if m.pending != opNone {
		t.Fatal("pending command must clear")
	}
	keys(m, "l", "l", "c", "$")
	if m.Mode() != ModeInsert {
		t.Fatal("c$ enters insert mode")
	}
	typed(m, "x")
	wantText(t, m, "fox")
	keys(m, "esc", "c", "0")
	typed(m, "y")
	wantText(t, m, "yx")
	keys(m, "esc", "d", "d")
	wantText(t, m, "")
	keys(m, "d", "d")
	wantText(t, m, "")
}

func TestViCommandLine(t *testing.T) {
	m := NewMulti("message", true)
	keys(m, "esc", ":")
	if m.Mode() != ModeCommand || m.ModeLabel() != ":" {
		t.Fatalf("':' enters command mode, label %q", m.ModeLabel())
	}
	typed(m, "wq")
	if m.ModeLabel() != ":wq" {
		t.Fatalf("label = %q", m.ModeLabel())
	}
	keys(m, "backspace")
	if m.ModeLabel() != ":w" {
		t.Fatalf("backspace label = %q", m.ModeLabel())
	}
	if keys(m, "enter") != ActionSubmit || m.Mode() != ModeNormal {
		t.Fatal(":w submits and returns to normal")
	}
	keys(m, ":")
	typed(m, "q")
	if keys(m, "enter") != ActionClose {
		t.Fatal(":q closes")
	}
	keys(m, ":")
	typed(m, "zz")
	if keys(m, "enter") != ActionNone {
		t.Fatal("unknown command is ignored")
	}
	keys(m, ":")
	typed(m, "q")
	keys(m, "esc")
	if m.Mode() != ModeNormal || m.cmdline != "" {
		t.Fatal("esc abandons the command line")
	}
	e := NewMulti("", true)
	keys(e, "esc", ":")
	typed(e, "w")
	if keys(e, "enter") != ActionClose {
		t.Fatal(":w on empty text closes instead of sending nothing")
	}
}

func TestSetVimResetsToInsert(t *testing.T) {
	m := NewMulti("x", true)
	keys(m, "esc")
	m.SetVim(false)
	if m.Vim() || m.ModeLabel() != "" {
		t.Fatal("vim off")
	}
	typed(m, "y")
	wantText(t, m, "yx") // esc had moved the cursor back onto x
	m.SetVim(true)
	if m.Mode() != ModeInsert {
		t.Fatal("vim on restarts in insert")
	}
}

func TestInsertModeIgnoresUnboundSpecialKeys(t *testing.T) {
	m := NewMulti("", true)
	m.Key(tea.KeyPressMsg{Code: tea.KeyF5})
	m.Key(tea.KeyPressMsg{Code: tea.KeyPgDown})
	wantText(t, m, "")
}
