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
	core_events "github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie/genietest"
)

// newTestProgram wires the Bubble Tea UI over a mocked genie core and a
// headless terminal of the given size.
func newTestProgram(t *testing.T, w, h int) (*Program, *genietest.TestFixture, *teatest.TestModel) {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // never read or write the developer's config
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

func TestProgramExpandedPromptRoundTrip(t *testing.T) {
	prog, fixture, tm := newTestProgram(t, 100, 30)
	fixture.ExpectMessage("line one\nline two").RespondWith("got both lines")
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	tm.Type("line one")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyF4})
	waitForOutput(t, tm, "F4 collapse")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	tm.Type("line two")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "got both lines")

	screen := finalScreen(t, tm)
	if strings.Contains(screen, "F4 collapse") || !strings.Contains(screen, "F4 expand") {
		t.Fatalf("prompt should have collapsed after sending:\n%s", screen)
	}
}

func TestProgramWriteCommandExpandsAndEscapeCollapses(t *testing.T) {
	prog, _, tm := newTestProgram(t, 100, 30)
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	tm.Type(":write")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "F4 collapse")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	screen := finalScreen(t, tm)
	if strings.Contains(screen, "F4 collapse") {
		t.Fatalf("esc should collapse the prompt:\n%s", screen)
	}
}

