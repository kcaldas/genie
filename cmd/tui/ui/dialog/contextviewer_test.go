package dialog

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
)

func key(s string) tea.KeyPressMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}

func TestContextViewerNavigatesSortedKeysAndScrolls(t *testing.T) {
	var long strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&long, "project line %d\n", i)
	}
	v := NewContextViewer(map[string]string{"project": long.String(), "chat": "hello chat", "files": ""})
	v.SetSize(80, 12)
	th := style.New("default", 1)

	out := ansi.Strip(v.View(th))
	if !strings.Contains(out, "► chat") || !strings.Contains(out, "{chat}") || !strings.Contains(out, "hello chat") {
		t.Fatalf("first key should be selected:\n%s", out)
	}
	v.Update(key("down"))
	if !strings.Contains(ansi.Strip(v.View(th)), "No content available for 'files'") {
		t.Fatal("empty part should say so")
	}
	v.Update(key("j"))
	out = ansi.Strip(v.View(th))
	if !strings.Contains(out, "project line 0") || strings.Contains(out, "project line 20") {
		t.Fatalf("project should start at the top:\n%s", out)
	}
	v.Update(key("pgdown"))
	if !strings.Contains(ansi.Strip(v.View(th)), "project line 9") {
		t.Fatal("pgdown should scroll a page")
	}
	v.Update(key("end"))
	if !strings.Contains(ansi.Strip(v.View(th)), "project line 39") {
		t.Fatal("end should show the last line")
	}
	v.Update(key("up"))
	if !strings.Contains(ansi.Strip(v.View(th)), "No content available for 'files'") {
		t.Fatal("up should move the selection and reset the scroll")
	}
	for _, l := range strings.Split(ansi.Strip(v.View(th)), "\n") {
		if ansi.StringWidth(l) != 80 {
			t.Fatalf("every row must be full width, got %d: %q", ansi.StringWidth(l), l)
		}
	}
}

func TestContextViewerCloseRefreshAndEmpty(t *testing.T) {
	v := NewContextViewer(nil)
	v.SetSize(60, 10)
	th := style.New("default", 1)
	out := ansi.Strip(v.View(th))
	if !strings.Contains(out, "No context data available") || !strings.Contains(out, "No Content") {
		t.Fatalf("empty state:\n%s", out)
	}
	cmd, done := v.Update(key("r"))
	if done || cmd == nil {
		t.Fatal("r should request a refresh without closing")
	}
	if _, ok := cmd().(ContextRefreshMsg); !ok {
		t.Fatal("refresh command must yield ContextRefreshMsg")
	}
	v.SetData(map[string]string{"a": "x", "b": "y"})
	v.Update(key("down"))
	v.SetData(map[string]string{"a": "x", "b": "y", "c": "z"})
	if v.selectedKey() != "b" {
		t.Fatalf("selection should survive a refresh, got %q", v.selectedKey())
	}
	if _, done := v.Update(key("q")); !done {
		t.Fatal("q closes")
	}
	if _, done := v.Update(key("esc")); !done {
		t.Fatal("esc closes")
	}
	if v.Cursor() != nil || !v.Fullscreen() {
		t.Fatal("viewer is fullscreen with no cursor")
	}
}

type fakeDialog struct{ closeOn string }

func (f *fakeDialog) SetSize(int, int) {}
func (f *fakeDialog) Update(msg tea.Msg) (tea.Cmd, bool) {
	k, ok := msg.(tea.KeyPressMsg)
	return nil, ok && k.String() == f.closeOn
}
func (f *fakeDialog) View(*style.Theme) string { return "" }
func (f *fakeDialog) Fullscreen() bool         { return false }
func (f *fakeDialog) ShowsStatus() bool        { return false }
func (f *fakeDialog) Cursor() *tea.Cursor      { return nil }

func TestStackRoutesToTopAndPopsWhenDone(t *testing.T) {
	var s Stack
	s.SetSize(10, 10)
	a, b := &fakeDialog{closeOn: "a"}, &fakeDialog{closeOn: "b"}
	s.Push(a)
	s.Push(b)
	if s.Top() != b || s.Len() != 2 {
		t.Fatal("newest dialog must be on top")
	}
	s.Update(key("a"))
	if s.Top() != b {
		t.Fatal("keys go to the top dialog only")
	}
	s.Update(key("b"))
	if s.Top() != a {
		t.Fatal("done dialog must pop")
	}
	s.Update(key("a"))
	if s.Top() != nil || s.Update(key("x")) != nil {
		t.Fatal("empty stack ignores input")
	}
}
