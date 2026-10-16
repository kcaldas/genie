// Package statusbar renders the one-line status row: request state on the
// left, debug or mode hints in the centre, counters on the right.
package statusbar

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

const (
	tickInterval    = 100 * time.Millisecond
	memSampleEvery  = 10 // ticks, so once a second
	confirmInterval = 2  // ticks per confirmation spinner frame
)

var (
	thinkingFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	confirmFrames  = []string{"◐", "◓", "◑", "◒"}
)

// TickMsg advances the spinner. It is only scheduled while something is
// in progress.
type TickMsg struct{}

// State is what the bar reads from the rest of the UI on every render.
type State interface {
	IsWaitingConfirmation() bool
	GetMessageCount() int
}

// Model is the status bar.
type Model struct {
	state State
	now   func() time.Time

	running   bool
	startedAt time.Time
	tokens    int32
	tick      int
	ticking   bool
	memMB     uint64
	debugOn   bool
	centre    string // overrides the centre text, e.g. the vim mode
	activity  string // what the agent says it is doing, while running
}

// SetActivity sets the running-tool narration shown next to the spinner.
func (m *Model) SetActivity(text string) { m.activity = text }

// New builds a status bar. now may be nil for the wall clock.
func New(state State, now func() time.Time) *Model {
	if now == nil {
		now = time.Now
	}
	m := &Model{state: state, now: now}
	m.debugOn = strings.EqualFold(os.Getenv("GENIE_DEBUG_LEVEL"), "debug")
	m.sampleMemory()
	return m
}

// SetCentre sets the centre text shown when debug is off.
func (m *Model) SetCentre(s string) { m.centre = s }

// SetDebug toggles the "Debug is ON" centre text.
func (m *Model) SetDebug(on bool) { m.debugOn = on }

// Running reports whether a request is in flight.
func (m *Model) Running() bool { return m.running }

// Start begins ticking if anything needs animating.
func (m *Model) Start() tea.Cmd { return m.ensureTicking() }

// Update handles request lifecycle and tick messages.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case uimsg.RequestStarted:
		if msg.Active == 1 {
			m.running = true
			m.startedAt = m.now()
			m.tokens = 0
		}
		return m.ensureTicking()
	case uimsg.RequestFinished:
		if msg.Last {
			m.running = false
			m.activity = ""
		}
	case uimsg.ToolActivity:
		m.activity = msg.Text
	case uimsg.TokenCount:
		m.tokens += msg.Tokens
	case TickMsg:
		m.ticking = false
		m.tick++
		if m.tick%memSampleEvery == 0 {
			m.sampleMemory()
		}
		return m.ensureTicking()
	}
	return nil
}

// ensureTicking schedules the next tick while the bar is animating.
func (m *Model) ensureTicking() tea.Cmd {
	if m.ticking || (!m.running && !m.state.IsWaitingConfirmation()) {
		return nil
	}
	m.ticking = true
	return tea.Tick(tickInterval, func(time.Time) tea.Msg { return TickMsg{} })
}

func (m *Model) sampleMemory() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	m.memMB = ms.Alloc / 1024 / 1024
}

// Left renders the request state: Ready, Thinking with a spinner and
// elapsed seconds, or the confirmation prompt.
func (m *Model) Left(th *style.Theme) string {
	switch {
	case m.state.IsWaitingConfirmation():
		return " Your call " + th.Error.Render(confirmFrames[(m.tick/confirmInterval)%len(confirmFrames)])
	case m.running:
		secs := int(m.now().Sub(m.startedAt).Seconds())
		what := "Thinking"
		if m.activity != "" {
			what = m.activity
		}
		return fmt.Sprintf(" %s %s %s %s",
			th.TextTertiary.Render(what),
			th.TextTertiary.Render(fmt.Sprintf("(%ds)", secs)),
			th.Error.Render(thinkingFrames[m.tick%len(thinkingFrames)]),
			th.TextTertiary.Render("(ESC to cancel)"))
	default:
		return " " + th.Primary.Render("Ready")
	}
}

// Centre renders the debug indicator or the configured centre text.
func (m *Model) Centre(th *style.Theme) string {
	if m.debugOn {
		return th.Secondary.Render("Debug is ON")
	}
	return th.TextTertiary.Render(m.centre)
}

// Right renders the counters.
func (m *Model) Right(th *style.Theme) string {
	return th.TextTertiary.Render(fmt.Sprintf("Tokens: %s | Msgs: %d | Mem: %dMB",
		formatTokenCount(m.tokens), m.state.GetMessageCount(), m.memMB))
}

// formatTokenCount shortens large counts: 1.2K, 3.4M.
func formatTokenCount(count int32) string {
	switch {
	case count >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(count)/1_000_000)
	case count >= 1_000:
		return fmt.Sprintf("%.1fK", float64(count)/1_000)
	default:
		return fmt.Sprint(count)
	}
}
