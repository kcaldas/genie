package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/history"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/bridge"
	"github.com/kcaldas/genie/cmd/tui/ui/dialog"
	"github.com/kcaldas/genie/cmd/tui/ui/editor"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

// newTestModel builds a root model over in-memory state at 100x30.
func newTestModel(t *testing.T) (*Model, *state.ChatState, *events.CommandEventBus) {
	t.Helper()
	cm, err := helpers.NewConfigManager()
	if err != nil {
		t.Fatal(err)
	}
	chat := state.NewChatState(100)
	bus := events.NewCommandEventBus()
	m, err := New(Deps{
		Commands: func() []dialog.PickerItem {
			return []dialog.PickerItem{{Label: "/compact", Detail: "Summarise", Value: "/compact"}, {Label: ":theme", Detail: "Theme", Value: ":theme"}}
		},
		Files:      func() ([]string, bool) { return []string{"cmd/main.go", "README.md"}, false },
		Dispatcher: bridge.NewDispatcher(),
		Config:     cm,
		Chat:       chat,
		Debug:      state.NewDebugState(),
		State:      state.NewStateAccessor(chat, state.NewUIState()),
		Bus:        bus,
		Prompt:     editor.NewPrompt(nil, history.NewChatHistory("", false)),
		History:    history.NewChatHistory("", false),
		Keys:       DefaultKeymap(),
	})
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m, chat, bus
}

func screen(m *Model) string { return ansi.Strip(m.View().Content) }

func TestRootRendersFullFrame(t *testing.T) {
	m, chat, _ := newTestModel(t)
	chat.AddMessage(types.Message{Role: "system", Content: "welcome aboard"})
	m.Update(uimsg.ChatChanged{})
	m.chat.Sync()
	s := screen(m)
	lines := strings.Split(s, "\n")
	if len(lines) != 30 {
		t.Fatalf("frame has %d rows, want 30", len(lines))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 100 {
			t.Fatalf("row %d is %d wide, want 100: %q", i, w, l)
		}
	}
	if !strings.Contains(s, "welcome aboard") || !strings.Contains(s, "Ready") || !strings.Contains(s, "newline · F4 expand") {
		t.Fatalf("frame missing content:\n%s", s)
	}
}

func TestRightPanelMessagesShowAndHide(t *testing.T) {
	m, _, _ := newTestModel(t)
	m.Update(uimsg.RightPanel{Mode: layout.RightDebug, Action: uimsg.PanelToggle})
	if s := screen(m); !strings.Contains(s, " Debug ") {
		t.Fatalf("debug panel not shown:\n%s", s)
	}
	m.Update(uimsg.RightPanel{Mode: layout.RightText, Action: uimsg.PanelToggle, Title: "Help", Content: "# GENIE(1)\n\nbody", ContentType: "markdown"})
	s := screen(m)
	if !strings.Contains(s, " Help ") || !strings.Contains(s, "GENIE(1)") || strings.Contains(s, " Debug ") {
		t.Fatalf("help panel not shown:\n%s", s)
	}
	m.Update(uimsg.RightPanel{Mode: layout.RightText, Action: uimsg.PanelToggle, Title: "Help", Content: "# GENIE(1)", ContentType: "markdown"})
	if s := screen(m); strings.Contains(s, " Help ") {
		t.Fatalf("toggle did not hide the panel:\n%s", s)
	}
	m.Update(tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl}) // zoom with nothing shown: no-op
	if m.side.Zoomed() {
		t.Fatal("zoom without a panel")
	}
}