func TestProgramToolConfirmationDialog(t *testing.T) {
	prog, fixture, tm := newTestProgram(t, 100, 30)
	responses := make(chan core_events.ToolConfirmationResponse, 1)
	fixture.EventBus.Subscribe("tool.confirmation.response", func(e interface{}) {
		if r, ok := e.(core_events.ToolConfirmationResponse); ok {
			responses <- r
		}
	})
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	fixture.EventBus.Publish("tool.confirmation.request", core_events.ToolConfirmationRequest{
		ExecutionID: "exec-1", ToolName: "bash", Message: "Run `make test`?",
	})
	waitForOutput(t, tm, "Tool: bash", "1 Yes · 2 No", "Your call")

	tm.Send(tea.KeyPressMsg{Code: '1', Text: "1"})
	select {
	case r := <-responses:
		if r.ExecutionID != "exec-1" || !r.Confirmed {
			t.Fatalf("response = %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no tool.confirmation.response published")
	}
	screen := finalScreen(t, tm)
	if strings.Contains(screen, "Tool: bash") || strings.Contains(screen, "Your call") {
		t.Fatalf("dialog should be gone and status back to Ready:\n%s", screen)
	}
}

func TestProgramUserConfirmationWithDiff(t *testing.T) {
	prog, fixture, tm := newTestProgram(t, 120, 40)
	responses := make(chan core_events.UserConfirmationResponse, 1)
	fixture.EventBus.Subscribe("user.confirmation.response", func(e interface{}) {
		if r, ok := e.(core_events.UserConfirmationResponse); ok {
			responses <- r
		}
	})
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	fixture.EventBus.Publish("user.confirmation.request", core_events.UserConfirmationRequest{
		ExecutionID: "u-1", Title: "Apply changes", FilePath: "main.go", ContentType: "diff",
		Content: "--- main.go\n+++ main.go\n@@ -1 +1 @@\n-old\n+new shiny line",
	})
	waitForOutput(t, tm, "Diff: main.go", "1 Confirm · 2 Cancel", "new shiny line")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	select {
	case r := <-responses:
		if r.ExecutionID != "u-1" || r.Confirmed {
			t.Fatalf("esc must answer no: %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no user.confirmation.response published")
	}
	tm.Send(uimsg.Quit{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestProgramMultiLinePromptRoundTrip(t *testing.T) {
	prog, fixture, tm := newTestProgram(t, 100, 30)
	fixture.ExpectMessage("first line\nsecond line").RespondWith("got two lines")
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	tm.Type("first line")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	tm.Type("second line")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	// The fixture only answers this exact two-line message, so the reply
	// proves Shift+Enter inserted the newline.
	waitForOutput(t, tm, "got two lines")

	tm.Send(uimsg.Quit{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestProgramPalettes(t *testing.T) {
	prog, _, tm := newTestProgram(t, 100, 30)
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	tm.Type("/")
	waitForOutput(t, tm, " Commands ", ":help")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}) // clear the '/'

	tm.Type("@")
	waitForOutput(t, tm, "Files ─")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	screen := finalScreen(t, tm)
	if strings.Contains(screen, "Files ─") || !strings.Contains(screen, "│@") {
		t.Fatalf("picker should be closed with the typed '@' kept:\n%s", screen)
	}
}

func TestProgramHistorySearch(t *testing.T) {
	prog, fixture, tm := newTestProgram(t, 100, 30)
	fixture.ExpectMessage("remember this").RespondWith("ok")
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	tm.Type("remember this")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	waitForOutput(t, tm, "ok")

	tm.Send(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	waitForOutput(t, tm, "History ─")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	screen := finalScreen(t, tm)
	if strings.Contains(screen, "History ─") || !strings.Contains(screen, "remember this") {
		t.Fatalf("the chosen history entry should be back in the prompt:\n%s", screen)
	}
}

func TestProgramToolCard(t *testing.T) {
	prog, fixture, tm := newTestProgram(t, 100, 30)
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	fixture.EventBus.Publish("tool.executed", core_events.ToolExecutedEvent{
		ExecutionID: "t1", ToolName: "bash", Success: true,
		Parameters: map[string]any{"command": "go test ./..."},
		Result:     map[string]any{"success": true, "results": "ok  pkg/a\nok  pkg/b\nok  pkg/c"},
	})
	waitForOutput(t, tm, "bash go test ./...", "3 lines")

	tm.Send(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	waitForOutput(t, tm, "ok  pkg/b")

	tm.Send(uimsg.Quit{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestProgramTodoPanelOpensOnTodoWrite(t *testing.T) {
	prog, fixture, tm := newTestProgram(t, 120, 40)
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	fixture.EventBus.Publish("tool.executed", core_events.ToolExecutedEvent{
		ExecutionID: "todo-1", ToolName: "TodoWrite", Success: true,
		Parameters: map[string]any{},
		Result: map[string]any{"success": true, "message": "Updated 2 todos", "todos": []any{
			map[string]any{"id": "1", "content": "write the parser", "status": "in_progress", "priority": "high"},
			map[string]any{"id": "2", "content": "add tests", "status": "pending", "priority": "medium"},
		}},
	})
	waitForOutput(t, tm, "Todos 0/2", "write the parser")

	tm.Send(uimsg.Quit{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestProgramAlwaysAllow(t *testing.T) {
	prog, fixture, tm := newTestProgram(t, 100, 30)
	responses := make(chan core_events.ToolConfirmationResponse, 2)
	fixture.EventBus.Subscribe("tool.confirmation.response", func(e interface{}) {
		if r, ok := e.(core_events.ToolConfirmationResponse); ok {
			responses <- r
		}
	})
	prog.Start("")
	waitForOutput(t, tm, "Type :? for help")

	fixture.EventBus.Publish("tool.confirmation.request", core_events.ToolConfirmationRequest{ExecutionID: "c1", ToolName: "bash", Message: "ls"})
	waitForOutput(t, tm, "Tool: bash", "a Session")
	tm.Send(tea.KeyPressMsg{Code: 'a', Text: "a"})
	select {
	case r := <-responses:
		if !r.Confirmed {
			t.Fatalf("a should accept: %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no response")
	}
	fixture.EventBus.Publish("tool.confirmation.request", core_events.ToolConfirmationRequest{ExecutionID: "c2", ToolName: "bash", Message: "ls"})
	select {
	case r := <-responses:
		if !r.Confirmed || r.ExecutionID != "c2" {
			t.Fatalf("second bash request should auto-accept: %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second request was not auto-accepted")
	}
	tm.Send(uimsg.Quit{})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
