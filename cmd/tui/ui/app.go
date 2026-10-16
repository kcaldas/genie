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
	"github.com/kcaldas/genie/cmd/history"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
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
	Prompt     *editor.Prompt
	History    history.ChatHistory
	Clipboard  Clipboard
	Keys       Keymap
	Now        func() time.Time // nil for the wall clock

	// Commands lists the palette entries (':' and '/' commands).
	Commands func() []dialog.PickerItem
	// Files lists workspace files for the @ picker and whether the list was
	// cut short; it may be slow, so it runs as a command.
	Files func() (paths []string, truncated bool)
	// EditExternally hands text to the user's editor and returns a command
	// that yields uimsg.ExternalEditDone. nil disables Ctrl+G.
	EditExternally func(text string) tea.Cmd
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

	theme *style.Theme
	gen   uint64

	chat    *chatview.Model
	input   *input.Model
	status  *statusbar.Model
	side    *sidepanel.Model
	dialogs dialog.Stack
	keys    Keymap

	focus     Focus
	prefs     prefs
	inputRows int  // prompt text rows the current layout was computed for
	kittyKeys bool // the terminal reports Shift+Enter distinctly
}

// prefs are the display settings mirrored from config, kept in one place
// so a theme reload and the live toggles update the same fields.
type prefs struct {
	showCursor bool
	mouse      bool
	persona    string
}

func (p *prefs) fromConfig(cfg *types.Config) {
	p.showCursor = cfg.IsShowCursorEnabled()
	p.mouse = cfg.IsMouseEnabled()
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
		deps:   d,
		theme:  style.New(cfg.Theme, 0),
		chat:   chatview.New(d.Chat, fmtr),
		input:  input.New(d.Prompt, d.Bus, d.Clipboard),
		status: statusbar.New(d.State, d.Now),
		side:   sidepanel.New(d.Config, d.Debug),
		keys:   d.Keys,
	}
	m.input.SetVim(cfg.VimMode)
	m.prefs.fromConfig(cfg)
	m.chat.SetStyling(chatview.Styling{Theme: m.theme, Cfg: d.Config})
	return m, nil
}

// Init starts listening for controller messages and the status animation.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.deps.Dispatcher.Next(), m.status.Start())
}

// Update applies one message: terminal input here, controller messages in
// applyUIMsg.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case bridge.Envelope:
		// Unwrap, apply, and re-arm Next exactly once per delivery.
		_, cmd := m.Update(msg.Msg)
		return m, tea.Batch(cmd, m.deps.Dispatcher.Next())
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyboardEnhancementsMsg:
		m.kittyKeys = msg.SupportsKeyDisambiguation()
		return m, nil
	case tea.KeyPressMsg:
		if m.dialogs.Len() > 0 {
			return m, m.dialogKey(msg)
		}
		cmd := m.key(msg)
		m.fitPrompt()
		return m, cmd
	case tea.MouseWheelMsg:
		m.wheel(msg)
		return m, nil
	case tea.MouseClickMsg:
		m.click(msg)
		return m, nil
	case tea.PasteMsg:
		if m.dialogs.Len() > 0 {
			return m, m.dialogs.Update(msg)
		}
		if m.focus == FocusInput {
			cmd := m.input.Update(msg)
			m.fitPrompt()
			return m, cmd
		}
		return m, nil
	}
	return m, m.applyUIMsg(msg)
}

// fitPrompt re-lays the screen out when the prompt grew or shrank.
func (m *Model) fitPrompt() {
	if m.input.Rows() != m.inputRows {
		m.layoutPanels()
	}
}

