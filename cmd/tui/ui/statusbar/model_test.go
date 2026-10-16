package statusbar

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

type fakeState struct {
	waiting bool
	count   int
}

func (f *fakeState) IsWaitingConfirmation() bool { return f.waiting }
func (f *fakeState) GetMessageCount() int        { return f.count }

func newBar(t *testing.T) (*Model, *fakeState, *time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	st := &fakeState{count: 3}
	m := New(st, func() time.Time { return now })
	m.SetDebug(false)
	return m, st, &now
}

func TestLeftShowsReadyThenThinkingWithElapsed(t *testing.T) {
	m, _, now := newBar(t)
	th := style.New("default", 1)
	if got := ansi.Strip(m.Left(th)); got != " Ready" {
		t.Fatalf("idle left = %q", got)
	}
	if cmd := m.Update(uimsg.RequestStarted{Active: 1}); cmd == nil {
		t.Fatal("starting a request must arm the tick")
	}
	*now = now.Add(7 * time.Second)
	got := ansi.Strip(m.Left(th))
	if !strings.HasPrefix(got, " Thinking (7s) ") || !strings.HasSuffix(got, "(ESC to cancel)") {
		t.Fatalf("running left = %q", got)
	}
	m.Update(uimsg.RequestFinished{Last: false})
	if !m.Running() {
		t.Fatal("non-last finish must keep running")
	}
	m.Update(uimsg.RequestFinished{Last: true})
	if got := ansi.Strip(m.Left(th)); got != " Ready" {
		t.Fatalf("after finish = %q", got)
	}
}

func TestTicksOnlyWhileBusy(t *testing.T) {
	m, st, _ := newBar(t)
	if m.Start() != nil {
		t.Fatal("idle bar must not tick")
	}
	m.Update(uimsg.RequestStarted{Active: 1})
	if m.Update(uimsg.RequestStarted{Active: 2}) != nil {
		t.Fatal("a second request must not arm a second tick")
	}
	if m.Update(TickMsg{}) == nil {
		t.Fatal("tick while running must re-arm")
	}
	m.Update(uimsg.RequestFinished{Last: true})
	if m.Update(TickMsg{}) != nil {
		t.Fatal("tick after finishing must stop")
	}
	st.waiting = true
	if m.Start() == nil {
		t.Fatal("waiting for confirmation must animate")
	}
}

func TestConfirmationSpinner(t *testing.T) {
	m, st, _ := newBar(t)
	st.waiting = true
	th := style.New("default", 1)
	a := ansi.Strip(m.Left(th))
	if !strings.HasPrefix(a, " Your call ") {
		t.Fatalf("waiting left = %q", a)
	}
	m.Update(TickMsg{})
	m.Update(TickMsg{})
	if b := ansi.Strip(m.Left(th)); b == a {
		t.Fatal("confirmation spinner did not advance after two ticks")
	}
}

func TestRightCountersAndTokens(t *testing.T) {
	m, _, _ := newBar(t)
	th := style.New("default", 1)
	m.Update(uimsg.RequestStarted{Active: 1})
	m.Update(uimsg.TokenCount{Tokens: 700})
	m.Update(uimsg.TokenCount{Tokens: 800})
	got := ansi.Strip(m.Right(th))
	if !strings.HasPrefix(got, "Tokens: 1.5K | Msgs: 3 | Mem: ") || !strings.HasSuffix(got, "MB") {
		t.Fatalf("right = %q", got)
	}
	m.Update(uimsg.RequestStarted{Active: 1})
	if !strings.Contains(ansi.Strip(m.Right(th)), "Tokens: 0 ") {
		t.Fatal("a new first request resets the token count")
	}
}

func TestCentreDebugOverridesText(t *testing.T) {
	m, _, _ := newBar(t)
	th := style.New("default", 1)
	m.SetCentre("INSERT")
	if got := ansi.Strip(m.Centre(th)); got != "INSERT" {
		t.Fatalf("centre = %q", got)
	}
	m.SetDebug(true)
	if got := ansi.Strip(m.Centre(th)); got != "Debug is ON" {
		t.Fatalf("debug centre = %q", got)
	}
}

func TestFormatTokenCount(t *testing.T) {
	for in, want := range map[int32]string{0: "0", 999: "999", 1000: "1.0K", 15600: "15.6K", 2_500_000: "2.5M"} {
		if got := formatTokenCount(in); got != want {
			t.Errorf("formatTokenCount(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestActivityReplacesThinkingWhileRunning(t *testing.T) {
	m, _, _ := newBar(t)
	th := style.New("default", 1)
	m.Update(uimsg.RequestStarted{Active: 1})
	m.Update(uimsg.ToolActivity{Text: "Listing files"})
	if got := ansi.Strip(m.Left(th)); !strings.HasPrefix(got, " Listing files (0s)") {
		t.Fatalf("activity left = %q", got)
	}
	m.Update(uimsg.RequestFinished{Last: true})
	m.Update(uimsg.RequestStarted{Active: 1})
	if got := ansi.Strip(m.Left(th)); !strings.HasPrefix(got, " Thinking") {
		t.Fatalf("activity must clear between requests: %q", got)
	}
}

func TestQueuedCountShowsWhileRunning(t *testing.T) {
	m, _, _ := newBar(t)
	th := style.New("default", 1)
	m.Update(uimsg.RequestStarted{Active: 1})
	m.Update(uimsg.QueueChanged{Count: 2})
	if got := ansi.Strip(m.Left(th)); !strings.HasSuffix(got, "· 2 queued") {
		t.Fatalf("left = %q", got)
	}
	m.Update(uimsg.QueueChanged{Count: 0})
	if got := ansi.Strip(m.Left(th)); strings.Contains(got, "queued") {
		t.Fatalf("left = %q", got)
	}
}
