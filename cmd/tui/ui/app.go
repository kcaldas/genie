// Package ui is the Bubble Tea root of the genie TUI. The Model owns the
// screen: it lays panels out, routes input by focus, and applies the
// messages controllers dispatch through the bridge.
package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/ui/bridge"
	"github.com/kcaldas/genie/cmd/tui/ui/chatview"
	"github.com/kcaldas/genie/cmd/tui/ui/editor"
	"github.com/kcaldas/genie/cmd/tui/ui/input"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
	"github.com/kcaldas/genie/cmd/tui/ui/statusbar"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

// wheelLines is how far one mouse wheel notch scrolls.
const wheelLines = 3

// Clipboard is the system clipboard as the UI needs it.
type Clipboard interface {
	Copy(text string) error
	Paste() (string, error)
}

// Deps are the collaborators the root model is built from.
type Deps struct {
	Dispatcher *bridge.Dispatcher
	Config     *helpers.ConfigManager
	Chat       *state.ChatState
	State      *state.StateAccessor
	Bus        *events.CommandEventBus
	Line       *editor.Line
	Clipboard  Clipboard
	Now        func() time.Time // nil for the wall clock
}

// Focus names the panel receiving keys.
type Focus int

const (
	FocusInput Focus = iota
	FocusMessages
)

// Model is the root tea.Model.
type Model struct {
	deps  Deps
	w, h  int
	rects layout.Rects
	lay   layout.State

	theme *style.Theme
	gen   uint64

	chat   *chatview.Model
	input  *input.Model
	status *statusbar.Model

	focus      Focus
	persona    string
	showCursor bool
	mouse      bool
}

// New builds the root model. It fails only if the configured markdown
// style cannot be loaded.
func New(d Deps) (*Model, error) {
	cfg := d.Config.GetConfig()
	fmtr, err := chatview.NewFormatter(cfg, d.Config.GetTheme())
	if err != nil {
		return nil, fmt.Errorf("message formatter: %w", err)
	}
	m := &Model{
		deps:       d,
		theme:      style.New(cfg.Theme, 0),
		chat:       chatview.New(d.Chat, fmtr),
		input:      input.New(d.Line, d.Bus, d.Clipboard),
		status:     statusbar.New(d.State, d.Now),
		showCursor: cfg.IsShowCursorEnabled(),
		mouse:      cfg.IsMouseEnabled(),
	}
	return m, nil
}

// Init starts listening for controller messages and the status animation.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.deps.Dispatcher.Next(), m.status.Start())
}

// Update applies one message.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case bridge.Envelope:
		_, cmd := m.Update(msg.Msg)
		return m, tea.Batch(cmd, m.deps.Dispatcher.Next())
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyPressMsg:
		return m, m.key(msg)
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.chat.ScrollBy(-wheelLines)
		case tea.MouseWheelDown:
			m.chat.ScrollBy(wheelLines)
		}
		return m, nil
	case tea.PasteMsg:
		if m.focus == FocusInput {
			return m, m.input.Update(msg)
		}
		return m, nil
	case uimsg.PersonaChanged:
		m.persona = msg.Name
		return m, nil
	case uimsg.ThemeChanged:
		return m, m.applyTheme(msg)
	case uimsg.CursorChanged:
		m.showCursor = msg.Visible
		return m, nil
	case uimsg.MouseChanged:
		m.mouse = msg.Enabled
		return m, nil
	case uimsg.Quit:
		return m, tea.Quit
	}
	// Chat and status own their own message types (including ticks).
	return m, tea.Batch(m.chat.Update(msg), m.status.Update(msg))
}

// key routes a key press: a few global shortcuts first, then the focused
// panel. A panel's own binding wins over a global one, as it did before.
func (m *Model) key(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "tab":
		m.toggleFocus()
		return nil
	case "pgup", "ctrl+u":
		m.chat.PageUp()
		return nil
	case "pgdown", "ctrl+d":
		m.chat.PageDown()
		return nil
	case "ctrl+p", "shift+tab":
		m.deps.Bus.Emit(input.TopicCommand, ":persona next")
		return nil
	}
	if m.focus == FocusInput {
		return m.input.Update(k)
	}
	switch k.String() {
	case "up", "k":
		m.chat.ScrollBy(-1)
	case "down", "j":
		m.chat.ScrollBy(1)
	case "home":
		m.chat.Top()
	case "end":
		m.chat.Bottom()
	case "y":
		return m.copyMessages(1)
	case "Y":
		return m.copyMessages(0)
	case "ctrl+c":
		return tea.Quit
	}
	return nil
}

