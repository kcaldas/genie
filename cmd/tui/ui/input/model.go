// Package input is the single-line prompt at the bottom of the screen. It
// owns an editor.Line and turns submitted text into command-bus events.
package input

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/ui/editor"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
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
// is pressed on an empty prompt, and an OpenWrite command when a
// multi-line paste or F4 moves composing into write mode.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		return m.paste(msg.Content, false)
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return nil
}

// paste inserts text, or hands the combined prompt over to write mode
// when the text spans lines (or force is set).
func (m *Model) paste(text string, force bool) tea.Cmd {
	if !force && !strings.Contains(text, "\n") {
		m.line.InsertString(text)
		return nil
	}
	return m.expand(text)
}

// expand opens write mode with the prompt text plus extra inserted at the
// cursor, and clears the prompt.
func (m *Model) expand(extra string) tea.Cmd {
	runes := []rune(m.line.Text())
	cur := min(m.line.Cursor(), len(runes))
	content := string(runes[:cur]) + extra + string(runes[cur:])
	m.line.Clear()
	m.line.ResetHistory()
	return func() tea.Msg { return uimsg.OpenWrite{Content: content} }
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
		m.bus.Emit(uimsg.TopicInputCancel, "")
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
	case "f4":
		return m.expand("")
	case "ctrl+v", "alt+v", "shift+insert":
		return m.pasteFromClipboard(false)
	case "ctrl+shift+v":
		return m.pasteFromClipboard(true)
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
		m.bus.Emit(uimsg.TopicInputCommand, text)
	case strings.HasPrefix(text, "/"):
		m.bus.Emit(uimsg.TopicInputSlashCommand, text)
	default:
		m.bus.Emit(uimsg.TopicInputText, text)
	}
}

func (m *Model) pasteFromClipboard(force bool) tea.Cmd {
	if m.clipboard == nil {
		return nil
	}
	text, err := m.clipboard.Paste()
	if err != nil {
		return nil
	}
	return m.paste(text, force)
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
