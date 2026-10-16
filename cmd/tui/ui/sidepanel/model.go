package sidepanel

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
)

// pageLines is how far PgUp/PgDn move the text viewer, as before.
const pageLines = 5

// Config is what the panels read from the TUI config.
type Config interface {
	GetConfig() *types.Config
	GetTheme() *types.Theme
}

// Model is the right panel. Exactly one viewer shows at a time; Mode says
// which, and RightNone hides the column.
type Model struct {
	cfg   Config
	debug *state.DebugState

	mode   layout.RightMode
	zoomed bool

	text  pane
	diff  pane
	dlog  pane
	title struct{ text, diff string }

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

// SetSize sets the inner (content) size and re-renders at the new width.
func (m *Model) SetSize(w, h int) {
	if w == m.width && h == m.height {
		return
	}
	m.width, m.height = w, h
	m.text.setSize(w, h)
	m.diff.setSize(w, h)
	m.dlog.setSize(w, h)
	m.renderText()
	m.renderDiff()
	m.reloadDebug()
}

// ThemeChanged re-renders content with the new theme.
func (m *Model) ThemeChanged() {
	m.gen++
	m.renderText()
	m.renderDiff()
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
			content = m.renderMarkdown(content)
			m.rendered.key = key
			m.text.setLines(strings.Split(content, "\n"))
			return
		}
		return
	}
	m.rendered.key = ""
	m.text.setLines(strings.Split(content, "\n"))
}

// renderMarkdown renders through glamour, falling back to the ascii style
// for very large documents and to raw text on error, as before.
func (m *Model) renderMarkdown(content string) string {
	cfg := m.cfg.GetConfig()
	glamourTheme := cfg.GlamourTheme
	if len(content) > 50000 {
		glamourTheme = "ascii"
	}
	r, err := presentation.CreateMarkdownRendererWithWidth(m.cfg.GetTheme(), cfg.Theme, glamourTheme, m.width)
	if err != nil {
		return content
	}
	out, err := r.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(out)
}

func (m *Model) renderDiff() {
	if m.diffContent == "" {
		m.diff.setLines(nil)
		return
	}
	cfg := m.cfg.GetConfig()
	name := cfg.DiffTheme
	if name == "" || name == "auto" {
		name = presentation.GetDiffThemeForMainTheme(cfg.Theme)
	}
	formatted := presentation.NewDiffFormatter(presentation.GetDiffTheme(name)).Format(m.diffContent)
	m.diff.setLines(strings.Split(formatted, "\n"))
}

func (m *Model) reloadDebug() {
	if m.debug == nil {
		return
	}
	m.dlog.setLines(m.debug.GetDebugMessages())
}