// applyUIMsg handles what controllers and dialogs send. Anything else is
// offered to the chat and status models, which own their own ticks.
func (m *Model) applyUIMsg(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case uimsg.RightPanel:
		m.applyRightPanel(msg)
	case uimsg.DebugChanged:
		m.side.DebugChanged()
	case uimsg.OpenContextViewer:
		m.dialogs.Push(dialog.NewContextViewer(msg.Data))
		m.deps.State.SetContextViewerActive(true)
	case uimsg.ContextData:
		if v, ok := m.dialogs.Top().(*dialog.ContextViewer); ok {
			v.SetData(msg.Data)
		}
	case uimsg.CloseContextViewer:
		if _, ok := m.dialogs.Top().(*dialog.ContextViewer); ok {
			m.dialogs.Pop()
			m.deps.State.SetContextViewerActive(false)
		}
	case dialog.ContextRefreshMsg:
		m.deps.Bus.Emit(uimsg.TopicContextRefresh, nil)
	case uimsg.ConfirmationRequested:
		m.dialogs.Push(dialog.NewConfirm(msg, m.deps.Config))
		return m.status.Start()
	case uimsg.ConfirmationClosed:
		if _, ok := m.dialogs.Top().(*dialog.Confirm); ok {
			m.dialogs.Pop()
		}
	case dialog.ConfirmAnswerMsg:
		m.deps.Bus.Emit(uimsg.TopicConfirmationAnswered, msg.Answer)
	case uimsg.OpenPalette:
		return m.openPalette(msg.Kind)
	case paletteFilesMsg:
		if p, ok := m.dialogs.Top().(*dialog.Picker); ok && p.Kind() == uimsg.PaletteFiles {
			p.SetItems(msg.items)
			if msg.truncated {
				p.SetTitle(fmt.Sprintf("Files (first %d)", len(msg.items)))
			}
		}
	case dialog.PickerChosenMsg:
		m.applyPick(msg)
	case dialog.PickerCancelledMsg:
		// The trigger character stays in the prompt; keep what was typed
		// into the picker too, so a path or command can be finished by hand.
		if msg.Query != "" {
			m.input.Insert(msg.Query)
			m.fitPrompt()
		}
	case uimsg.OpenExternalEditor:
		if m.deps.EditExternally != nil {
			return m.deps.EditExternally(msg.Text)
		}
	case uimsg.ExternalEditDone:
		if msg.Err != nil {
			m.deps.State.AddMessage(types.Message{Role: "error", Content: "External editor failed: " + msg.Err.Error()})
			return m.chat.Update(uimsg.ChatChanged{})
		}
		m.input.Replace(strings.TrimRight(msg.Text, "\n"))
		m.fitPrompt()
	case uimsg.ExpandPrompt:
		if msg.Content != "" {
			m.input.Insert(msg.Content)
		}
		m.input.SetExpanded(true)
		m.fitPrompt()
	case uimsg.VimModeChanged:
		m.input.SetVim(msg.Enabled)
	case uimsg.PersonaChanged:
		m.prefs.persona = msg.Name
	case uimsg.ThemeChanged:
		m.applyTheme(msg)
	case uimsg.CursorChanged:
		m.prefs.showCursor = msg.Visible
	case uimsg.MouseChanged:
		m.prefs.mouse = msg.Enabled
	case uimsg.ToggleDetails:
		m.chat.ToggleDetails()
	case uimsg.Quit:
		return tea.Quit
	default:
		return tea.Batch(m.chat.Update(msg), m.status.Update(msg))
	}
	return nil
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
		m.deps.Bus.Emit(uimsg.TopicShortcutHelp, nil)
	case key.Matches(k, m.keys.Debug):
		m.deps.Bus.Emit(uimsg.TopicDebugView, nil)
	case key.Matches(k, m.keys.Write):
		m.input.ToggleExpanded()
		m.focus = FocusInput
		m.fitPrompt()
	case key.Matches(k, m.keys.Zoom):
		m.side.ToggleZoom()
		m.layoutPanels()
	case key.Matches(k, m.keys.Context):
		m.deps.Bus.Emit(uimsg.TopicInputCommand, ":context")
	case key.Matches(k, m.keys.NextPanel):
		if m.focus == FocusInput && m.input.Complete() {
			return nil
		}
		m.nextFocus()
	case key.Matches(k, m.keys.PersonaNext):
		m.deps.Bus.Emit(uimsg.TopicInputCommand, ":persona next")
	case key.Matches(k, m.keys.PageUp):
		m.activeScrollable().PageUp()
	case key.Matches(k, m.keys.PageDown):
		m.activeScrollable().PageDown()
	case key.Matches(k, m.keys.Details):
		m.chat.ToggleDetails()
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
		if key.Matches(k, m.keys.Quit) {
			return tea.Quit
		}
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

