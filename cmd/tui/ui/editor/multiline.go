package editor

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Mode is the vi editing mode.
type Mode int

const (
	ModeInsert Mode = iota
	ModeNormal
	ModeCommand
)

// Action is what a key press asks the owner of the editor to do.
type Action int

const (
	ActionNone   Action = iota
	ActionSubmit        // send the text
	ActionClose         // discard and close
)

// Multi is the multi-line editor behind write mode. With vim on it has
// normal, insert and command modes (i a A o O h j k l w W b B x dd d$ d0
// c$ c0 $ 0 gg G :q :w :wq); with vim off it is a plain editor where Esc
// closes. Ctrl+S submits and Ctrl+C or Ctrl+L clear, or close when
// already empty, in both.
type Multi struct {
	Buf *Buffer

	vim      bool
	mode     Mode
	pending  rune // 'd' or 'c' awaiting its motion
	pendingG bool
	cmdline  string
}

// NewMulti returns an editor holding text with the cursor at the end.
// With vim on it starts in insert mode, since the user came here to type.
func NewMulti(text string, vim bool) *Multi {
	m := &Multi{Buf: NewBuffer(text), vim: vim}
	m.mode = ModeInsert
	return m
}

// SetVim switches vi emulation on or off, resetting to insert mode.
func (m *Multi) SetVim(on bool) {
	m.vim = on
	m.mode = ModeInsert
	m.pending, m.pendingG, m.cmdline = 0, false, ""
}

// Vim reports whether vi emulation is on.
func (m *Multi) Vim() bool { return m.vim }

// Mode returns the current vi mode; it is always insert with vim off.
func (m *Multi) Mode() Mode { return m.mode }

// ModeLabel is the mode as shown in the title: "[NORMAL]", "[INSERT]" or
// the command line being typed; "" with vim off.
func (m *Multi) ModeLabel() string {
	if !m.vim {
		return ""
	}
	switch m.mode {
	case ModeNormal:
		return "[NORMAL]"
	case ModeCommand:
		return ":" + m.cmdline
	default:
		return "[INSERT]"
	}
}

// Text returns the buffer content.
func (m *Multi) Text() string { return m.Buf.String() }

// Key applies one key press and reports the resulting action.
func (m *Multi) Key(k tea.KeyPressMsg) Action {
	switch k.String() {
	case "ctrl+s":
		if strings.TrimSpace(m.Text()) == "" {
			return ActionClose
		}
		return ActionSubmit
	case "ctrl+c", "ctrl+l":
		if strings.TrimSpace(m.Text()) != "" {
			m.Buf.Clear()
			return ActionNone
		}
		if m.vim && m.mode != ModeNormal {
			m.setMode(ModeNormal)
			return ActionNone
		}
		return ActionClose
	}
	if !m.vim {
		return m.plainKey(k)
	}
	switch m.mode {
	case ModeNormal:
		m.normalKey(k)
	case ModeInsert:
		if k.String() == "esc" {
			m.setMode(ModeNormal)
			if _, col := m.Buf.Cursor(); col > 0 {
				m.Buf.Left()
			}
			return ActionNone
		}
		m.editKey(k)
	case ModeCommand:
		return m.commandKey(k)
	}
	return ActionNone
}

// plainKey is the non-vim editor: Esc closes, everything else edits.
func (m *Multi) plainKey(k tea.KeyPressMsg) Action {
	switch k.String() {
	case "esc":
		return ActionClose
	case "ctrl+left", "alt+left":
		m.Buf.LineStart()
	case "ctrl+right", "alt+right":
		m.Buf.LineEnd()
	default:
		m.editKey(k)
	}
	return ActionNone
}

// editKey is insert-mode editing shared by both flavours.
func (m *Multi) editKey(k tea.KeyPressMsg) {
	switch k.String() {
	case "enter":
		m.Buf.Newline()
	case "backspace":
		m.Buf.Backspace()
	case "delete":
		m.Buf.Delete()
	case "left":
		m.Buf.Left()
	case "right":
		m.Buf.Right()
	case "up":
		m.Buf.Up()
	case "down":
		m.Buf.Down()
	case "home":
		m.Buf.LineStart()
	case "end":
		m.Buf.LineEnd()
	case "tab":
		m.Buf.Insert('\t')
	default:
		if k.Text != "" {
			m.Buf.InsertString(k.Text)
		}
	}
}