func (m *Model) toggleFocus() {
	if m.focus == FocusInput {
		m.focus = FocusMessages
	} else {
		m.focus = FocusInput
	}
}

// copyMessages copies the last n messages (all when n is 0) to the system
// clipboard, through OSC 52 and the native clipboard; either may work.
func (m *Model) copyMessages(n int) tea.Cmd {
	msgs := m.deps.Chat.GetMessages()
	if n > 0 && len(msgs) > n {
		msgs = msgs[len(msgs)-n:]
	}
	var sb strings.Builder
	for _, msg := range msgs {
		fmt.Fprintf(&sb, "[%s]\n%s\n\n", msg.Role, msg.Content)
	}
	text := sb.String()
	if text == "" {
		return nil
	}
	if m.deps.Clipboard != nil {
		_ = m.deps.Clipboard.Copy(text)
	}
	return tea.SetClipboard(text)
}

// applyTheme rebuilds styles and the message formatter after a config or
// theme change.
func (m *Model) applyTheme(msg uimsg.ThemeChanged) tea.Cmd {
	cfg := msg.Config
	if cfg == nil {
		cfg = m.deps.Config.GetConfig()
	}
	m.gen++
	m.theme = style.New(cfg.Theme, m.gen)
	m.showCursor = cfg.IsShowCursorEnabled()
	m.mouse = cfg.IsMouseEnabled()
	if fmtr, err := chatview.NewFormatter(cfg, m.deps.Config.GetTheme()); err == nil {
		m.chat.SetFormatter(fmtr)
	} else {
		m.chat.Sync()
	}
	m.layoutPanels()
	return nil
}

func (m *Model) resize(w, h int) {
	m.w, m.h = w, h
	m.layoutPanels()
}

// layoutPanels recomputes rects and pushes inner sizes to the panels.
func (m *Model) layoutPanels() {
	m.rects = layout.Compute(m.w, m.h, m.lay)
	m.chat.SetSize(inner(m.rects.Messages, m.messagesBorder()))
	m.input.SetWidth(m.rects.Input.W - 2)
}

func (m *Model) messagesBorder() bool {
	return m.deps.Config.GetConfig().IsShowMessagesBorderEnabled()
}

// inner returns the content size of a rect, minus the border if drawn.
func inner(r layout.Rect, border bool) (int, int) {
	if !border {
		return r.W, r.H
	}
	return max(r.W-2, 0), max(r.H-2, 0)
}

// View composes the frame.
func (m *Model) View() tea.View {
	if m.w <= 0 || m.h <= 0 {
		return tea.NewView("")
	}
	r := m.rects
	border := m.messagesBorder()
	messages := m.theme.RenderFrame(r.Messages, style.Frame{
		Title:   m.messagesTitle(border),
		Focused: m.focus == FocusMessages,
		Border:  border,
	}, m.chat.View())
	prompt := m.theme.RenderFrame(r.Input, style.Frame{
		Subtitle: " F4 / Ctrl+V Expand ",
		Focused:  m.focus == FocusInput,
		Border:   true,
	}, m.input.View(m.theme))
	status := lipgloss.JoinHorizontal(lipgloss.Top,
		style.Fit(m.status.Left(m.theme), r.StatusLeft.W, r.StatusLeft.H),
		style.Fit(m.status.Centre(m.theme), r.StatusCenter.W, r.StatusCenter.H),
		style.Fit(m.status.Right(m.theme), r.StatusRight.W, r.StatusRight.H),
	)

	parts := make([]string, 0, 3)
	for _, p := range []string{messages, prompt, status} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, parts...))
	v.AltScreen = true
	if m.mouse {
		v.MouseMode = tea.MouseModeCellMotion
	}
	if m.focus == FocusInput && m.showCursor && r.Input.H >= layout.InputHeight {
		v.Cursor = tea.NewCursor(r.Input.X+1+m.input.CursorX(), r.Input.Y+1)
	}
	return v
}

func (m *Model) messagesTitle(border bool) string {
	if !border {
		return ""
	}
	if m.persona != "" {
		return " " + m.persona + " "
	}
	return " Chat "
}
