// Package input is the single-line prompt at the bottom of the screen. It
// owns an editor.Line and turns submitted text into command-bus events.
package input

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/ui/editor"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
)

// Bus topics the input emits.
const (
	TopicText         = "user.input.text"
	TopicCommand      = "user.input.command"
	TopicSlashCommand = "user.input.slashcommand"
	TopicCancel       = "user.input.cancel"
)

// Clipboard reads the system clipboard for the explicit paste keys.
type Clipboard interface {
	Paste() (string, error)
}

// Model is the prompt.
type Model struct {
	line      *editor.Line
	bus       *events.CommandEventBus
	clipboard Clipboard
	width     int
}

// New builds a prompt over line. clipboard may be nil.
func New(line *editor.Line, bus *events.CommandEventBus, clipboard Clipboard) *Model {
	return &Model{line: line, bus: bus, clipboard: clipboard}
}

// SetWidth sets the inner width in cells.
func (m *Model) SetWidth(w int) { m.width = w }

// Text returns what has been typed so far.
func (m *Model) Text() string { return m.line.Text() }

// Update handles a key or paste. It returns tea.Quit when ctrl+c or ctrl+l
// is pressed on an empty prompt.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		m.line.InsertString(msg.Content)
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return nil
}

func (m *Model) key(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "enter":
		m.submit()
	case "up":
		m.line.HistoryPrev()
	case "down":
		m.line.HistoryNext()
	case "ctrl+c", "ctrl+l":
		if strings.TrimSpace(m.line.Text()) == "" {
			return tea.Quit
		}
		m.line.Clear()
		m.line.ResetHistory()
	case "esc":
		m.bus.Emit(TopicCancel, "")
	case "tab":
		m.line.Complete()
	case "left":
		m.line.Left()
	case "right":
		m.line.Right()
	case "home", "ctrl+a":
		m.line.Home()
	case "end", "ctrl+e":
		m.line.End()
	case "alt+left", "alt+b":
		m.line.WordLeft()
	case "alt+right", "alt+f":
		m.line.WordRight()
	case "ctrl+b":
		m.line.Left()
	case "ctrl+w":
		m.line.DeleteWordLeft()
	case "backspace":
		m.line.Backspace()
	case "delete":
		m.line.Delete()
	case "ctrl+v", "alt+v", "shift+insert":
		m.pasteFromClipboard()
	default:
		if k.Text != "" {
			m.line.InsertString(k.Text)
		}
	}
	return nil
}

// submit routes the trimmed text by its prefix: ':' commands, '/' slash
// commands, everything else chat.
func (m *Model) submit() {
	text := m.line.Submit()
	switch {
	case text == "":
	case strings.HasPrefix(text, ":"):
		m.bus.Emit(TopicCommand, text)
	case strings.HasPrefix(text, "/"):
		m.bus.Emit(TopicSlashCommand, text)
	default:
		m.bus.Emit(TopicText, text)
	}
}

func (m *Model) pasteFromClipboard() {
	if m.clipboard == nil {
		return
	}
	if text, err := m.clipboard.Paste(); err == nil {
		m.line.InsertString(text)
	}
}

// View renders the visible window of the line with the ghost suggestion.
func (m *Model) View(theme *style.Theme) string {
	visible, _, ghost := m.line.Window(m.width)
	if ghost == "" {
		return visible
	}
	return visible + theme.Ghost.Render(ghost)
}

// CursorX returns the cursor column within the view.
func (m *Model) CursorX() int {
	_, x, _ := m.line.Window(m.width)
	return x
}
