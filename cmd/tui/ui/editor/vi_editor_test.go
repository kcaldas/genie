package editor

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// viBuf pairs a buffer with a vi state machine and types insert-mode keys
// into the buffer the way the prompt would.
type viBuf struct {
	buf *Buffer
	vi  *Vi
}

func newVi(text string) *viBuf { return &viBuf{buf: NewBuffer(text), vi: NewVi()} }

func (m *viBuf) key(k tea.KeyPressMsg) Action {
	action, handled := m.vi.Key(m.buf, k)
	if handled {
		return action
	}
	switch k.String() {
	case "enter":
		m.buf.Newline()
	case "backspace":
		m.buf.Backspace()
	default:
		if k.Text != "" {
			m.buf.InsertString(k.Text)
		}
	}
	return ActionNone
}

func k(s string) tea.KeyPressMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	r := []rune(s)
	return tea.KeyPressMsg{Code: r[0], Text: s}
}

func keys(m *viBuf, seq ...string) Action {
	last := ActionNone
	for _, s := range seq {
		last = m.key(k(s))
	}
	return last
}

func typed(m *viBuf, s string) {
	for _, r := range s {
		m.key(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func wantText(t *testing.T, m *viBuf, text string) {
	t.Helper()
	if m.buf.String() != text {
		t.Fatalf("text = %q, want %q", m.buf.String(), text)
	}
}

func wantCursor(t *testing.T, m *viBuf, row, col int) {
	t.Helper()
	r, c := m.buf.Cursor()
	if r != row || c != col {
		t.Fatalf("cursor = (%d,%d), want (%d,%d) in %q", r, c, row, col, m.buf.String())
	}
}

func TestViModeSwitching(t *testing.T) {
	m := newVi("")
	if m.vi.Mode() != ModeInsert || m.vi.ModeLabel() != "[INSERT]" {
		t.Fatal("vi starts in insert mode")
	}
	typed(m, "abc")
	keys(m, "esc")
	if m.vi.Mode() != ModeNormal || m.vi.ModeLabel() != "[NORMAL]" {
		t.Fatal("esc enters normal mode")
	}
	wantCursor(t, m, 0, 2) // esc steps back one, like vi
	if a := keys(m, "esc"); a != ActionNone {
		t.Fatal("esc in normal mode never closes")
	}
	keys(m, "i")
	if m.vi.Mode() != ModeInsert {
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
	m := newVi("foo.bar baz\n  qux")
	keys(m, "esc", "g", "g")
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
	if m.vi.pending != opNone {
		t.Fatal("unrecognised key cancels pending g")
	}
}

func TestViDeleteAndChange(t *testing.T) {
	m := newVi("one two three\nfour")
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
	if m.vi.pending != opNone {
		t.Fatal("pending command must clear")
	}
	keys(m, "l", "l", "c", "$")
	if m.vi.Mode() != ModeInsert {
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
	m := newVi("message")
	keys(m, "esc", ":")
	if m.vi.Mode() != ModeCommand || m.vi.ModeLabel() != ":" {
		t.Fatalf("':' enters command mode, label %q", m.vi.ModeLabel())
	}
	typed(m, "wq")
	if m.vi.ModeLabel() != ":wq" {
		t.Fatalf("label = %q", m.vi.ModeLabel())
	}
	keys(m, "backspace")
	if m.vi.ModeLabel() != ":w" {
		t.Fatalf("backspace label = %q", m.vi.ModeLabel())
	}
	if keys(m, "enter") != ActionSubmit || m.vi.Mode() != ModeNormal {
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
	if m.vi.Mode() != ModeNormal || m.vi.cmdline != "" {
		t.Fatal("esc abandons the command line")
	}
	e := newVi("")
	keys(e, "esc", ":")
	typed(e, "w")
	if keys(e, "enter") != ActionClose {
		t.Fatal(":w on empty text closes instead of sending nothing")
	}
}

func TestViInsertModeLeavesKeysToTheOwner(t *testing.T) {
	v := NewVi()
	buf := NewBuffer("")
	if _, handled := v.Key(buf, k("a")); handled {
		t.Fatal("insert mode must not claim printable keys")
	}
	if _, handled := v.Key(buf, k("enter")); handled {
		t.Fatal("insert mode must not claim enter")
	}
	v.Reset()
	if v.Mode() != ModeInsert {
		t.Fatal("reset returns to insert")
	}
}
