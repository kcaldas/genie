package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kcaldas/genie/cmd/history"
	"github.com/kcaldas/genie/cmd/tui/ui/editor"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
	"github.com/kcaldas/genie/cmd/tui/ui/style"
)

// Submitter receives the composed text when write mode submits.
type Submitter interface {
	Emit(topic string, data interface{})
}

// WriteMode is the full-screen composer for long or multi-line messages.
// It wraps an editor.Multi and, on submit, records the text in history
// and emits it as a user.input.text event.
type WriteMode struct {
	editor  *editor.Multi
	bus     Submitter
	history history.ChatHistory

	w, h    int
	offsetY int // first visual row shown
}

// NewWriteMode opens the composer prefilled with content.
func NewWriteMode(content string, vim bool, bus Submitter, hist history.ChatHistory) *WriteMode {
	return &WriteMode{editor: editor.NewMulti(content, vim), bus: bus, history: hist}
}

// SetVim switches vi emulation while open.
func (w *WriteMode) SetVim(on bool) { w.editor.SetVim(on) }

// ModeLabel returns the vi mode label, or "" with vim off.
func (w *WriteMode) ModeLabel() string { return w.editor.ModeLabel() }

// Text returns the composed text.
func (w *WriteMode) Text() string { return w.editor.Text() }

// SetSize implements Dialog.
func (w *WriteMode) SetSize(width, height int) { w.w, w.h = width, height }

// Fullscreen implements Dialog.
func (w *WriteMode) Fullscreen() bool { return true }

// ShowsStatus implements Dialog: the status row stays visible.
func (w *WriteMode) ShowsStatus() bool { return true }

// Update implements Dialog.
func (w *WriteMode) Update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		w.editor.Buf.InsertString(msg.Content)
	case tea.KeyPressMsg:
		switch w.editor.Key(msg) {
		case editor.ActionSubmit:
			text := strings.TrimSpace(w.editor.Text())
			if w.history != nil {
				w.history.AddCommand(text)
			}
			w.bus.Emit("user.input.text", text)
			return nil, true
		case editor.ActionClose:
			return nil, true
		}
	}
	return nil, false
}

// frameRect is the editor frame: the full width, every row but the
// status row the root draws underneath.
func (w *WriteMode) frameRect() layout.Rect {
	return layout.Rect{X: 0, Y: 0, W: w.w, H: max(w.h-layout.StatusHeight, 0)}
}

func (w *WriteMode) innerSize() (int, int) {
	r := w.frameRect()
	return max(r.W-2, 0), max(r.H-2, 0)
}

// View implements Dialog.
func (w *WriteMode) View(th *style.Theme) string {
	r := w.frameRect()
	if r.Empty() {
		return ""
	}
	innerW, innerH := w.innerSize()
	rows, cursorRow := w.visualRows(innerW)
	w.scrollTo(cursorRow, innerH)
	end := min(w.offsetY+innerH, len(rows))
	visible := ""
	if w.offsetY < end {
		visible = strings.Join(rows[w.offsetY:end], "\n")
	}
	return th.RenderFrame(r, style.Frame{
		Title:    w.title(),
		Subtitle: w.subtitle(),
		Focused:  true,
		Border:   true,
	}, visible)
}

func (w *WriteMode) title() string {
	if label := w.editor.ModeLabel(); label != "" {
		return " " + label + " "
	}
	return " Write Mode - Compose Message "
}

func (w *WriteMode) subtitle() string {
	if w.editor.Vim() {
		return " Ctrl+S: Submit | :q Cancel | Ctrl+C/L: Clear "
	}
	return " Ctrl+S: Submit | Esc: Cancel | Ctrl+C/L: Clear "
}

// Cursor implements Dialog, mapping the buffer cursor through soft wrap
// and the scroll offset to a screen cell inside the frame.
func (w *WriteMode) Cursor() *tea.Cursor {
	innerW, innerH := w.innerSize()
	if innerW <= 0 || innerH <= 0 {
		return nil
	}
	_, cursorRow := w.visualRows(innerW)
	w.scrollTo(cursorRow, innerH)
	_, col := w.editor.Buf.Cursor()
	c := tea.NewCursor(1+col%innerW, 1+cursorRow-w.offsetY)
	if w.editor.Vim() && w.editor.Mode() == editor.ModeNormal {
		c.Shape = tea.CursorBlock
	} else {
		c.Shape = tea.CursorBar
	}
	return c
}

// visualRows soft-wraps every logical line to width and returns the rows
// plus the row holding the cursor. A line whose length is a multiple of
// the width gets one extra empty row so a cursor at its end has a cell.
func (w *WriteMode) visualRows(width int) ([]string, int) {
	if width <= 0 {
		return nil, 0
	}
	row, col := w.editor.Buf.Cursor()
	var rows []string
	cursorRow := 0
	for i, line := range w.editor.Buf.Lines() {
		runes := []rune(line)
		if i == row {
			cursorRow = len(rows) + col/width
		}
		for start := 0; ; start += width {
			end := min(start+width, len(runes))
			rows = append(rows, string(runes[start:end]))
			if end >= len(runes) {
				break
			}
		}
	}
	return rows, cursorRow
}

// scrollTo keeps cursorRow inside the visible window of height rows.
func (w *WriteMode) scrollTo(cursorRow, height int) {
	if height <= 0 {
		return
	}
	if cursorRow < w.offsetY {
		w.offsetY = cursorRow
	} else if cursorRow >= w.offsetY+height {
		w.offsetY = cursorRow - height + 1
	}
}
