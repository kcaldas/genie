// Package chatview renders the chat transcript. It keeps one rendered copy
// of every message per width, so a frame is a slice of cached lines and
// only the message currently streaming is ever re-rendered.
package chatview

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

// flushInterval bounds how often streaming chunks are re-rendered.
const flushInterval = 33 * time.Millisecond

// flushMsg is the coalesced "render now" tick.
type flushMsg struct{}

// Formatter renders one message into ANSI text at a width. A formatter
// that also implements PartFormatter lets a streaming message render its
// settled prefix once and only its tail per chunk.
type Formatter interface {
	FormatMessageWithWidth(msg types.Message, width int) string
}

// PartFormatter renders a message's header and body separately.
type PartFormatter interface {
	Formatter
	FormatHeader(msg types.Message) string
	FormatBody(msg types.Message, width int) string
	IsMarkdown(msg types.Message) bool
}

// Model is the chat transcript view.
type Model struct {
	chat *state.ChatState
	fmtr Formatter
	gen  uint64 // theme/config generation; part of every cache key

	width, height int

	cache *renderCache
	items []item
	total int

	offset int  // first visible line
	follow bool // keep the newest line in view as content grows

	liveID  int64 // message currently streaming, 0 if none
	dirty   bool
	pending bool // a flush tick is scheduled

	// stable caches the rendered prefix of the live message.
	stable stablePrefix

	// Cards: tool calls and thinking blocks collapse to one line unless
	// details are shown or the card was toggled.
	styling     Styling
	showDetails bool
	expanded    map[int64]bool

	sel selection // mouse text selection
}

// stablePrefix is the rendered, settled part of the streaming message,
// already split and wrapped into lines.
type stablePrefix struct {
	id    int64
	width int
	gen   uint64
	n     int // byte length of the prefix rendered
	lines []string
}

type item struct {
	id    int64
	lines []string
}

// New builds a view over chat state. The formatter may be swapped later
// with SetFormatter when the theme changes.
func New(chat *state.ChatState, fmtr Formatter) *Model {
	return &Model{chat: chat, fmtr: fmtr, cache: newRenderCache(), follow: true, expanded: map[int64]bool{}}
}

// SetSize sets the inner (content) size in cells and re-syncs.
func (m *Model) SetSize(w, h int) {
	if w == m.width && h == m.height {
		return
	}
	m.width, m.height = w, h
	m.Sync()
}

// SetFormatter swaps the formatter, for example after a theme change, and
// invalidates every cached rendering.
func (m *Model) SetFormatter(f Formatter) {
	m.fmtr = f
	m.gen++
	m.Sync()
}

// Update handles chat and tick messages. Other messages are ignored.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case uimsg.ChatChanged:
		m.liveID = 0
		return m.scheduleFlush()
	case uimsg.ChatStreaming:
		m.liveID = msg.ID
		return m.scheduleFlush()
	case flushMsg:
		m.pending = false
		if m.dirty {
			m.Sync()
		}
	}
	return nil
}

// scheduleFlush marks the view dirty and arms one flush tick if none is
// pending, so a burst of chunks costs a single render.
func (m *Model) scheduleFlush() tea.Cmd {
	m.dirty = true
	if m.pending {
		return nil
	}
	m.pending = true
	return tea.Tick(flushInterval, func(time.Time) tea.Msg { return flushMsg{} })
}

// Sync rebuilds the line list from state, rendering only messages whose
// cached rendering is missing or stale.
func (m *Model) Sync() {
	m.dirty = false
	if m.width <= 0 {
		return
	}
	msgs := m.chat.GetMessages()
	items := make([]item, 0, len(msgs))
	total := 0
	for _, msg := range msgs {
		key := cacheKey{
			id: msg.ID, width: m.width, gen: m.gen,
			contentLen: len(msg.Content), role: msg.Role, ctype: msg.ContentType,
			live: msg.ID == m.liveID,
		}
		if msg.Tool != nil || msg.ContentType == "thought" {
			key.expanded = m.isExpanded(msg.ID)
		}
		lines, ok := m.cache.get(key)
		if !ok {
			switch {
			case msg.Tool != nil || msg.ContentType == "thought":
				lines = m.renderCard(msg)
			case key.live:
				lines = m.renderLive(msg)
			default:
				lines = m.render(msg)
			}
			m.cache.put(key, lines)
		}
		items = append(items, item{id: msg.ID, lines: lines})
		total += len(lines)
	}
	m.cache.pruneExcept(msgs)
	m.items, m.total = items, total
	m.clampOffset()
}

