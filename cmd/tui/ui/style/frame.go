package style

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
)

// Frame describes the chrome around a panel's content.
type Frame struct {
	Title    string // drawn on the top border, left
	Subtitle string // drawn on the top border, right
	Focused  bool
	Border   bool // false renders the bare content clamped to the rect
	Accent   bool // draw border and titles in the secondary colour (dialogs)
}

var rounded = lipgloss.RoundedBorder()

// RenderFrame fits content into r, drawing a rounded border with the title
// and subtitle on its top edge when f.Border is set. Content is clamped so
// the result is exactly r.W wide and r.H tall; an empty rect renders "".
func (t *Theme) RenderFrame(r layout.Rect, f Frame, content string) string {
	if r.Empty() {
		return ""
	}
	if !f.Border {
		return Fit(content, r.W, r.H)
	}
	if r.W < 2 || r.H < 2 {
		return Fit("", r.W, r.H)
	}

	border, title := t.Border, t.Title
	if f.Focused {
		border, title = t.BorderFocused, t.TitleFocused
	}
	if f.Accent {
		border, title = t.Secondary, t.Secondary
	}
	innerW, innerH := r.W-2, r.H-2

	var sb strings.Builder
	sb.WriteString(border.Render(rounded.TopLeft))
	sb.WriteString(topEdge(innerW, f.Title, f.Subtitle, border, title))
	sb.WriteString(border.Render(rounded.TopRight))
	sb.WriteByte('\n')

	left, right := border.Render(rounded.Left), border.Render(rounded.Right)
	for _, line := range strings.Split(Fit(content, innerW, innerH), "\n") {
		sb.WriteString(left)
		sb.WriteString(line)
		sb.WriteString(right)
		sb.WriteByte('\n')
	}

	sb.WriteString(border.Render(rounded.BottomLeft))
	sb.WriteString(border.Render(strings.Repeat(rounded.Top, innerW)))
	sb.WriteString(border.Render(rounded.BottomRight))
	return sb.String()
}

// topEdge draws the top border of width w with the title at the left and
// the subtitle at the right, dropping whichever does not fit.
func topEdge(w int, title, subtitle string, border, titleStyle lipgloss.Style) string {
	title = ansi.Truncate(title, w, "")
	tw := ansi.StringWidth(title)
	subtitle = ansi.Truncate(subtitle, max(w-tw, 0), "")
	sw := ansi.StringWidth(subtitle)
	fill := max(w-tw-sw, 0)
	return titleStyle.Render(title) + border.Render(strings.Repeat(rounded.Top, fill)) + titleStyle.Render(subtitle)
}

// Fit clamps content to exactly w columns by h rows: long lines are cut,
// extra lines dropped, short lines and missing rows padded with spaces.
func Fit(content string, w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	out := make([]string, h)
	for i := range out {
		line := ""
		if i < len(lines) {
			line = ansi.Truncate(lines[i], w, "")
		}
		if pad := w - ansi.StringWidth(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		out[i] = line
	}
	return strings.Join(out, "\n")
}
