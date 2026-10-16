package sidepanel

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
)

type fakeCfg struct{ cfg types.Config }

func (f *fakeCfg) GetConfig() *types.Config { return &f.cfg }
func (f *fakeCfg) GetTheme() *types.Theme {
	return presentation.GetThemeForMode(f.cfg.Theme, "true")
}

func newPanel(t *testing.T) (*Model, *state.DebugState) {
	t.Helper()
	dbg := state.NewDebugState()
	m := New(&fakeCfg{cfg: types.Config{Theme: "default", GlamourTheme: "auto", DiffTheme: "auto", MarkdownRendering: "enabled"}}, dbg)
	m.SetSize(30, 4)
	return m, dbg
}

func key(s string) tea.KeyPressMsg {
	switch s {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}

func TestHiddenByDefaultAndToggle(t *testing.T) {
	m, _ := newPanel(t)
	if m.Visible() || m.View() != "" || m.Title() != "" {
		t.Fatal("new panel must be hidden")
	}
	m.Toggle(layout.RightText)
	if m.Mode() != layout.RightText {
		t.Fatal("toggle should show text")
	}
	m.Toggle(layout.RightDiff)
	if m.Mode() != layout.RightDiff {
		t.Fatal("toggle to another mode switches")
	}
	m.Toggle(layout.RightDiff)
	if m.Visible() {
		t.Fatal("toggle on the visible mode hides")
	}
	m.ToggleZoom()
	if m.Zoomed() {
		t.Fatal("zoom needs a visible panel")
	}
	m.Show(layout.RightDebug)
	m.ToggleZoom()
	m.Hide()
	if m.Zoomed() {
		t.Fatal("hide resets zoom")
	}
}

func TestTextViewerScrollsFiveLinesPerPage(t *testing.T) {
	m, _ := newPanel(t)
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	m.Show(layout.RightText)
	m.SetText("Help", strings.Join(lines, "\n"), "text")
	if got := m.View(); !strings.HasPrefix(got, "line 0") {
		t.Fatalf("view starts at %q", got)
	}
	m.Update(key("pgdown"))
	if got := m.View(); !strings.HasPrefix(got, "line 5") {
		t.Fatalf("pgdown = %q", got)
	}
	m.Update(key("end"))
	if got := m.View(); !strings.HasPrefix(got, "line 16") {
		t.Fatalf("end = %q", got)
	}
	m.Update(key("up"))
	if got := m.View(); !strings.HasPrefix(got, "line 15") {
		t.Fatalf("up = %q", got)
	}
	m.Update(key("home"))
	if got := m.View(); !strings.HasPrefix(got, "line 0") {
		t.Fatalf("home = %q", got)
	}
	if m.Title() != " Help " {
		t.Fatalf("title = %q", m.Title())
	}
}

func TestMarkdownRendersAndIsCachedPerWidth(t *testing.T) {
	m, _ := newPanel(t)
	m.Show(layout.RightText)
	m.SetText("Help", "# Title\n\nSome *text*", "markdown")
	v := ansi.Strip(m.View())
	if !strings.Contains(v, "Title") || strings.Contains(v, "# Title") {
		t.Fatalf("markdown not rendered: %q", v)
	}
	k1 := m.rendered.key
	m.SetSize(40, 4)
	if m.rendered.key == k1 {
		t.Fatal("width change must re-render")
	}
	m.ThemeChanged()
	if m.rendered.key == k1 {
		t.Fatal("theme change must re-render")
	}
}

func TestDiffViewerScrollsHorizontally(t *testing.T) {
	m, _ := newPanel(t)
	m.Show(layout.RightDiff)
	m.SetDiff("Diff: a.go", "--- a\n+++ b\n@@ -1 +1 @@\n-old line that is quite long indeed\n+new line that is quite long indeed")
	first := ansi.Strip(m.View())
	m.Update(key("right"))
	m.Update(key("right"))
	shifted := ansi.Strip(m.View())
	if first == shifted || !strings.HasPrefix(shifted, "- a") || !strings.Contains(shifted, "ld line that") {
		t.Fatalf("horizontal scroll failed:\n%s\n--\n%s", first, shifted)
	}
	m.Update(key("left"))
	m.Update(key("left"))
	if ansi.Strip(m.View()) != first {
		t.Fatal("scrolling back left should restore the view")
	}
	for _, l := range strings.Split(m.View(), "\n") {
		if ansi.StringWidth(l) > 30 {
			t.Fatalf("diff line wider than pane: %q", l)
		}
	}
	if m.Title() != " Diff: a.go " {
		t.Fatalf("title = %q", m.Title())
	}
}

func TestDebugPanelSticksToBottomAndEmitsKeys(t *testing.T) {
	m, dbg := newPanel(t)
	for i := 0; i < 10; i++ {
		dbg.AddDebugMessage(fmt.Sprintf("log %d", i))
	}
	m.Show(layout.RightDebug)
	if got := m.View(); !strings.HasSuffix(got, "log 9") {
		t.Fatalf("debug should show the newest lines: %q", got)
	}
	dbg.AddDebugMessage("log 10")
	m.DebugChanged()
	if got := m.View(); !strings.HasSuffix(got, "log 10") {
		t.Fatalf("new lines should stay in view: %q", got)
	}
	m.Update(key("up"))
	dbg.AddDebugMessage("log 11")
	m.DebugChanged()
	if got := m.View(); strings.HasSuffix(got, "log 11") {
		t.Fatal("scrolled-up debug view must not jump to the bottom")
	}
	if m.Update(key("c")) != "debug.clear" || m.Update(key("y")) != "debug.copy" {
		t.Fatal("c/y must map to the debug bus topics")
	}
	m.Show(layout.RightText)
	if m.Update(key("c")) != "" {
		t.Fatal("c outside the debug panel does nothing")
	}
}

func TestLongLinesWrapInTextAndDebug(t *testing.T) {
	m, dbg := newPanel(t)
	dbg.AddDebugMessage(strings.Repeat("x", 100))
	m.Show(layout.RightDebug)
	for _, l := range strings.Split(m.View(), "\n") {
		if ansi.StringWidth(l) > 30 {
			t.Fatalf("debug line not wrapped: %q", l)
		}
	}
	if len(m.dlog.lines) != 4 {
		t.Fatalf("100 chars at width 30 should wrap to 4 lines, got %d", len(m.dlog.lines))
	}
}

func TestTodosPanelRendersTheListWithProgressInTheTitle(t *testing.T) {
	m, _ := newPanel(t)
	if m.HasTodos() {
		t.Fatal("no todos yet")
	}
	m.SetTodos([]presentation.TodoItem{
		{ID: "1", Content: "write tests", Status: "completed", Priority: "high"},
		{ID: "2", Content: "ship it", Status: "in_progress", Priority: "medium"},
		{ID: "3", Content: "celebrate", Status: "pending", Priority: "low"},
	})
	m.Show(layout.RightTodos)
	if m.Title() != " Todos 1/3 " {
		t.Fatalf("title = %q", m.Title())
	}
	v := ansi.Strip(m.View())
	if !strings.Contains(v, "write tests") || !strings.Contains(v, "ship it") || !strings.Contains(v, "celebrate") {
		t.Fatalf("todos view:\n%s", v)
	}
	m.SetTodos([]presentation.TodoItem{{ID: "1", Content: "write tests", Status: "completed"}})
	if m.Title() != " Todos 1/1 " || strings.Contains(ansi.Strip(m.View()), "ship it") {
		t.Fatal("a new list replaces the old one")
	}
}
