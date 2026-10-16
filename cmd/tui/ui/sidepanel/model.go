package sidepanel

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
	"github.com/kcaldas/genie/cmd/tui/ui/render"
)

// pageLines is how far PgUp/PgDn move the text viewer, as before.
const pageLines = 5

// Config is what the panels read from the TUI config.
type Config = render.Config

// Model is the right panel. Exactly one viewer shows at a time; Mode says
// which, and RightNone hides the column.
type Model struct {
	cfg   Config
	debug *state.DebugState

	mode   layout.RightMode
	zoomed bool

	text     pane
	diff     pane
	dlog     pane
	todos    pane
	title    struct{ text, diff string }
	todoList []presentation.TodoItem

	// Raw content kept so a resize or theme change can re-render it.
	textContent, textType, diffContent string
	gen                                uint64
	rendered                           struct{ key string }
	width, height                      int
}

// New builds a hidden right panel.
func New(cfg Config, debug *state.DebugState) *Model {
	m := &Model{cfg: cfg, debug: debug}
	m.text.wrap = true
	m.dlog.wrap = true
	m.dlog.stick = true
	m.todos.wrap = true
	m.title.text, m.title.diff = "Help", "Diff"
	return m
}

// Mode returns the visible viewer, RightNone when hidden.
func (m *Model) Mode() layout.RightMode { return m.mode }

// Zoomed reports whether the panel takes most of the width.
func (m *Model) Zoomed() bool { return m.zoomed }

// Visible reports whether the panel is shown.
func (m *Model) Visible() bool { return m.mode != layout.RightNone }

// Show switches to mode and makes the panel visible.
func (m *Model) Show(mode layout.RightMode) {
	m.mode = mode
	if mode == layout.RightDebug {
		m.reloadDebug()
	}
}

// Hide hides the panel and resets the zoom.
func (m *Model) Hide() {
	m.mode = layout.RightNone
	m.zoomed = false
}

// Toggle shows mode, or hides the panel if mode is already showing.
func (m *Model) Toggle(mode layout.RightMode) {
	if m.mode == mode {
		m.Hide()
		return
	}
	m.Show(mode)
}

// ToggleZoom flips the zoom while the panel is visible.
func (m *Model) ToggleZoom() {
	if m.Visible() {
		m.zoomed = !m.zoomed
	}
}

// SetText sets the text viewer content; ctype "markdown" renders through
// glamour, anything else is shown as-is.
func (m *Model) SetText(title, content, ctype string) {
	m.title.text = title
	m.textContent, m.textType = content, ctype
	m.text.top()
	m.renderText()
}

// SetDiff sets the diff viewer content.
func (m *Model) SetDiff(title, content string) {
	m.title.diff = title
	m.diffContent = content
	m.diff.top()
	m.renderDiff()
}

// SetTodos replaces the todo list shown by the Todos viewer.
func (m *Model) SetTodos(items []presentation.TodoItem) {
	m.todoList = items
	m.renderTodos()
}

// HasTodos reports whether a todo list has been received.
func (m *Model) HasTodos() bool { return m.todoList != nil }

func (m *Model) renderTodos() {
	if m.todoList == nil {
		m.todos.setLines(nil)
		return
	}
	f := presentation.NewTodoFormatter(m.cfg.GetTheme())
	m.todos.setLines(strings.Split(f.FormatTodoList(m.todoList), "\n"))
}

// SetSize sets the inner (content) size and re-renders at the new width.
func (m *Model) SetSize(w, h int) {
	if w == m.width && h == m.height {
		return
	}
	m.width, m.height = w, h
	m.text.setSize(w, h)
	m.diff.setSize(w, h)
	m.dlog.setSize(w, h)
	m.todos.setSize(w, h)
	m.renderText()
	m.renderDiff()
	m.renderTodos()
	m.reloadDebug()
}

// ThemeChanged re-renders content with the new theme.
func (m *Model) ThemeChanged() {
	m.gen++
	m.renderText()
	m.renderDiff()
	m.renderTodos()
}

// DebugChanged re-reads the debug log from state.
func (m *Model) DebugChanged() { m.reloadDebug() }

// Title is the frame title for the visible viewer.
func (m *Model) Title() string {
	switch m.mode {
	case layout.RightText:
		return " " + m.title.text + " "
	case layout.RightDiff:
		return " " + m.title.diff + " "
	case layout.RightDebug:
		return " Debug "
	case layout.RightTodos:
		done := 0
		for _, t := range m.todoList {
			if t.Status == "completed" {
				done++
			}
		}
		return fmt.Sprintf(" Todos %d/%d ", done, len(m.todoList))
	}
	return ""
}

// Update handles navigation keys while the panel has focus. It returns a
// bus topic to emit for the debug panel's clear/copy keys, or "".
func (m *Model) Update(msg tea.Msg) (topic string) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return ""
	}
	p := m.active()
	if p == nil {
		return ""
	}
	switch k.String() {
	case "up", "k":
		p.scrollBy(-1)
	case "down", "j":
		p.scrollBy(1)
	case "pgup", "ctrl+u":
		m.PageUp()
	case "pgdown", "ctrl+d":
		m.PageDown()
	case "home":
		p.top()
	case "end":
		p.bottom()
	case "left":
		if m.mode == layout.RightDiff {
			p.scrollX(-1)
		}
	case "right":
		if m.mode == layout.RightDiff {
			p.scrollX(1)
		}
	case "c":
		if m.mode == layout.RightDebug {
			return "debug.clear"
		}
	case "y":
		if m.mode == layout.RightDebug {
			return "debug.copy"
		}
	}
	return ""
}

// ScrollBy scrolls the visible viewer by n lines.
func (m *Model) ScrollBy(n int) {
	if p := m.active(); p != nil {
		p.scrollBy(n)
	}
}

// PageUp scrolls up: five lines in the text viewer, a screen elsewhere.
func (m *Model) PageUp() { m.ScrollBy(-m.pageSize()) }

// PageDown scrolls down: five lines in the text viewer, a screen elsewhere.
func (m *Model) PageDown() { m.ScrollBy(m.pageSize()) }

func (m *Model) pageSize() int {
	if m.mode == layout.RightText {
		return pageLines
	}
	return max(m.height, 1)
}

// View renders the visible viewer's window.
func (m *Model) View() string {
	if p := m.active(); p != nil {
		return p.view()
	}
	return ""
}

func (m *Model) active() *pane {
	switch m.mode {
	case layout.RightText:
		return &m.text
	case layout.RightDiff:
		return &m.diff
	case layout.RightDebug:
		return &m.dlog
	case layout.RightTodos:
		return &m.todos
	}
	return nil
}

func (m *Model) renderText() {
	if m.width <= 0 {
		return
	}
	content := m.textContent
	if m.textType == "markdown" && content != "" {
		key := fmt.Sprintf("%d:%d:%d:%s", m.width, m.gen, len(content), m.title.text)
		if key != m.rendered.key {
			content = render.Markdown(m.cfg, content, m.width)
			m.rendered.key = key
			m.text.setLines(strings.Split(content, "\n"))
			return
		}
		return
	}
	m.rendered.key = ""
	m.text.setLines(strings.Split(content, "\n"))
}

func (m *Model) renderDiff() {
	if m.diffContent == "" {
		m.diff.setLines(nil)
		return
	}
	m.diff.setLines(strings.Split(render.Diff(m.cfg, m.diffContent), "\n"))
}

func (m *Model) reloadDebug() {
	if m.debug == nil {
		return
	}
	m.dlog.setLines(m.debug.GetDebugMessages())
}
