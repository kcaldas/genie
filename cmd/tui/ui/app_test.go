package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/history"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/bridge"
	"github.com/kcaldas/genie/cmd/tui/ui/editor"
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
		Dispatcher: bridge.NewDispatcher(),
		Config:     cm,
		Chat:       chat,
		Debug:      state.NewDebugState(),
		State:      state.NewStateAccessor(chat, state.NewUIState()),
		Bus:        bus,
		Line:       editor.NewLine(nil, history.NewChatHistory("", false)),
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
	if !strings.Contains(s, "welcome aboard") || !strings.Contains(s, "Ready") || !strings.Contains(s, "F4 / Ctrl+V Expand") {
		t.Fatalf("frame missing content:\n%s", s)
	}
}

func TestRightPanelMessagesShowAndHide(t *testing.T) {
	m, _, _ := newTestModel(t)
	m.Update(uimsg.RightPanel{Mode: uimsg.PanelDebug, Action: uimsg.PanelToggle})
	if s := screen(m); !strings.Contains(s, " Debug ") {
		t.Fatalf("debug panel not shown:\n%s", s)
	}
	m.Update(uimsg.RightPanel{Mode: uimsg.PanelText, Action: uimsg.PanelToggle, Title: "Help", Content: "# GENIE(1)\n\nbody", ContentType: "markdown"})
	s := screen(m)
	if !strings.Contains(s, " Help ") || !strings.Contains(s, "GENIE(1)") || strings.Contains(s, " Debug ") {
		t.Fatalf("help panel not shown:\n%s", s)
	}
	m.Update(uimsg.RightPanel{Mode: uimsg.PanelText, Action: uimsg.PanelToggle, Title: "Help", Content: "# GENIE(1)", ContentType: "markdown"})
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
	m.Update(uimsg.RightPanel{Mode: uimsg.PanelDebug, Action: uimsg.PanelShow})
	m.Update(tab)
	m.Update(tab)
	if m.focus != FocusSide {
		t.Fatal("with a side panel, messages -> side")
	}
	m.Update(uimsg.RightPanel{Mode: uimsg.PanelDebug, Action: uimsg.PanelHide})
	if m.focus != FocusInput {
		t.Fatal("hiding the focused panel returns focus to the prompt")
	}
}