// click toggles the tool or thinking card under the pointer, if any.
func (m *Model) click(msg tea.MouseClickMsg) {
	if msg.Button != tea.MouseLeft || m.dialogs.Len() > 0 || !m.rects.Messages.Contains(msg.X, msg.Y) {
		return
	}
	y := msg.Y - m.rects.Messages.Y
	if m.messagesBorder() {
		y--
	}
	if y >= 0 {
		m.chat.ToggleAt(y)
	}
}

// applyRightPanel updates the right panel from a controller message.
func (m *Model) applyRightPanel(msg uimsg.RightPanel) {
	if msg.Content != "" || msg.Title != "" {
		switch msg.Mode {
		case layout.RightText:
			m.side.SetText(msg.Title, msg.Content, msg.ContentType)
		case layout.RightDiff:
			m.side.SetDiff(msg.Title, msg.Content)
		}
	}
	switch msg.Action {
	case uimsg.PanelShow:
		m.side.Show(msg.Mode)
	case uimsg.PanelHide:
		m.side.Hide()
	case uimsg.PanelToggle:
		m.side.Toggle(msg.Mode)
	case uimsg.PanelToggleZoom:
		m.side.ToggleZoom()
	}
	if !m.side.Visible() && m.focus == FocusSide {
		m.focus = FocusInput
	}
	m.layoutPanels()
}

// paletteFilesMsg delivers the file list to an open file picker.
type paletteFilesMsg struct {
	items     []dialog.PickerItem
	truncated bool
}

// openPalette pushes a picker above the prompt. The command palette is
// filled at once; the file picker loads its list in the background.
func (m *Model) openPalette(kind string) tea.Cmd {
	if _, open := m.dialogs.Top().(*dialog.Picker); open {
		return nil
	}
	var p *dialog.Picker
	var cmd tea.Cmd
	switch kind {
	case uimsg.PaletteCommands:
		var items []dialog.PickerItem
		if m.deps.Commands != nil {
			items = m.deps.Commands()
		}
		p = dialog.NewPicker(kind, "Commands", items)
	case uimsg.PaletteHistory:
		p = dialog.NewPicker(kind, "History", m.historyItems())
	case uimsg.PaletteFiles:
		p = dialog.NewPicker(kind, "Files", nil)
		list := m.deps.Files
		cmd = func() tea.Msg {
			items := []dialog.PickerItem{}
			truncated := false
			if list != nil {
				var paths []string
				paths, truncated = list()
				for _, f := range paths {
					items = append(items, dialog.PickerItem{Label: f, Value: f})
				}
			}
			return paletteFilesMsg{items: items, truncated: truncated}
		}
	default:
		return nil
	}
	p.SetAnchor(m.rects.Input.Y)
	m.dialogs.Push(p)
	return cmd
}

// applyPick puts the chosen entry into the prompt: a command replaces the
// typed '/', a file path follows the typed '@'.
func (m *Model) applyPick(msg dialog.PickerChosenMsg) {
	switch msg.Kind {
	case uimsg.PaletteCommands:
		m.input.Replace(msg.Item.Value + " ")
	case uimsg.PaletteFiles:
		m.input.Insert(msg.Item.Value + " ")
	case uimsg.PaletteHistory:
		m.input.Replace(msg.Item.Value)
	}
	m.fitPrompt()
}

