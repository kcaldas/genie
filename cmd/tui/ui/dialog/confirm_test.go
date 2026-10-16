package dialog

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

type cfg struct{ c types.Config }

func (f *cfg) GetConfig() *types.Config { return &f.c }
func (f *cfg) GetTheme() *types.Theme   { return presentation.GetThemeForMode(f.c.Theme, "true") }

func testCfg() *cfg {
	return &cfg{c: types.Config{Theme: "default", GlamourTheme: "auto", DiffTheme: "auto", MarkdownRendering: "enabled"}}
}

func TestConfirmRendersCentredWithPromptAndAnswers(t *testing.T) {
	req := uimsg.ConfirmationRequested{Kind: uimsg.ConfirmTool, ExecutionID: "e1", Title: "Tool: bash", Prompt: "1 - Yes | 2 - No", Content: "run `ls -la`?", ContentType: "text"}
	c := NewConfirm(req, testCfg())
	c.SetSize(100, 30)
	r := c.Rect()
	if r.W != 80 || r.H != 24 || r.X != 10 || r.Y != 2 {
		t.Fatalf("rect = %+v", r)
	}
	out := ansi.Strip(c.View(style.New("default", 1)))
	if !strings.Contains(out, " Tool: bash ") || !strings.Contains(out, "1 - Yes | 2 - No") || !strings.Contains(out, "run `ls -la`?") {
		t.Fatalf("dialog:\n%s", out)
	}
	if lines := strings.Split(out, "\n"); len(lines) != 24 || ansi.StringWidth(lines[0]) != 80 {
		t.Fatalf("dialog size %dx%d", ansi.StringWidth(lines[0]), len(lines))
	}
	cmd, done := c.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if !done || cmd == nil {
		t.Fatal("y must answer and close")
	}
	a := cmd().(ConfirmAnswerMsg).Answer
	if !a.Confirmed || a.ExecutionID != "e1" || a.Kind != uimsg.ConfirmTool {
		t.Fatalf("answer = %+v", a)
	}
	for _, k := range []tea.KeyPressMsg{{Code: '2', Text: "2"}, {Code: 'n', Text: "n"}, {Code: 'N', Text: "N"}, {Code: tea.KeyEscape}} {
		cmd, done := c.Update(k)
		if !done || cmd().(ConfirmAnswerMsg).Answer.Confirmed {
			t.Fatalf("%v should answer no", k)
		}
	}
	if _, done := c.Update(tea.KeyPressMsg{Code: 'x', Text: "x"}); done {
		t.Fatal("other keys keep the dialog open")
	}
}

func TestConfirmShowsDiffAndScrolls(t *testing.T) {
	var diff strings.Builder
	diff.WriteString("--- a.go\n+++ b.go\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&diff, "+added line %d\n", i)
	}
	req := uimsg.ConfirmationRequested{Kind: uimsg.ConfirmUser, ExecutionID: "d", Title: "Diff: a.go", Prompt: "1 - Confirm | 2 - Cancel", Content: diff.String(), ContentType: "diff"}
	c := NewConfirm(req, testCfg())
	c.SetSize(80, 20)
	th := style.New("default", 1)
	out := ansi.Strip(c.View(th))
	if !strings.Contains(out, "+added line 0") || strings.Contains(out, "+added line 30") || !strings.Contains(out, "scroll") {
		t.Fatalf("diff view:\n%s", out)
	}
	c.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	if !strings.Contains(ansi.Strip(c.View(th)), "+added line 39") {
		t.Fatal("end should reach the last line")
	}
	c.Update(tea.KeyPressMsg{Code: tea.KeyHome})
	c.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if strings.Contains(ansi.Strip(c.View(th)), "+added line 0\n") {
		t.Fatal("pgdown should scroll past the first line")
	}
	c.Update(tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
	if !c.Fullscreen() || c.Rect() != (struct{ X, Y, W, H int }{0, 0, 80, 19}) {
		t.Fatalf("zoomed rect = %+v", c.Rect())
	}
	if lines := strings.Split(ansi.Strip(c.View(th)), "\n"); len(lines) != 19 {
		t.Fatalf("zoomed height = %d", len(lines))
	}
}

func TestConfirmMarkdownAndTinyScreens(t *testing.T) {
	req := uimsg.ConfirmationRequested{Kind: uimsg.ConfirmUser, ExecutionID: "m", Title: "Plan", Prompt: "1 - Go | 2 - Stop", Content: "# Plan\n\n- step one", ContentType: "markdown"}
	c := NewConfirm(req, testCfg())
	c.SetSize(60, 16)
	out := ansi.Strip(c.View(style.New("default", 1)))
	if !strings.Contains(out, "step one") || strings.Contains(out, "# Plan") {
		t.Fatalf("markdown body:\n%s", out)
	}
	c.SetSize(3, 2)
	if v := c.View(style.New("default", 1)); strings.Contains(v, "Plan") {
		t.Fatal("a tiny screen must not panic or overflow")
	}
	c.SetSize(0, 0)
	if c.View(style.New("default", 1)) != "" {
		t.Fatal("zero screen renders nothing")
	}
}
