// Package ui is the Bubble Tea root of the genie TUI. The Model owns the
// screen: it lays panels out, routes input by focus, and applies the
// messages controllers dispatch through the bridge.
package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/ui/bridge"
	"github.com/kcaldas/genie/cmd/tui/ui/chatview"
	"github.com/kcaldas/genie/cmd/tui/ui/dialog"
	"github.com/kcaldas/genie/cmd/tui/ui/editor"
	"github.com/kcaldas/genie/cmd/tui/ui/input"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
	"github.com/kcaldas/genie/cmd/tui/ui/sidepanel"
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
	Debug      *state.DebugState
	State      *state.StateAccessor
	Bus        *events.CommandEventBus
	Line       *editor.Line
	Clipboard  Clipboard
	Keys       Keymap
	Now        func() time.Time // nil for the wall clock
}

// Focus names the panel receiving keys.
type Focus int

const (
	FocusInput Focus = iota
	FocusMessages
	FocusSide
)

// Model is the root tea.Model.
type Model struct {
	deps  Deps
	w, h  int
	rects layout.Rects
	lay   layout.State

	theme *style.Theme
	gen   uint64

	chat    *chatview.Model
	input   *input.Model
	status  *statusbar.Model
	side    *sidepanel.Model
	dialogs dialog.Stack
	keys    Keymap

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
		side:       sidepanel.New(d.Config, d.Debug),
		keys:       d.Keys,
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
		if m.dialogs.Len() > 0 {
			return m, m.dialogKey(msg)
		}
		return m, m.key(msg)
	case tea.MouseWheelMsg:
		m.wheel(msg)
		return m, nil
	case tea.PasteMsg:
		if m.dialogs.Len() > 0 {
			return m, m.dialogs.Update(msg)
		}
		if m.focus == FocusInput {
			return m, m.input.Update(msg)
		}
		return m, nil
	case dialog.ContextRefreshMsg:
		m.deps.Bus.Emit("context.refresh", nil)
		return m, nil
	case uimsg.RightPanel:
		m.applyRightPanel(msg)
		return m, nil
	case uimsg.DebugChanged:
		m.side.DebugChanged()
		return m, nil
	case uimsg.OpenContextViewer:
		m.dialogs.Push(dialog.NewContextViewer(msg.Data))
		m.deps.State.SetContextViewerActive(true)
		return m, nil
	case uimsg.ContextData:
		if v, ok := m.dialogs.Top().(*dialog.ContextViewer); ok {
			v.SetData(msg.Data)
		}
		return m, nil
	case uimsg.CloseContextViewer:
		if _, ok := m.dialogs.Top().(*dialog.ContextViewer); ok {
			m.dialogs.Pop()
			m.deps.State.SetContextViewerActive(false)
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

// dialogKey sends a key to the open dialog and tidies up when it closes.
func (m *Model) dialogKey(k tea.KeyPressMsg) tea.Cmd {
	_, wasContext := m.dialogs.Top().(*dialog.ContextViewer)
	cmd := m.dialogs.Update(k)
	if wasContext && m.dialogs.Len() == 0 {
		m.deps.State.SetContextViewerActive(false)
	}
	return cmd
}

// key routes a key press: global shortcuts first, then the focused panel.
// A panel's own binding wins over a global one for the same key, so the
// prompt keeps up/down for history while the chat uses them to scroll.
func (m *Model) key(k tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(k, m.keys.Help):
		m.deps.Bus.Emit("shortcut.help", nil)
	case key.Matches(k, m.keys.Debug):
		m.deps.Bus.Emit("debug.view", nil)
	case key.Matches(k, m.keys.Zoom):
		m.side.ToggleZoom()
		m.layoutPanels()
	case key.Matches(k, m.keys.Context):
		m.deps.Bus.Emit(input.TopicCommand, ":context")
	case key.Matches(k, m.keys.NextPanel):
		m.nextFocus()
	case key.Matches(k, m.keys.PersonaNext):
		m.deps.Bus.Emit(input.TopicCommand, ":persona next")
	case key.Matches(k, m.keys.PageUp):
		m.activeScrollable().PageUp()
	case key.Matches(k, m.keys.PageDown):
		m.activeScrollable().PageDown()
	default:
		return m.focusedKey(k)
	}
	return nil
}

// scrollable is what PgUp/PgDn and the wheel act on.
type scrollable interface {
	ScrollBy(n int)
	PageUp()
	PageDown()
}

// activeScrollable is the right panel while one is shown, else the chat,
// matching the old "active scrollable" rule.
func (m *Model) activeScrollable() scrollable {
	if m.side.Visible() {
		return m.side
	}
	return m.chat
}

func (m *Model) focusedKey(k tea.KeyPressMsg) tea.Cmd {
	switch m.focus {
	case FocusInput:
		return m.input.Update(k)
	case FocusSide:
		if topic := m.side.Update(k); topic != "" {
			m.deps.Bus.Emit(topic, "")
		}
		return nil
	}
	switch {
	case key.Matches(k, m.keys.LineUp):
		m.chat.ScrollBy(-1)
	case key.Matches(k, m.keys.LineDown):
		m.chat.ScrollBy(1)
	case key.Matches(k, m.keys.Top):
		m.chat.Top()
	case key.Matches(k, m.keys.Bottom):
		m.chat.Bottom()
	case key.Matches(k, m.keys.CopyLast):
		return m.copyMessages(1)
	case key.Matches(k, m.keys.CopyAll):
		return m.copyMessages(0)
	case key.Matches(k, m.keys.Quit):
		return tea.Quit
	}
	return nil
}

// nextFocus moves focus through the visible panels in order.
func (m *Model) nextFocus() {
	switch m.focus {
	case FocusInput:
		m.focus = FocusMessages
	case FocusMessages:
		if m.side.Visible() {
			m.focus = FocusSide
		} else {
			m.focus = FocusInput
		}
	default:
		m.focus = FocusInput
	}
}

// wheel scrolls whichever panel is under the pointer.
func (m *Model) wheel(msg tea.MouseWheelMsg) {
	n := 0
	switch msg.Button {
	case tea.MouseWheelUp:
		n = -wheelLines
	case tea.MouseWheelDown:
		n = wheelLines
	default:
		return
	}
	if m.side.Visible() && m.rects.Right.Contains(msg.X, msg.Y) {
		m.side.ScrollBy(n)
		return
	}
	m.chat.ScrollBy(n)
}

// applyRightPanel updates the right panel from a controller message.
func (m *Model) applyRightPanel(msg uimsg.RightPanel) {
	mode := layout.RightNone
	switch msg.Mode {
	case uimsg.PanelText:
		mode = layout.RightText
		if msg.Content != "" || msg.Title != "" {
			m.side.SetText(msg.Title, msg.Content, msg.ContentType)
		}
	case uimsg.PanelDiff:
		mode = layout.RightDiff
		if msg.Content != "" || msg.Title != "" {
			m.side.SetDiff(msg.Title, msg.Content)
		}
	case uimsg.PanelDebug:
		mode = layout.RightDebug
	}
	switch msg.Action {
	case uimsg.PanelShow:
		m.side.Show(mode)
	case uimsg.PanelHide:
		m.side.Hide()
	case uimsg.PanelToggle:
		m.side.Toggle(mode)
	case uimsg.PanelToggleZoom:
		m.side.ToggleZoom()
	}
	if !m.side.Visible() && m.focus == FocusSide {
		m.focus = FocusInput
	}
	m.layoutPanels()
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
	m.side.ThemeChanged()
	m.layoutPanels()
	return nil
}

func (m *Model) resize(w, h int) {
	m.w, m.h = w, h
	m.layoutPanels()
}

// layoutPanels recomputes rects and pushes inner sizes to the panels.
func (m *Model) layoutPanels() {
	m.lay = layout.State{Right: m.side.Mode(), Zoomed: m.side.Zoomed()}
	m.rects = layout.Compute(m.w, m.h, m.lay)
	m.chat.SetSize(inner(m.rects.Messages, m.messagesBorder()))
	m.side.SetSize(inner(m.rects.Right, true))
	m.input.SetWidth(m.rects.Input.W - 2)
	m.dialogs.SetSize(m.w, m.h)
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
	if top := m.dialogs.Top(); top != nil && top.Fullscreen() {
		v := tea.NewView(style.Fit(top.View(m.theme), m.w, m.h))
		v.AltScreen = true
		v.Cursor = top.Cursor()
		return v
	}
	r := m.rects
	border := m.messagesBorder()
	messages := m.theme.RenderFrame(r.Messages, style.Frame{
		Title:   m.messagesTitle(border),
		Focused: m.focus == FocusMessages,
		Border:  border,
	}, m.chat.View())
	if m.side.Visible() {
		side := m.theme.RenderFrame(r.Right, style.Frame{
			Title:   m.side.Title(),
			Focused: m.focus == FocusSide,
			Border:  true,
		}, m.side.View())
		messages = lipgloss.JoinHorizontal(lipgloss.Top, messages, side)
	}
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
