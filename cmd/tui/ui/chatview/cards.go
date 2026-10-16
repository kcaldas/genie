package chatview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/render"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
)

// Limits on what an expanded card shows, so a huge tool output cannot
// swamp the transcript.
const (
	maxCardLines = 200
	maxDiffLines = 400
)

// Styling is what the cards need beyond the message formatter: the theme
// for colours and the config for diff rendering.
type Styling struct {
	Theme *style.Theme
	Cfg   render.Config
}

// SetStyling sets the theme and config used by tool and thinking cards
// and invalidates the cache.
func (m *Model) SetStyling(s Styling) {
	m.styling = s
	m.gen++
	m.Sync()
}

// ShowDetails reports whether cards are expanded by default.
func (m *Model) ShowDetails() bool { return m.showDetails }

// ToggleDetails expands or collapses every card at once and forgets
// per-card overrides.
func (m *Model) ToggleDetails() {
	m.showDetails = !m.showDetails
	m.expanded = map[int64]bool{}
	m.gen++
	m.Sync()
}

// ToggleAt flips the card whose rendering covers the visible row y, and
// reports whether a card was there.
func (m *Model) ToggleAt(y int) bool {
	line := m.offset + y
	for _, it := range m.items {
		if line < len(it.lines) {
			if !m.isCard(it.id) {
				return false
			}
			m.expanded[it.id] = !m.isExpanded(it.id)
			m.gen++
			m.Sync()
			return true
		}
		line -= len(it.lines)
	}
	return false
}

func (m *Model) isCard(id int64) bool {
	for _, msg := range m.chat.GetMessages() {
		if msg.ID == id {
			return msg.Tool != nil || msg.ContentType == "thought"
		}
	}
	return false
}

func (m *Model) isExpanded(id int64) bool {
	if v, ok := m.expanded[id]; ok {
		return v
	}
	return m.showDetails
}

// renderCard renders a tool call or a thinking block as a collapsible
// card: one header line, plus the body when expanded.
func (m *Model) renderCard(msg types.Message) []string {
	th := m.styling.Theme
	if th == nil {
		th = style.New("default", 0)
	}
	expanded := m.isExpanded(msg.ID)
	if msg.Tool != nil {
		return m.toolCard(th, msg, expanded)
	}
	return m.thoughtCard(th, msg, expanded)
}

func (m *Model) toolCard(th *style.Theme, msg types.Message, expanded bool) []string {
	t := msg.Tool
	icon, status := th.Success.Render("●"), th.TextTertiary.Render("✓ "+t.Summary)
	if !t.Success {
		icon, status = th.Error.Render("●"), th.Error.Render("✗ "+t.Summary)
	}
	chevron := "▸"
	if expanded {
		chevron = "▾"
	}
	left := fmt.Sprintf("%s %s %s %s", icon, th.TextPrimary.Render(t.Name), th.TextTertiary.Render(t.Args), th.Muted.Render(chevron))
	lines := []string{headerLine(left, status, m.width)}
	if expanded {
		body := m.cardBody(th, t)
		for _, l := range body {
			lines = append(lines, "  "+l)
		}
	}
	return append(m.wrapAll(lines), "")
}

// cardBody is the expanded content: the diff when there is one, else the
// output, each capped with a trailing count of hidden lines.
func (m *Model) cardBody(th *style.Theme, t *types.ToolCall) []string {
	if t.Diff != "" && t.Success {
		return capLines(render.Body(m.styling.Cfg, t.Diff, "diff", max(m.width-2, 1)), maxDiffLines, th)
	}
	if t.Output == "" {
		return nil
	}
	lines := strings.Split(t.Output, "\n")
	styled := make([]string, len(lines))
	for i, l := range lines {
		if t.Success {
			styled[i] = th.TextSecondary.Render(l)
		} else {
			styled[i] = th.Error.Render(l)
		}
	}
	return capLines(styled, maxCardLines, th)
}

func (m *Model) thoughtCard(th *style.Theme, msg types.Message, expanded bool) []string {
	text := strings.TrimSpace(msg.Content)
	count := strings.Count(text, "\n") + 1
	if !expanded {
		first := firstLine(text)
		header := th.Muted.Render(fmt.Sprintf("✱ Thinking (%d lines) ▸ ", count)) + th.Muted.Render(first)
		return append(m.wrapAll([]string{ansi.Truncate(header, m.width, "…")}), "")
	}
	lines := []string{th.Muted.Render(fmt.Sprintf("✱ Thinking (%d lines) ▾", count))}
	for _, l := range strings.Split(text, "\n") {
		lines = append(lines, "  "+th.Muted.Render(l))
	}
	return append(m.wrapAll(lines), "")
}

// headerLine joins a left part and a right-aligned status within width,
// truncating the left part first.
func headerLine(left, right string, width int) string {
	rw := ansi.StringWidth(right)
	room := width - rw - 1
	if room < 10 {
		return ansi.Truncate(left, width, "…")
	}
	left = ansi.Truncate(left, room, "…")
	pad := width - ansi.StringWidth(left) - rw
	return left + strings.Repeat(" ", max(pad, 1)) + right
}

func capLines(lines []string, limit int, th *style.Theme) []string {
	if len(lines) <= limit {
		return lines
	}
	hidden := len(lines) - limit
	return append(lines[:limit:limit], th.TextTertiary.Render(fmt.Sprintf("… %d more lines (Ctrl+O to toggle)", hidden)))
}

func (m *Model) wrapAll(lines []string) []string {
	var out []string
	for _, l := range lines {
		out = append(out, m.wrapLines(l)...)
	}
	return out
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
