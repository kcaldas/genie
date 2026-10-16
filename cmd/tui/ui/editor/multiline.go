package editor

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// ViMode is the vi editing mode.
type ViMode int

const (
	ModeInsert ViMode = iota
	ModeNormal
	ModeCommand
)

// pendingOp is an operator waiting for its motion.
type pendingOp int

const (
	opNone   pendingOp = iota
	opDelete           // d
	opChange           // c
	opGoto             // first g of gg
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

	vim     bool
	mode    ViMode
	pending pendingOp
	cmdline string
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
	m.pending, m.cmdline = opNone, ""
}

// Vim reports whether vi emulation is on.
func (m *Multi) Vim() bool { return m.vim }

// Mode returns the current vi mode; it is always insert with vim off.
func (m *Multi) Mode() ViMode { return m.mode }

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

// normalKey handles one key in vi normal mode. An operator (d, c, g)
// waits for its motion; anything that is not a valid motion cancels it.
func (m *Multi) normalKey(k tea.KeyPressMsg) {
	if m.pending != opNone {
		m.operatorMotion(k.Text)
		return
	}
	switch k.String() {
	case "esc", "ctrl+c":
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
		return
	}
	switch k.Text {
	case "d":
		m.pending = opDelete
	case "c":
		m.pending = opChange
	case "g":
		m.pending = opGoto
	case ":":
		m.setMode(ModeCommand)
		m.cmdline = ""
	case "x":
		m.Buf.Delete()
	default:
		if !m.enterInsert(k.Text) {
			m.motion(k.Text)
		}
	}
}

// operatorMotion completes a pending d, c or g with the key that follows.
func (m *Multi) operatorMotion(text string) {
	op := m.pending
	m.pending = opNone
	switch {
	case op == opGoto && text == "g":
		m.Buf.DocStart()
	case op == opDelete && text == "d":
		m.Buf.DeleteLine()
	case (op == opDelete || op == opChange) && text == "$":
		m.Buf.DeleteToLineEnd()
	case (op == opDelete || op == opChange) && text == "0":
		m.Buf.DeleteToLineStart()
	default:
		return
	}
	if op == opChange {
		m.setMode(ModeInsert)
	}
}

// enterInsert handles the keys that switch to insert mode (i a A o O)
// and reports whether text was one of them.
func (m *Multi) enterInsert(text string) bool {
	row, col := m.Buf.Cursor()
	switch text {
	case "i":
	case "a":
		if col < m.Buf.LineLen(row) {
			m.Buf.SetCursor(row, col+1)
		}
	case "A":
		m.Buf.LineEnd()
	case "o":
		m.Buf.LineEnd()
		m.Buf.Newline()
	case "O":
		m.Buf.LineStart()
		m.Buf.Newline()
		m.Buf.Up()
	default:
		return false
	}
	m.setMode(ModeInsert)
	return true
}

// motion handles the cursor motions of normal mode.
func (m *Multi) motion(text string) {
	row, col := m.Buf.Cursor()
	lineLen := m.Buf.LineLen(row)
	switch text {
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
	case "G":
		m.gotoBottom()
	case "$":
		if lineLen > 0 {
			m.Buf.SetCursor(row, lineLen-1)
		}
	case "0":
		m.Buf.LineStart()
	}
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

func (m *Multi) setMode(mode ViMode) {
	m.mode = mode
	m.pending = opNone
}
