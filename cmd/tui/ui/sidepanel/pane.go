// Package sidepanel is the right-hand column: a text viewer for help and
// tool output, a diff viewer, and the debug log, all scrolling through one
// shared pane.
package sidepanel

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// pane holds pre-rendered lines and a two-dimensional scroll offset.
type pane struct {
	lines         []string
	offX, offY    int
	width, height int
	wrap          bool // wrap long lines instead of scrolling horizontally
	stick         bool // keep the newest line visible as content grows
}

// setLines replaces the content, wrapping it to the pane width when wrap
// is set, and clamps the offsets.
func (p *pane) setLines(lines []string) {
	if p.wrap && p.width > 0 {
		wrapped := make([]string, 0, len(lines))
		for _, l := range lines {
			if ansi.StringWidth(l) <= p.width {
				wrapped = append(wrapped, l)
				continue
			}
			wrapped = append(wrapped, strings.Split(ansi.Hardwrap(l, p.width, true), "\n")...)
		}
		lines = wrapped
	}
	p.lines = lines
	if p.stick {
		p.offY = p.maxY()
	}
	p.clamp()
}

func (p *pane) setSize(w, h int) {
	p.width, p.height = w, h
	p.clamp()
}

func (p *pane) maxY() int { return max(len(p.lines)-p.height, 0) }

func (p *pane) clamp() {
	p.offY = min(max(p.offY, 0), p.maxY())
	p.offX = max(p.offX, 0)
}

func (p *pane) scrollBy(dy int) {
	p.offY += dy
	p.clamp()
	p.stick = p.offY >= p.maxY()
}

func (p *pane) scrollX(dx int) {
	p.offX += dx
	p.clamp()
}

func (p *pane) top()    { p.offY = 0; p.stick = false }
func (p *pane) bottom() { p.offY = p.maxY(); p.stick = true }

// view returns the visible window, one line per row, cut at the width.
func (p *pane) view() string {
	if p.height <= 0 || p.width <= 0 {
		return ""
	}
	end := min(p.offY+p.height, len(p.lines))
	out := make([]string, 0, p.height)
	for _, l := range p.lines[p.offY:end] {
		if p.offX > 0 {
			l = ansi.TruncateLeft(l, p.offX, "")
		}
		out = append(out, ansi.Truncate(l, p.width, ""))
	}
	return strings.Join(out, "\n")
}
