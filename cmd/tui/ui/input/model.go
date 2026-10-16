// Package input is the prompt at the bottom of the screen. It owns an
// editor.Prompt and turns submitted text into command-bus events.
package input

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/ui/editor"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

// MaxRows is how tall the prompt grows before it scrolls.
const MaxRows = 8

// Clipboard reads the system clipboard for the explicit paste keys.
type Clipboard interface {
	Paste() (string, error)
}

// Model is the prompt.
type Model struct {
	prompt    *editor.Prompt
	bus       *events.CommandEventBus
	clipboard Clipboard
	width     int
	window    editor.Window
}

// New builds a prompt. clipboard may be nil.
func New(prompt *editor.Prompt, bus *events.CommandEventBus, clipboard Clipboard) *Model {
	return &Model{prompt: prompt, bus: bus, clipboard: clipboard}
}

// SetWidth sets the inner width in cells.
func (m *Model) SetWidth(w int) { m.width = w }

// Text returns what has been typed so far, with pastes expanded.
func (m *Model) Text() string { return m.prompt.Text() }

// Rows returns how many rows the prompt needs right now, between 1 and
// MaxRows, so the layout can size it.
func (m *Model) Rows() int {
	return max(1, min(m.prompt.Layout(max(m.width, 1), MaxRows).Total, MaxRows))
}

// Update handles a key or paste. It returns tea.Quit when ctrl+c or ctrl+l
// is pressed on an empty prompt, and an OpenWrite command when F4 or a
// forced paste moves composing into write mode.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		m.prompt.Paste(msg.Content)
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return nil
}

// expand opens write mode with the prompt text plus extra inserted at the
// end, and clears the prompt.
func (m *Model) expand(extra string) tea.Cmd {
	content := m.prompt.Text() + extra
	m.prompt.Clear()
	m.prompt.ResetHistory()
	return func() tea.Msg { return uimsg.OpenWrite{Content: content} }
}

func (m *Model) key(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "enter":
		m.submit()
	case "shift+enter", "alt+enter", "ctrl+j":
		m.prompt.Newline()
	case "up":
		if m.prompt.Lines() == 1 || m.prompt.OnFirstLine() {
			m.prompt.HistoryPrev()
		} else {
			m.prompt.Up()
		}
	case "down":
		if m.prompt.Lines() == 1 || m.prompt.OnLastLine() {
			m.prompt.HistoryNext()
		} else {
			m.prompt.Down()
		}
	case "ctrl+c", "ctrl+l":
		if strings.TrimSpace(m.prompt.Text()) == "" {
			return tea.Quit
		}
		m.prompt.Clear()
		m.prompt.ResetHistory()
	case "esc":
		m.bus.Emit(uimsg.TopicInputCancel, "")
	case "tab":
		m.prompt.Complete()
	case "left":
		m.prompt.Left()
	case "right":
		m.prompt.Right()
	case "home", "ctrl+a":
		m.prompt.Home()
	case "end", "ctrl+e":
		m.prompt.End()
	case "alt+left", "alt+b":
		m.prompt.WordLeft()
	case "alt+right", "alt+f":
		m.prompt.WordRight()
	case "ctrl+b":
		m.prompt.Left()
	case "ctrl+w":
		m.prompt.DeleteWordLeft()
	case "backspace":
		m.prompt.Backspace()
	case "delete":
		m.prompt.Delete()
	case "f4":
		return m.expand("")
	case "ctrl+v", "alt+v", "shift+insert":
		m.pasteFromClipboard()
	case "ctrl+shift+v":
		if text, err := m.clipboardText(); err == nil {
			return m.expand(text)
		}
	default:
		if k.Text != "" {
			m.prompt.InsertString(k.Text)
		}
	}
	return nil
}

// submit routes the trimmed text by its prefix: ':' commands, '/' slash
// commands, everything else chat.
func (m *Model) submit() {
	text := m.prompt.Submit()
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

func (m *Model) clipboardText() (string, error) {
	if m.clipboard == nil {
		return "", errNoClipboard
	}
	return m.clipboard.Paste()
}

func (m *Model) pasteFromClipboard() {
	if text, err := m.clipboardText(); err == nil {
		m.prompt.Paste(text)
	}
}

type clipboardError string

func (e clipboardError) Error() string { return string(e) }

const errNoClipboard = clipboardError("no clipboard available")

// View renders the visible rows with chips highlighted and the ghost
// suggestion after the cursor. It must be called before CursorPos.
func (m *Model) View(theme *style.Theme) string {
	m.window = m.prompt.Layout(max(m.width, 1), MaxRows)
	rows := make([]string, len(m.window.Rows))
	labels := m.prompt.ChipLabels()
	for i, row := range m.window.Rows {
		for _, label := range labels {
			row = strings.ReplaceAll(row, label, theme.Chip.Render(label))
		}
		rows[i] = row
	}
	if m.window.Ghost != "" && len(rows) > 0 {
		rows[m.window.CursorRow] += theme.Ghost.Render(m.window.Ghost)
	}
	return strings.Join(rows, "\n")
}

// CursorPos returns the cursor column and row within the view.
func (m *Model) CursorPos() (x, y int) {
	return m.window.CursorCol, m.window.CursorRow
}