func (m *Multi) normalKey(k tea.KeyPressMsg) {
	s := k.String()
	switch s {
	case "esc", "ctrl+c":
		m.pending, m.pendingG = 0, false
		return
	case "enter":
		if m.Buf.Down() {
			m.Buf.LineStart()
		}
		return
	case "left":
		m.Buf.Left()
		return
	case "right":
		m.Buf.Right()
		return
	case "up":
		m.Buf.Up()
		return
	case "down":
		m.Buf.Down()
		return
	}
	if k.Text == "" {
		m.pending, m.pendingG = 0, false
		return
	}
	// An operator waits for its motion; any other key cancels it, as "dx"
	// does nothing in vi.
	if m.pending != 0 {
		valid := k.Text == "$" || k.Text == "0" || (m.pending == 'd' && k.Text == "d")
		if !valid {
			m.pending = 0
			return
		}
	}
	row, col := m.Buf.Cursor()
	lineLen := m.Buf.LineLen(row)
	switch k.Text {
	case "i":
		m.setMode(ModeInsert)
	case "a":
		m.setMode(ModeInsert)
		if col < lineLen {
			m.Buf.SetCursor(row, col+1)
		}
	case "A":
		m.setMode(ModeInsert)
		m.Buf.LineEnd()
	case "o":
		m.setMode(ModeInsert)
		m.Buf.LineEnd()
		m.Buf.Newline()
	case "O":
		m.setMode(ModeInsert)
		m.Buf.LineStart()
		m.Buf.Newline()
		m.Buf.Up()
	case "h":
		if col > 0 {
			m.Buf.SetCursor(row, col-1)
		}
	case "l":
		if col < lineLen {
			m.Buf.SetCursor(row, col+1)
		}
	case "j":
		m.Buf.Down()
	case "k":
		m.Buf.Up()
	case "w":
		m.Buf.ViWordForward(false)
	case "W":
		m.Buf.ViWordForward(true)
	case "b":
		m.Buf.ViWordBackward(false)
	case "B":
		m.Buf.ViWordBackward(true)
	case "x":
		m.Buf.Delete()
	case "d":
		if m.pending == 'd' {
			m.Buf.DeleteLine()
			m.pending = 0
		} else {
			m.pending = 'd'
		}
		return
	case "c":
		m.pending = 'c'
		return
	case "u":
	case ":":
		m.setMode(ModeCommand)
		m.cmdline = ""
	case "g":
		if m.pendingG {
			m.Buf.DocStart()
			m.pendingG = false
		} else {
			m.pendingG = true
		}
		return
	case "G":
		m.gotoBottom()
	case "$":
		switch m.pending {
		case 'd':
			m.Buf.DeleteToLineEnd()
		case 'c':
			m.Buf.DeleteToLineEnd()
			m.setMode(ModeInsert)
		default:
			if lineLen > 0 {
				m.Buf.SetCursor(row, lineLen-1)
			}
		}
	case "0":
		switch m.pending {
		case 'd':
			m.Buf.DeleteToLineStart()
		case 'c':
			m.Buf.DeleteToLineStart()
			m.setMode(ModeInsert)
		default:
			m.Buf.LineStart()
		}
	}
	m.pending, m.pendingG = 0, false
}

// gotoBottom moves to the start of the last line, skipping a trailing
// empty line as vi's G does on a buffer that ends with a newline.
func (m *Multi) gotoBottom() {
	last := m.Buf.LineCount() - 1
	if last > 0 && m.Buf.LineLen(last) == 0 {
		last--
	}
	m.Buf.SetCursor(last, 0)
}

func (m *Multi) commandKey(k tea.KeyPressMsg) Action {
	switch k.String() {
	case "esc":
		m.setMode(ModeNormal)
		m.cmdline = ""
	case "enter":
		cmd := strings.TrimSpace(m.cmdline)
		m.setMode(ModeNormal)
		m.cmdline = ""
		switch cmd {
		case "q", "q!":
			return ActionClose
		case "w", "wq", "x":
			if strings.TrimSpace(m.Text()) == "" {
				return ActionClose
			}
			return ActionSubmit
		}
	case "backspace":
		if r := []rune(m.cmdline); len(r) > 0 {
			m.cmdline = string(r[:len(r)-1])
		}
	default:
		if k.Text != "" {
			m.cmdline += k.Text
		}
	}
	return ActionNone
}

func (m *Multi) setMode(mode Mode) {
	m.mode = mode
	m.pending, m.pendingG = 0, false
}
