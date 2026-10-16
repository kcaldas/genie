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

// Action is what a vi key asks the owner of the buffer to do.
type Action int

const (
	ActionNone   Action = iota
	ActionSubmit        // send the text (:w, :wq)
	ActionClose         // leave the editor (:q)
)

// Vi is the vi state machine over a Buffer: normal, insert and command
// modes with i a A o O h j k l w W b B x dd d$ d0 c$ c0 $ 0 gg G and
// :q :w :wq. In insert mode it only claims Esc; every other key is left
// to the owner, so typing keeps the owner's undo grouping and chips.
type Vi struct {
	mode    ViMode
	pending pendingOp
	cmdline string
}

// NewVi returns a state machine in insert mode, since the user came here
// to type.
func NewVi() *Vi { return &Vi{mode: ModeInsert} }

// Mode returns the current mode.
func (v *Vi) Mode() ViMode { return v.mode }

// Reset returns to insert mode and drops any pending operator.
func (v *Vi) Reset() { v.mode, v.pending, v.cmdline = ModeInsert, opNone, "" }

// ModeLabel is the mode as shown to the user: "[NORMAL]", "[INSERT]" or
// the command line being typed.
func (v *Vi) ModeLabel() string {
	switch v.mode {
	case ModeNormal:
		return "[NORMAL]"
	case ModeCommand:
		return ":" + v.cmdline
	default:
		return "[INSERT]"
	}
}

// Key applies one key press to buf. handled is false when the key should
// be processed as ordinary editing (insert mode).
func (v *Vi) Key(buf *Buffer, k tea.KeyPressMsg) (action Action, handled bool) {
	switch v.mode {
	case ModeInsert:
		if k.String() == "esc" {
			v.setMode(ModeNormal)
			if _, col := buf.Cursor(); col > 0 {
				buf.Left()
			}
			return ActionNone, true
		}
		return ActionNone, false
	case ModeCommand:
		return v.commandKey(buf, k), true
	default:
		v.normalKey(buf, k)
		return ActionNone, true
	}
}

// normalKey handles one key in normal mode. An operator (d, c, g) waits
// for its motion; anything that is not a valid motion cancels it.
func (v *Vi) normalKey(buf *Buffer, k tea.KeyPressMsg) {
	if v.pending != opNone {
		v.operatorMotion(buf, k.Text)
		return
	}
	switch k.String() {
	case "esc", "ctrl+c":
		return
	case "enter":
		if buf.Down() {
			buf.LineStart()
		}
		return
	case "left":
		buf.Left()
		return
	case "right":
		buf.Right()
		return
	case "up":
		buf.Up()
		return
	case "down":
		buf.Down()
		return
	}
	if k.Text == "" {
		return
	}
	switch k.Text {
	case "d":
		v.pending = opDelete
	case "c":
		v.pending = opChange
	case "g":
		v.pending = opGoto
	case ":":
		v.setMode(ModeCommand)
		v.cmdline = ""
	case "x":
		buf.Delete()
	default:
		if !v.enterInsert(buf, k.Text) {
			motion(buf, k.Text)
		}
	}
}

// operatorMotion completes a pending d, c or g with the key that follows.
func (v *Vi) operatorMotion(buf *Buffer, text string) {
	op := v.pending
	v.pending = opNone
	switch {
	case op == opGoto && text == "g":
		buf.DocStart()
	case op == opDelete && text == "d":
		buf.DeleteLine()
	case (op == opDelete || op == opChange) && text == "$":
		buf.DeleteToLineEnd()
	case (op == opDelete || op == opChange) && text == "0":
		buf.DeleteToLineStart()
	default:
		return
	}
	if op == opChange {
		v.setMode(ModeInsert)
	}
}

// enterInsert handles the keys that switch to insert mode (i a A o O)
// and reports whether text was one of them.
func (v *Vi) enterInsert(buf *Buffer, text string) bool {
	row, col := buf.Cursor()
	switch text {
	case "i":
	case "a":
		if col < buf.LineLen(row) {
			buf.SetCursor(row, col+1)
		}
	case "A":
		buf.LineEnd()
	case "o":
		buf.LineEnd()
		buf.Newline()
	case "O":
		buf.LineStart()
		buf.Newline()
		buf.Up()
	default:
		return false
	}
	v.setMode(ModeInsert)
	return true
}

// motion handles the cursor motions of normal mode.
func motion(buf *Buffer, text string) {
	row, col := buf.Cursor()
	lineLen := buf.LineLen(row)
	switch text {
	case "h":
		if col > 0 {
			buf.SetCursor(row, col-1)
		}
	case "l":
		if col < lineLen {
			buf.SetCursor(row, col+1)
		}
	case "j":
		buf.Down()
	case "k":
		buf.Up()
	case "w":
		buf.ViWordForward(false)
	case "W":
		buf.ViWordForward(true)
	case "b":
		buf.ViWordBackward(false)
	case "B":
		buf.ViWordBackward(true)
	case "G":
		gotoBottom(buf)
	case "$":
		if lineLen > 0 {
			buf.SetCursor(row, lineLen-1)
		}
	case "0":
		buf.LineStart()
	}
}

// gotoBottom moves to the start of the last line, skipping a trailing
// empty line as vi's G does on a buffer that ends with a newline.
func gotoBottom(buf *Buffer) {
	last := buf.LineCount() - 1
	if last > 0 && buf.LineLen(last) == 0 {
		last--
	}
	buf.SetCursor(last, 0)
}

func (v *Vi) commandKey(buf *Buffer, k tea.KeyPressMsg) Action {
	switch k.String() {
	case "esc":
		v.setMode(ModeNormal)
		v.cmdline = ""
	case "enter":
		cmd := strings.TrimSpace(v.cmdline)
		v.setMode(ModeNormal)
		v.cmdline = ""
		switch cmd {
		case "q", "q!":
			return ActionClose
		case "w", "wq", "x":
			if strings.TrimSpace(buf.String()) == "" {
				return ActionClose
			}
			return ActionSubmit
		}
	case "backspace":
		if r := []rune(v.cmdline); len(r) > 0 {
			v.cmdline = string(r[:len(r)-1])
		}
	default:
		if k.Text != "" {
			v.cmdline += k.Text
		}
	}
	return ActionNone
}

func (v *Vi) setMode(mode ViMode) {
	v.mode = mode
	v.pending = opNone
}
