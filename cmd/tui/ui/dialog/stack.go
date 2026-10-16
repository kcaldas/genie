// Package dialog holds the modal layers that sit above the main screen:
// the LLM context viewer, write mode and confirmations. While a dialog is
// open it receives every key, which is what replaces the old global
// keybinding enable/disable dance.
package dialog

import (
	tea "charm.land/bubbletea/v2"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
)

// Dialog is one modal layer.
type Dialog interface {
	// SetSize gives the dialog the full screen size to lay itself out in.
	SetSize(w, h int)
	// Update handles a message. done reports that the dialog wants to close.
	Update(msg tea.Msg) (cmd tea.Cmd, done bool)
	// View renders the dialog. Fullscreen dialogs render the whole screen;
	// others render just themselves and are composited centred over the
	// base frame.
	View(theme *style.Theme) string
	Fullscreen() bool
	// ShowsStatus reports whether the status row stays visible below a
	// fullscreen dialog.
	ShowsStatus() bool
	// Cursor returns where the terminal cursor should be, or nil to hide it.
	Cursor() *tea.Cursor
}

// Stack is the open dialogs, newest on top.
type Stack struct {
	items []Dialog
	w, h  int
}

// Push opens a dialog on top.
func (s *Stack) Push(d Dialog) {
	d.SetSize(s.w, s.h)
	s.items = append(s.items, d)
}

// Pop closes the top dialog.
func (s *Stack) Pop() {
	if len(s.items) > 0 {
		s.items = s.items[:len(s.items)-1]
	}
}

// Top returns the open dialog receiving input, or nil.
func (s *Stack) Top() Dialog {
	if len(s.items) == 0 {
		return nil
	}
	return s.items[len(s.items)-1]
}

// Len returns how many dialogs are open.
func (s *Stack) Len() int { return len(s.items) }

// SetSize propagates a resize to every open dialog.
func (s *Stack) SetSize(w, h int) {
	s.w, s.h = w, h
	for _, d := range s.items {
		d.SetSize(w, h)
	}
}

// Update routes a message to the top dialog, popping it when done.
func (s *Stack) Update(msg tea.Msg) tea.Cmd {
	top := s.Top()
	if top == nil {
		return nil
	}
	cmd, done := top.Update(msg)
	if done {
		s.Pop()
	}
	return cmd
}