// historyItems lists past prompts newest first, without repeats, with
// multi-line entries shown on one line.
func (m *Model) historyItems() []dialog.PickerItem {
	items := []dialog.PickerItem{}
	if m.deps.History == nil {
		return items
	}
	seen := map[string]bool{}
	entries := m.deps.History.GetHistory()
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e == "" || seen[e] {
			continue
		}
		seen[e] = true
		label := strings.Join(strings.Fields(e), " ")
		detail := ""
		if n := strings.Count(e, "\n"); n > 0 {
			detail = fmt.Sprintf("%d lines", n+1)
		}
		items = append(items, dialog.PickerItem{Label: label, Detail: detail, Value: e})
	}
	return items
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
func (m *Model) applyTheme(msg uimsg.ThemeChanged) {
	cfg := msg.Config
	if cfg == nil {
		cfg = m.deps.Config.GetConfig()
	}
	m.gen++
	m.theme = style.New(cfg.Theme, m.gen)
	m.prefs.fromConfig(cfg)
	if fmtr, err := chatview.NewFormatter(cfg, m.deps.Config.GetTheme()); err == nil {
		m.chat.SetFormatter(fmtr)
	}
	m.chat.SetStyling(chatview.Styling{Theme: m.theme, Cfg: m.deps.Config})
	m.side.ThemeChanged()
	m.layoutPanels()
}

func (m *Model) resize(w, h int) {
	m.w, m.h = w, h
	m.layoutPanels()
}

// layoutPanels recomputes rects and pushes inner sizes to the panels.
func (m *Model) layoutPanels() {
	m.input.SetWidth(m.w - 2)
	m.input.SetMaxRows(0) // measure the full need first
	m.inputRows = m.input.Rows()
	m.rects = layout.Compute(m.w, m.h, layout.State{Right: m.side.Mode(), Zoomed: m.side.Zoomed(), InputRows: m.inputRows})
	m.input.SetMaxRows(m.rects.Input.H - 2)
	m.chat.SetSize(inner(m.rects.Messages, m.messagesBorder()))
	m.side.SetSize(inner(m.rects.Right, true))
	m.dialogs.SetSize(m.w, m.h)
	if p, ok := m.dialogs.Top().(*dialog.Picker); ok {
		p.SetAnchor(m.rects.Input.Y)
	}
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
	if top := m.dialogs.Top(); top != nil && top.Fullscreen() {
		content := top.View(m.theme)
		if top.ShowsStatus() {
			content = lipgloss.JoinVertical(lipgloss.Left,
				style.Fit(content, m.w, m.h-layout.StatusHeight), m.statusRow())
		}
		v := tea.NewView(style.Fit(content, m.w, m.h))
		v.AltScreen = true
		v.Cursor = top.Cursor()
		return v
	}
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
		Subtitle: m.input.Hint(m.kittyKeys),
		Focused:  m.focus == FocusInput,
		Border:   true,
	}, m.input.View(m.theme))
	parts := make([]string, 0, 3)
	for _, p := range []string{messages, prompt, m.statusRow()} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	overlay := m.dialogs.Top()
	if o, ok := overlay.(floating); ok {
		rect := o.Rect()
		content = lipgloss.NewCompositor(
			lipgloss.NewLayer(content),
			lipgloss.NewLayer(o.View(m.theme)).X(rect.X).Y(rect.Y).Z(1),
		).Render()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	if m.prefs.mouse {
		v.MouseMode = tea.MouseModeCellMotion
	}
	if overlay == nil && m.focus == FocusInput && m.prefs.showCursor && r.Input.H >= layout.InputHeight {
		cx, cy := m.input.CursorPos()
		v.Cursor = tea.NewCursor(r.Input.X+1+cx, r.Input.Y+1+cy)
	}
	return v
}

// floating is a dialog drawn over the base frame at its own rectangle.
type floating interface {
	dialog.Dialog
	Rect() layout.Rect
}

// statusRow renders the three status segments at their rects' widths.
// While write mode is open the centre shows the vi mode.
func (m *Model) statusRow() string {
	r := m.rects
	m.status.SetCentre(strings.Trim(m.input.ModeLabel(), "[]"))
	return lipgloss.JoinHorizontal(lipgloss.Top,
		style.Fit(m.status.Left(m.theme), r.StatusLeft.W, r.StatusLeft.H),
		style.Fit(m.status.Centre(m.theme), r.StatusCenter.W, r.StatusCenter.H),
		style.Fit(m.status.Right(m.theme), r.StatusRight.W, r.StatusRight.H),
	)
}

func (m *Model) messagesTitle(border bool) string {
	if !border {
		return ""
	}
	if m.prefs.persona != "" {
		return " " + m.prefs.persona + " "
	}
	return " Chat "
}