func TestContextViewerDialogLifecycle(t *testing.T) {
	m, _, _ := newTestModel(t)
	m.Update(uimsg.OpenContextViewer{Data: map[string]string{"project": "ctx body"}})
	if !m.deps.State.IsContextViewerActive() {
		t.Fatal("state must record the open viewer")
	}
	s := screen(m)
	if !strings.Contains(s, "Context Parts") || !strings.Contains(s, "ctx body") {
		t.Fatalf("viewer not drawn:\n%s", s)
	}
	m.Update(uimsg.ContextData{Data: map[string]string{"project": "refreshed"}})
	if !strings.Contains(screen(m), "refreshed") {
		t.Fatal("refresh data not applied")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.deps.State.IsContextViewerActive() || strings.Contains(screen(m), "Context Parts") {
		t.Fatal("esc must close the viewer and clear the flag")
	}
	m.Update(uimsg.OpenContextViewer{Data: nil})
	m.Update(uimsg.CloseContextViewer{})
	if m.dialogs.Len() != 0 {
		t.Fatal("CloseContextViewer must pop the viewer")
	}
}

func TestFocusCyclesThroughVisiblePanels(t *testing.T) {
	m, _, _ := newTestModel(t)
	tab := tea.KeyPressMsg{Code: tea.KeyTab}
	m.Update(tab)
	if m.focus != FocusMessages {
		t.Fatal("tab from input goes to messages")
	}
	m.Update(tab)
	if m.focus != FocusInput {
		t.Fatal("without a side panel, tab wraps to input")
	}
	m.Update(uimsg.RightPanel{Mode: layout.RightDebug, Action: uimsg.PanelShow})
	m.Update(tab)
	m.Update(tab)
	if m.focus != FocusSide {
		t.Fatal("with a side panel, messages -> side")
	}
	m.Update(uimsg.RightPanel{Mode: layout.RightDebug, Action: uimsg.PanelHide})
	if m.focus != FocusInput {
		t.Fatal("hiding the focused panel returns focus to the prompt")
	}
}

func TestF4ExpandsThePromptToFullHeight(t *testing.T) {
	m, _, bus := newTestModel(t)
	got := make(chan string, 1)
	bus.Subscribe(uimsg.TopicInputText, func(e interface{}) { got <- e.(string) })

	for _, r := range "draft" {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyF4})
	if !m.input.Expanded() || m.rects.Messages.H != 5 {
		t.Fatalf("expanded prompt should leave the chat its minimum rows: expanded=%v chat=%d", m.input.Expanded(), m.rects.Messages.H)
	}
	s := screen(m)
	if !strings.Contains(s, "Esc / F4 collapse") || !strings.Contains(s, "draft") || !strings.Contains(s, "Ready") {
		t.Fatalf("expanded prompt frame:\n%s", s)
	}
	if lines := strings.Split(s, "\n"); len(lines) != 30 {
		t.Fatalf("frame has %d rows, want 30", len(lines))
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	for _, r := range "more" {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	bus.WaitForPendingEvents()
	select {
	case text := <-got:
		if text != "draft\nmore" {
			t.Fatalf("submitted %q", text)
		}
	default:
		t.Fatal("enter did not send")
	}
	if m.input.Expanded() || m.rects.Input.H != layout.InputHeight {
		t.Fatalf("prompt should collapse after sending: expanded=%v h=%d", m.input.Expanded(), m.rects.Input.H)
	}
}

func TestExpandedPromptVimModeShowsInStatus(t *testing.T) {
	m, _, _ := newTestModel(t)
	m.Update(uimsg.VimModeChanged{Enabled: true})
	m.Update(uimsg.ExpandPrompt{Content: "x"})
	if s := screen(m); !strings.Contains(s, "INSERT") || !strings.Contains(s, ":w send") {
		t.Fatalf("vi mode should show in the status and hint:\n%s", s)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if s := screen(m); !strings.Contains(s, "NORMAL") || !m.input.Expanded() {
		t.Fatal("esc in vi goes to normal, does not collapse")
	}
	m.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.input.Expanded() || m.input.Text() != "x" {
		t.Fatal(":q collapses and keeps the text")
	}
}

func TestConfirmationOverlayAnswersThroughTheBus(t *testing.T) {
	m, _, bus := newTestModel(t)
	answers := make(chan uimsg.ConfirmationAnswer, 1)
	bus.Subscribe(uimsg.TopicConfirmationAnswered, func(e interface{}) { answers <- e.(uimsg.ConfirmationAnswer) })

	req := uimsg.ConfirmationRequested{Kind: uimsg.ConfirmTool, ExecutionID: "t1", Title: "Tool: bash", Prompt: "1 - Yes | 2 - No", Content: "rm -rf build", ContentType: "text"}
	m.deps.State.SetWaitingConfirmation(true) // the controller sets this before dispatching
	_, cmd := m.Update(req)
	if cmd == nil {
		t.Fatal("a confirmation must start the status animation")
	}
	s := screen(m)
	if !strings.Contains(s, " Tool: bash ") || !strings.Contains(s, "1 - Yes | 2 - No") || !strings.Contains(s, "rm -rf build") {
		t.Fatalf("overlay not drawn:\n%s", s)
	}
	if !strings.Contains(s, "newline · F4 expand") || !strings.Contains(s, "Your call") {
		t.Fatalf("the prompt frame and status row must remain visible under the overlay:\n%s", s)
	}
	if m.View().Cursor != nil {
		t.Fatal("no text cursor while a dialog is open")
	}
	for _, r := range "bcd" { // keys go to the dialog, not the prompt (a/y/n answer)
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if m.input.Text() != "" {
		t.Fatalf("prompt received keys meant for the dialog: %q", m.input.Text())
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	m.Update(cmd())
	bus.WaitForPendingEvents()
	select {
	case a := <-answers:
		if !a.Confirmed || a.ExecutionID != "t1" || a.Kind != uimsg.ConfirmTool {
			t.Fatalf("answer = %+v", a)
		}
	default:
		t.Fatal("answer not emitted on the bus")
	}
	if m.dialogs.Len() != 0 || strings.Contains(screen(m), "Tool: bash") {
		t.Fatal("dialog must close after answering")
	}
	m.Update(req)
	m.Update(uimsg.ConfirmationClosed{})
	if m.dialogs.Len() != 0 {
		t.Fatal("ConfirmationClosed pops the dialog")
	}
}

func TestPromptGrowsAndShrinksTheLayout(t *testing.T) {
	m, _, _ := newTestModel(t)
	base := m.rects.Input.H
	for _, r := range "a" {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	for i := 0; i < 3; i++ {
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	}
	if m.rects.Input.H != base+3 {
		t.Fatalf("input height = %d, want %d", m.rects.Input.H, base+3)
	}
	if lines := strings.Split(screen(m), "\n"); len(lines) != 30 {
		t.Fatalf("frame still %d rows", len(lines))
	}
	v := m.View()
	if v.Cursor == nil || v.Cursor.Y != m.rects.Input.Y+1+3 {
		t.Fatalf("cursor should sit on the fourth prompt row: %+v", v.Cursor)
	}
	m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}) // clears
	if m.rects.Input.H != base {
		t.Fatalf("input height after clear = %d, want %d", m.rects.Input.H, base)
	}
}

// run applies a message and then any command it returned, synchronously,
// which is how the picker's follow-up messages reach the model in tests.
func run(m *Model, msg tea.Msg) {
	_, cmd := m.Update(msg)
	for cmd != nil {
		next := cmd()
		if next == nil {
			return
		}
		if batch, ok := next.(tea.BatchMsg); ok {
			for _, c := range batch {
				if c != nil {
					run(m, c())
				}
			}
			return
		}
		_, cmd = m.Update(next)
	}
}

func TestSlashOpensCommandPaletteAndChoiceFillsPrompt(t *testing.T) {
	m, _, _ := newTestModel(t)
	run(m, tea.KeyPressMsg{Code: '/', Text: "/"})
	s := screen(m)
	if !strings.Contains(s, " Commands ") || !strings.Contains(s, "/compact") || !strings.Contains(s, ":theme") {
		t.Fatalf("palette not shown:\n%s", s)
	}
	for _, r := range "thm" {
		run(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	run(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.dialogs.Len() != 0 || m.input.Text() != ":theme " {
		t.Fatalf("after choosing: dialogs=%d prompt=%q", m.dialogs.Len(), m.input.Text())
	}
	// A '/' typed into a non-empty prompt is just a character.
	run(m, tea.KeyPressMsg{Code: '/', Text: "/"})
	if m.dialogs.Len() != 0 {
		t.Fatal("'/' mid-prompt must not open the palette")
	}
}

func TestEscapeKeepsTheTypedSlash(t *testing.T) {
	m, _, _ := newTestModel(t)
	run(m, tea.KeyPressMsg{Code: '/', Text: "/"})
	run(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.dialogs.Len() != 0 || m.input.Text() != "/" {
		t.Fatalf("after esc: dialogs=%d prompt=%q", m.dialogs.Len(), m.input.Text())
	}
	run(m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}) // clear
	run(m, tea.KeyPressMsg{Code: '@', Text: "@"})
	for _, r := range "src/new.go" {
		run(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	run(m, tea.KeyPressMsg{Code: tea.KeyEnter}) // no match: keep what was typed
	if m.input.Text() != "@src/new.go" {
		t.Fatalf("a path with no match must survive: %q", m.input.Text())
	}
}

func TestTabCyclesFocusWhenNothingToComplete(t *testing.T) {
	m, _, _ := newTestModel(t)
	run(m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focus != FocusMessages {
		t.Fatal("tab with nothing to complete cycles focus")
	}
}

func TestPromptHintFollowsKeyboardSupport(t *testing.T) {
	m, _, _ := newTestModel(t)
	if !strings.Contains(screen(m), "or Alt+Enter newline") {
		t.Fatal("without kitty keys the hint names Alt+Enter")
	}
	run(m, tea.KeyboardEnhancementsMsg{Flags: 1})
	if !strings.Contains(screen(m), "Shift+Enter newline") {
		t.Fatalf("with disambiguation the hint names Shift+Enter:\n%s", screen(m))
	}
}

func TestAtOpensFilePickerAndInsertsPath(t *testing.T) {
	m, _, _ := newTestModel(t)
	for _, r := range "see " {
		run(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	run(m, tea.KeyPressMsg{Code: '@', Text: "@"})
	s := screen(m)
	if !strings.Contains(s, " Files ") || !strings.Contains(s, "README.md") {
		t.Fatalf("file picker not shown with files:\n%s", s)
	}
	for _, r := range "read" {
		run(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	run(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.input.Text() != "see @README.md " {
		t.Fatalf("prompt = %q", m.input.Text())
	}
	if lines := strings.Split(screen(m), "\n"); len(lines) != 30 {
		t.Fatalf("frame has %d rows after the picker closed", len(lines))
	}
}

func TestHistorySearchPickerAndExternalEditor(t *testing.T) {
	m, _, _ := newTestModel(t)
	m.deps.History.AddCommand("older prompt")
	m.deps.History.AddCommand("multi\nline prompt")
	m.deps.History.AddCommand("older prompt") // repeat: listed once
	edited := "edited outside"
	m.deps.EditExternally = func(text string) tea.Cmd {
		return func() tea.Msg { return uimsg.ExternalEditDone{Text: text + " + " + edited + "\n"} }
	}

	run(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	s := screen(m)
	if !strings.Contains(s, " History ") || !strings.Contains(s, "multi line prompt") || !strings.Contains(s, "2 lines") {
		t.Fatalf("history picker:\n%s", s)
	}
	if strings.Count(s, "older prompt") != 1 {
		t.Fatalf("history entries must be unique:\n%s", s)
	}
	run(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.input.Text() != "older prompt" {
		t.Fatalf("chosen history = %q", m.input.Text())
	}

	run(m, tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl})
	if m.input.Text() != "older prompt + edited outside" {
		t.Fatalf("after external edit = %q", m.input.Text())
	}
	m.deps.EditExternally = func(string) tea.Cmd {
		return func() tea.Msg { return uimsg.ExternalEditDone{Text: "x", Err: errFake} }
	}
	run(m, tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl})
	if m.input.Text() != "older prompt + edited outside" {
		t.Fatal("a failed external edit leaves the prompt alone")
	}
}

type fakeErr struct{}

func (fakeErr) Error() string { return "editor failed" }

var errFake error = fakeErr{}

func TestUndoRedoKeys(t *testing.T) {
	m, _, _ := newTestModel(t)
	for _, r := range "abc" {
		run(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	run(m, tea.KeyPressMsg{Code: 'z', Mod: tea.ModAlt})
	if m.input.Text() != "" {
		t.Fatalf("alt+z undo = %q", m.input.Text())
	}
	run(m, tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
	if m.input.Text() != "abc" {
		t.Fatalf("ctrl+y redo = %q", m.input.Text())
	}
}

func TestCtrlOAndClickToggleToolCards(t *testing.T) {
	m, chat, _ := newTestModel(t)
	chat.AddMessage(types.Message{Role: "assistant", ContentType: "tool", Content: "bash ls",
		Tool: &types.ToolCall{Name: "bash", Args: "ls", Success: true, Summary: "2 lines", Output: "a.go\nb.go"}})
	m.Update(uimsg.ChatChanged{})
	m.chat.Sync()
	if s := screen(m); !strings.Contains(s, "bash ls") || strings.Contains(s, "a.go") {
		t.Fatalf("collapsed card expected:\n%s", s)
	}
	m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	if s := screen(m); !strings.Contains(s, "a.go") {
		t.Fatalf("ctrl+o should expand:\n%s", s)
	}
	m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	// Click the header: row 1 inside the bordered chat frame.
	m.Update(tea.MouseClickMsg{X: 5, Y: m.rects.Messages.Y + 1, Button: tea.MouseLeft})
	if s := screen(m); !strings.Contains(s, "a.go") {
		t.Fatalf("click should expand the card:\n%s", s)
	}
}

func TestTodosOpenThePanelOnceAndCtrlTToggles(t *testing.T) {
	m, _, _ := newTestModel(t)
	items := []presentation.TodoItem{{ID: "1", Content: "first task", Status: "pending", Priority: "high"}}
	m.Update(uimsg.TodosUpdated{Items: items})
	if !m.side.Visible() || !strings.Contains(screen(m), "first task") || !strings.Contains(screen(m), "Todos 0/1") {
		t.Fatalf("first todo list should open the panel:\n%s", screen(m))
	}
	m.Update(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	if m.side.Visible() {
		t.Fatal("ctrl+t hides the todo panel")
	}
	m.Update(uimsg.TodosUpdated{Items: append(items, presentation.TodoItem{ID: "2", Content: "second", Status: "completed"})})
	if m.side.Visible() {
		t.Fatal("later updates must not reopen a panel the user closed")
	}
	m.Update(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	if !strings.Contains(screen(m), "Todos 1/2") || !strings.Contains(screen(m), "second") {
		t.Fatalf("ctrl+t shows the updated list:\n%s", screen(m))
	}
}
