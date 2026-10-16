package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
	"github.com/kcaldas/genie/cmd/tui/ui/render"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
)

// ConfirmAnswerMsg is produced when the user answers a confirmation.
type ConfirmAnswerMsg struct{ Answer uimsg.ConfirmationAnswer }

// Confirm is the yes/no dialog for tool and user confirmations. It floats
// over the chat with the request's content (text, markdown or a diff)
// scrollable inside it; Ctrl+Z expands it to the whole screen.
type Confirm struct {
	req    uimsg.ConfirmationRequested
	cfg    render.Config
	w, h   int
	zoomed bool

	lines  []string // rendered body at the current inner width
	bodyW  int
	offset int
}

// NewConfirm builds the dialog for a request.
func NewConfirm(req uimsg.ConfirmationRequested, cfg render.Config) *Confirm {
	return &Confirm{req: req, cfg: cfg}
}

// ExecutionID identifies the request being answered.
func (c *Confirm) ExecutionID() string { return c.req.ExecutionID }

// SetSize implements Dialog.
func (c *Confirm) SetSize(w, h int) { c.w, c.h = w, h }

// Fullscreen implements Dialog: only when zoomed.
func (c *Confirm) Fullscreen() bool { return c.zoomed }

// ShowsStatus implements Dialog.
func (c *Confirm) ShowsStatus() bool { return true }

// Cursor implements Dialog: no text cursor.
func (c *Confirm) Cursor() *tea.Cursor { return nil }

// Rect is where the dialog sits on screen: centred, about four fifths of
// each dimension, or the whole screen above the status row when zoomed.
func (c *Confirm) Rect() layout.Rect {
	if c.zoomed {
		return layout.Rect{X: 0, Y: 0, W: c.w, H: max(c.h-layout.StatusHeight, 0)}
	}
	w := min(c.w-4, max(c.w*4/5, 40))
	h := min(c.h-3, max(c.h*4/5, 8))
	w, h = max(min(w, c.w), 0), max(min(h, c.h), 0)
	return layout.Rect{X: (c.w - w) / 2, Y: (c.h - layout.StatusHeight - h) / 2, W: w, H: h}
}

// Update implements Dialog.
func (c *Confirm) Update(msg tea.Msg) (tea.Cmd, bool) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil, false
	}
	switch k.String() {
	case "1", "y", "Y":
		return c.answer(true), true
	case "2", "n", "N", "esc":
		return c.answer(false), true
	case "ctrl+z":
		c.zoomed = !c.zoomed
	case "up", "k":
		c.scroll(-1)
	case "down", "j":
		c.scroll(1)
	case "pgup", "ctrl+u":
		c.scroll(-c.bodyHeight())
	case "pgdown", "ctrl+d":
		c.scroll(c.bodyHeight())
	case "home":
		c.offset = 0
	case "end":
		c.offset = c.maxOffset()
	}
	return nil, false
}

func (c *Confirm) answer(yes bool) tea.Cmd {
	a := uimsg.ConfirmationAnswer{Kind: c.req.Kind, ExecutionID: c.req.ExecutionID, Confirmed: yes}
	return func() tea.Msg { return ConfirmAnswerMsg{Answer: a} }
}

func (c *Confirm) scroll(n int) {
	c.offset = min(max(c.offset+n, 0), c.maxOffset())
}

// bodyHeight is the rows available for content: the frame minus the
// prompt line and its separator.
func (c *Confirm) bodyHeight() int { return max(c.Rect().H-2-2, 1) }

func (c *Confirm) maxOffset() int {
	c.ensureRendered()
	return max(len(c.lines)-c.bodyHeight(), 0)
}

// ensureRendered renders the body for the current width once.
func (c *Confirm) ensureRendered() {
	innerW := max(c.Rect().W-2, 1)
	if c.lines != nil && c.bodyW == innerW {
		return
	}
	c.bodyW = innerW
	c.lines = nil
	for _, l := range render.Body(c.cfg, c.req.Content, c.req.ContentType, innerW) {
		if ansi.StringWidth(l) <= innerW {
			c.lines = append(c.lines, l)
			continue
		}
		if c.req.ContentType == "diff" {
			c.lines = append(c.lines, ansi.Truncate(l, innerW, "…"))
			continue
		}
		c.lines = append(c.lines, strings.Split(ansi.Hardwrap(l, innerW, true), "\n")...)
	}
	c.offset = min(c.offset, c.maxOffset())
}

// View implements Dialog, rendering just the dialog box.
func (c *Confirm) View(th *style.Theme) string {
	r := c.Rect()
	if r.Empty() {
		return ""
	}
	c.ensureRendered()
	innerW := max(r.W-2, 1)
	bodyH := c.bodyHeight()
	end := min(c.offset+bodyH, len(c.lines))
	body := ""
	if c.offset < end {
		body = strings.Join(c.lines[c.offset:end], "\n")
	}
	body = style.Fit(body, innerW, bodyH)
	rule := th.Secondary.Render(strings.Repeat("─", innerW))
	prompt := lipgloss.PlaceHorizontal(innerW, lipgloss.Center, th.Secondary.Render(c.req.Prompt))
	hint := ""
	if len(c.lines) > bodyH {
		hint = " ↑↓ scroll · Ctrl+Z zoom "
	}
	return th.RenderFrame(r, style.Frame{
		Title:    " " + c.req.Title + " ",
		Subtitle: hint,
		Border:   true,
		Accent:   true,
	}, body+"\n"+rule+"\n"+prompt)
}