// render formats one message and splits it into width-fitted lines. The
// formatter ends every message with a blank line, which is kept as the
// separator.
func (m *Model) render(msg types.Message) []string {
	return m.toLines(strings.TrimRight(m.fmtr.FormatMessageWithWidth(msg, m.width), "\n"))
}

// renderLive formats the streaming message. Its markdown is split at the
// last settled paragraph: the prefix is rendered once and reused, so each
// chunk costs only the tail.
func (m *Model) renderLive(msg types.Message) []string {
	pf, ok := m.fmtr.(PartFormatter)
	if !ok || !pf.IsMarkdown(msg) {
		return m.render(msg)
	}
	stable, tail := splitStable(msg.Content)
	if stable == "" {
		return m.render(msg)
	}
	want := stablePrefix{id: msg.ID, width: m.width, gen: m.gen, n: len(stable)}
	if m.stable.id != want.id || m.stable.width != want.width || m.stable.gen != want.gen || m.stable.n != want.n {
		head := msg
		head.Content = stable
		want.lines = m.wrapLines(pf.FormatHeader(msg) + pf.FormatBody(head, m.width))
		m.stable = want
	}
	rest := msg
	rest.Content = tail
	lines := make([]string, 0, len(m.stable.lines)+8)
	lines = append(lines, m.stable.lines...)
	if body := pf.FormatBody(rest, m.width); body != "" {
		lines = append(lines, "")
		lines = append(lines, m.wrapLines(body)...)
	}
	return append(lines, "")
}

// toLines wraps rendered text and appends the blank separator line.
func (m *Model) toLines(text string) []string {
	return append(m.wrapLines(text), "")
}

// wrapLines splits rendered text into lines no wider than the view.
func (m *Model) wrapLines(text string) []string {
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if ansi.StringWidth(l) <= m.width {
			lines = append(lines, l)
			continue
		}
		lines = append(lines, strings.Split(ansi.Hardwrap(l, m.width, true), "\n")...)
	}
	return lines
}

// View returns the visible slice of lines, one per row, without padding.
func (m *Model) View() string {
	if m.height <= 0 || m.total == 0 {
		return ""
	}
	out := make([]string, 0, m.height)
	skip := m.offset
	abs := m.offset
	for _, it := range m.items {
		if skip >= len(it.lines) {
			skip -= len(it.lines)
			continue
		}
		for _, l := range it.lines[skip:] {
			if len(out) == m.height {
				return strings.Join(out, "\n")
			}
			out = append(out, m.highlight(abs, l))
			abs++
		}
		skip = 0
	}
	return strings.Join(out, "\n")
}

// ScrollBy moves the view by n lines (negative is up). Scrolling away from
// the bottom stops following new content; reaching it resumes.
func (m *Model) ScrollBy(n int) {
	m.follow = false
	m.offset += n
	m.clampOffset()
	m.follow = m.offset >= m.maxOffset()
}

// PageUp scrolls up one screen.
func (m *Model) PageUp() { m.ScrollBy(-max(m.height, 1)) }

// PageDown scrolls down one screen.
func (m *Model) PageDown() { m.ScrollBy(max(m.height, 1)) }

// Top jumps to the first line.
func (m *Model) Top() { m.ScrollBy(-m.total) }

// Bottom jumps to the last line and resumes following.
func (m *Model) Bottom() { m.ScrollBy(m.total) }

// Following reports whether the view is pinned to the newest line.
func (m *Model) Following() bool { return m.follow }

// Offset returns the first visible line index.
func (m *Model) Offset() int { return m.offset }

// LineCount returns the total number of rendered lines.
func (m *Model) LineCount() int { return m.total }

func (m *Model) maxOffset() int { return max(m.total-m.height, 0) }

func (m *Model) clampOffset() {
	if m.follow {
		m.offset = m.maxOffset()
		return
	}
	m.offset = min(max(m.offset, 0), m.maxOffset())
}

// NewFormatter is the formatter the view uses in production.
func NewFormatter(cfg *types.Config, theme *types.Theme) (Formatter, error) {
	return presentation.NewMessageFormatter(cfg, theme)
}
