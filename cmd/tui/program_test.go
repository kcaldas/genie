package tui

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/kcaldas/genie/cmd/tui/ui"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
	"github.com/kcaldas/genie/pkg/genie/genietest"
)

// newTestProgram wires the Bubble Tea UI over a mocked genie core and a
// headless terminal of the given size.
func newTestProgram(t *testing.T, w, h int) (*Program, *genietest.TestFixture, *teatest.TestModel) {
	t.Helper()
	fixture := genietest.NewTestFixture(t)
	session := fixture.StartAndGetSession()
	prog, err := NewProgram(session,
		WithGenie(fixture.Genie),
		WithHistoryPath(filepath.Join(t.TempDir(), "history")),
		WithClock(func() time.Time { return time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC) }),
	)
	if err != nil {
		t.Fatalf("NewProgram: %v", err)
	}
	tm := teatest.NewTestModel(t, prog.Model(), teatest.WithInitialTermSize(w, h))
	return prog, fixture, tm
}

// waitForOutput blocks until every needle has appeared in the output
// written since the previous wait. teatest hands each WaitFor only the
// bytes produced after the last one, so strings expected from the same
// redraw must be waited for together.
func waitForOutput(t *testing.T, tm *teatest.TestModel, needles ...string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		for _, n := range needles {
			if !bytes.Contains(b, []byte(n)) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

func TestProgramChatRoundTrip(t *testing.T) {
	prog, fixture, tm := newTestProgram(t, 100, 30)
	fixture.ExpectMessage("hello there").RespondWith("Greetings from the mock")

	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	tm.Type("hello there")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "Greetings from the mock")

	tm.Send(uimsg.Quit{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestProgramInitialMessageIsSent(t *testing.T) {
	prog, fixture, tm := newTestProgram(t, 100, 30)
	fixture.ExpectMessage("from stdin").RespondWith("stdin reply")

	prog.Start("from stdin")
	waitForOutput(t, tm, "stdin reply")

	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestProgramColonCommandRuns(t *testing.T) {
	prog, _, tm := newTestProgram(t, 100, 30)
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	tm.Type(":theme dracula")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "dracula")

	tm.Send(uimsg.Quit{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// finalScreen quits the program and returns the last frame as plain text.
// The dispatcher preserves order, so anything the test saw answered has
// every earlier controller message applied before this frame.
func finalScreen(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	tm.Send(uimsg.Quit{})
	m := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).(*ui.Model)
	return ansi.Strip(m.View().Content)
}

func TestProgramHelpTogglesRightPanel(t *testing.T) {
	prog, fixture, tm := newTestProgram(t, 120, 40)
	fixture.ExpectMessage("ping").RespondWith("pong")
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyF1})
	waitForOutput(t, tm, "GENIE(1)", " Help ")

	tm.Type(":help")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	tm.Type("ping")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "pong")

	if screen := finalScreen(t, tm); strings.Contains(screen, " Help ") || strings.Contains(screen, "GENIE(1)") {
		t.Fatalf("help panel still shown after toggling off:\n%s", screen)
	}
}

func TestProgramDebugPanelAndContextViewer(t *testing.T) {
	prog, _, tm := newTestProgram(t, 120, 40)
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyF12})
	waitForOutput(t, tm, " Debug ")

	tm.Type(":context")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "Context Parts", "Esc/q Close")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	screen := finalScreen(t, tm)
	if strings.Contains(screen, "Context Parts") || !strings.Contains(screen, " Debug ") {
		t.Fatalf("expected the context viewer closed and the debug panel kept:\n%s", screen)
	}
}

func TestProgramClearRemovesMessages(t *testing.T) {
	prog, fixture, tm := newTestProgram(t, 100, 30)
	fixture.ExpectMessage("remember me").RespondWith("noted")
	fixture.ExpectMessage("ping").RespondWith("pong")
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	tm.Type("remember me")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "noted")

	tm.Type(":clear")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	tm.Type("ping")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "pong")

	screen := finalScreen(t, tm)
	if strings.Contains(screen, "remember me") || strings.Contains(screen, "noted") || !strings.Contains(screen, "pong") {
		t.Fatalf("clear did not remove the earlier messages:\n%s", screen)
	}
}

func TestProgramWriteModeRoundTrip(t *testing.T) {
	prog, fixture, tm := newTestProgram(t, 100, 30)
	fixture.ExpectMessage("line one\nline two").RespondWith("got both lines")
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	tm.Type("line one")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyF4})
	waitForOutput(t, tm, "Write Mode - Compose Message", "line one")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	tm.Type("line two")
	tm.Send(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	waitForOutput(t, tm, "got both lines")

	screen := finalScreen(t, tm)
	if strings.Contains(screen, "Write Mode") || !strings.Contains(screen, "F4 / Ctrl+V Expand") {
		t.Fatalf("write mode should have closed back to the prompt:\n%s", screen)
	}
}

func TestProgramWriteCommandAndEscape(t *testing.T) {
	prog, _, tm := newTestProgram(t, 100, 30)
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	tm.Type(":write")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "Write Mode - Compose Message")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	screen := finalScreen(t, tm)
	if strings.Contains(screen, "Write Mode") {
		t.Fatalf("esc should close write mode:\n%s", screen)
	}
}
