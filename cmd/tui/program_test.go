package tui

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
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

func waitForOutput(t *testing.T, tm *teatest.TestModel, want string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(want))
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
