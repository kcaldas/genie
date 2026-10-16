package chatview

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

type cfg struct{ c types.Config }

func (f *cfg) GetConfig() *types.Config { return &f.c }
func (f *cfg) GetTheme() *types.Theme   { return presentation.GetThemeForMode(f.c.Theme, "true") }

func newCardModel(t *testing.T) (*Model, *state.ChatState) {
	t.Helper()
	chat := state.NewChatState(100)
	m := New(chat, &countingFormatter{})
	m.SetStyling(Styling{Theme: style.New("default", 1), Cfg: &cfg{c: types.Config{Theme: "default", DiffTheme: "auto"}}})
	m.SetSize(60, 12)
	return m, chat
}

func toolMsg(name, args, summary, output string, ok bool) types.Message {
	return types.Message{Role: "assistant", ContentType: "tool", Content: name + " " + args,
		Tool: &types.ToolCall{Name: name, Args: args, Success: ok, Summary: summary, Output: output}}
}

func view(m *Model) string { return ansi.Strip(m.View()) }

func TestToolCardCollapsesToOneLineAndExpands(t *testing.T) {
	m, chat := newCardModel(t)
	id := chat.AddMessage(toolMsg("bash", "ls -la", "3 lines", "a.go\nb.go\nc.go", true))
	m.Update(uimsg.ChatChanged{})
	flush(m)
	v := view(m)
	if !strings.Contains(v, "● bash ls -la ▸") || !strings.Contains(v, "✓ 3 lines") || strings.Contains(v, "a.go") {
		t.Fatalf("collapsed card:\n%s", v)
	}
	if m.LineCount() != 2 { // header + separator
		t.Fatalf("collapsed card lines = %d", m.LineCount())
	}
	m.ToggleDetails()
	v = view(m)
	if !strings.Contains(v, "▾") || !strings.Contains(v, "  a.go") || !strings.Contains(v, "  c.go") {
		t.Fatalf("expanded card:\n%s", v)
	}
	m.ToggleDetails()
	if strings.Contains(view(m), "a.go") {
		t.Fatal("toggling details again collapses")
	}
	if !m.ToggleAt(0) || !strings.Contains(view(m), "a.go") {
		t.Fatal("clicking the header expands that card")
	}
	if !m.ToggleAt(2) || strings.Contains(view(m), "a.go") {
		t.Fatal("clicking inside the expanded card collapses it")
	}
	_ = id
}

func TestFailedToolCardShowsTheError(t *testing.T) {
	m, chat := newCardModel(t)
	chat.AddMessage(toolMsg("readFile", "nope.go", "no such file", "open nope.go: no such file", false))
	m.Update(uimsg.ChatChanged{})
	flush(m)
	if v := view(m); !strings.Contains(v, "✗ no such file") {
		t.Fatalf("failed card:\n%s", v)
	}
}

func TestToolCardShowsDiffAndCapsLongOutput(t *testing.T) {
	m, chat := newCardModel(t)
	msg := toolMsg("writeFile", "x.go", "Wrote x.go", "", true)
	msg.Tool.Diff = "--- x.go\n+++ x.go\n@@ -1 +1 @@\n-old\n+new line"
	chat.AddMessage(msg)
	var long strings.Builder
	for i := 0; i < maxCardLines+50; i++ {
		long.WriteString("out\n")
	}
	chat.AddMessage(toolMsg("bash", "yes", "250 lines", strings.TrimSpace(long.String()), true))
	m.ToggleDetails()
	m.SetSize(60, 400) // tall enough to see both cards
	m.Update(uimsg.ChatChanged{})
	flush(m)
	v := view(m)
	if !strings.Contains(v, "+new line") {
		t.Fatalf("diff not shown:\n%s", v[:min(len(v), 600)])
	}
	if !strings.Contains(v, "… 50 more lines") {
		t.Fatal("long output must be capped with a count")
	}
}

func TestThinkingCollapsesToOneLine(t *testing.T) {
	m, chat := newCardModel(t)
	chat.AddMessage(types.Message{Role: "assistant", ContentType: "thought", Content: "first idea\nsecond idea\nthird"})
	chat.AddMessage(types.Message{Role: "user", Content: "hi"})
	m.Update(uimsg.ChatChanged{})
	flush(m)
	v := view(m)
	if !strings.Contains(v, "✱ Thinking (3 lines) ▸ first idea") || strings.Contains(v, "second idea") {
		t.Fatalf("collapsed thought:\n%s", v)
	}
	m.ToggleDetails()
	if v := view(m); !strings.Contains(v, "second idea") || !strings.Contains(v, "▾") {
		t.Fatalf("expanded thought:\n%s", v)
	}
	if m.ToggleAt(m.LineCount() - 1) {
		t.Fatal("clicking a plain message is not a toggle")
	}
}

func TestCardsNeverExceedTheWidth(t *testing.T) {
	m, chat := newCardModel(t)
	m.SetSize(30, 10)
	chat.AddMessage(toolMsg("bash", strings.Repeat("x", 100), strings.Repeat("y", 50), strings.Repeat("z", 100), true))
	m.ToggleDetails()
	m.Update(uimsg.ChatChanged{})
	flush(m)
	for _, l := range strings.Split(m.View(), "\n") {
		if ansi.StringWidth(l) > 30 {
			t.Fatalf("line wider than view: %q", l)
		}
	}
}
