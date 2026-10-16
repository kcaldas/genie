package editor

import (
	"strings"

	"github.com/kcaldas/genie/cmd/history"
)

// Suggester proposes a completion for the text typed so far. It returns
// the full completed text, or "" when it has nothing to offer.
type Suggester interface {
	Suggest(input string) string
}

// Line is the single-line input model: a rune buffer with inline
// completion, command history and a horizontal scroll window. It mirrors
// the behaviour of the old gocui shell editor without any view dependency.
type Line struct {
	buf       *Buffer
	suggester Suggester
	history   history.ChatHistory

	navigating bool   // walking history with up/down
	savedInput string // what was typed before history navigation began
	scroll     int    // first visible rune when the text is wider than the view
}

// NewLine returns an empty input line. Either dependency may be nil.
func NewLine(suggester Suggester, hist history.ChatHistory) *Line {
	return &Line{buf: NewBuffer(""), suggester: suggester, history: hist}
}

// Text returns the typed text.
func (l *Line) Text() string { return l.buf.String() }

// Cursor returns the cursor position in runes.
func (l *Line) Cursor() int {
	_, col := l.buf.Cursor()
	return col
}

// IsEmpty reports whether nothing has been typed.
func (l *Line) IsEmpty() bool { return l.buf.IsEmpty() }

// Set replaces the text and puts the cursor at the end.
func (l *Line) Set(text string) {
	l.buf.Set(flatten(text))
	l.scroll = 0
}

// Clear empties the line.
func (l *Line) Clear() {
	l.buf.Clear()
	l.scroll = 0
}

// Insert types one rune at the cursor. Newlines are typed as spaces.
func (l *Line) Insert(r rune) {
	if r == '\n' || r == '\r' {
		r = ' '
	}
	l.buf.Insert(r)
	l.edited()
}

// InsertString types text at the cursor.
func (l *Line) InsertString(s string) {
	for _, r := range s {
		l.Insert(r)
	}
}

// Backspace deletes the rune before the cursor.
func (l *Line) Backspace() {
	if l.buf.Backspace() {
		l.edited()
	}
}

// Delete deletes the rune under the cursor.
func (l *Line) Delete() {
	if l.buf.Delete() {
		l.edited()
	}
}

// Left moves the cursor one rune left.
func (l *Line) Left() { l.buf.Left() }

// Right moves the cursor one rune right. At the end of the text it accepts
// the pending suggestion instead, and reports whether it did.
func (l *Line) Right() (accepted bool) {
	if l.Cursor() == l.len() && l.Suggestion() != "" {
		return l.AcceptSuggestion()
	}
	l.buf.Right()
	return false
}

// Home moves the cursor to the start.
func (l *Line) Home() { l.buf.LineStart() }

// End moves the cursor to the end.
func (l *Line) End() { l.buf.LineEnd() }

// WordLeft moves the cursor to the previous word start.
func (l *Line) WordLeft() { l.buf.WordLeft() }

// WordRight moves the cursor to the next word start.
func (l *Line) WordRight() { l.buf.WordRight() }

// DeleteWordLeft deletes the word before the cursor.
func (l *Line) DeleteWordLeft() {
	if l.buf.DeleteWordLeft() {
		l.edited()
	}
}

// Suggestion returns the ghost text to show after the cursor: the part of
// the completion not yet typed. It is empty unless the cursor is at the
// end and the suggester extends the current text.
func (l *Line) Suggestion() string {
	if l.suggester == nil || l.Cursor() != l.len() {
		return ""
	}
	text := l.Text()
	full := l.suggester.Suggest(text)
	if full == "" || full == text || !strings.HasPrefix(full, text) {
		return ""
	}
	return full[len(text):]
}

// AcceptSuggestion appends the ghost text and reports whether there was any.
func (l *Line) AcceptSuggestion() bool {
	ghost := l.Suggestion()
	if ghost == "" {
		return false
	}
	l.buf.InsertString(ghost)
	return true
}

// Complete is the Tab action: accept the full suggestion when it extends
// the current text.
func (l *Line) Complete() bool { return l.AcceptSuggestion() }

// HistoryPrev replaces the text with the previous history entry, saving
// what was typed so HistoryNext can restore it.
func (l *Line) HistoryPrev() {
	if l.history == nil {
		return
	}
	if !l.navigating {
		l.savedInput = l.Text()
		l.navigating = true
	}
	l.Set(l.history.NavigatePrev())
}

// HistoryNext replaces the text with the next history entry, or with the
// saved input once the newest entry has been passed.
func (l *Line) HistoryNext() {
	if l.history == nil {
		return
	}
	cmd := l.history.NavigateNext()
	if cmd == "" && l.navigating {
		cmd = l.savedInput
		l.navigating = false
	}
	l.Set(cmd)
}

// Submit returns the trimmed, single-line text and records it in history,
// leaving the line empty. It returns "" and does nothing for blank input.
func (l *Line) Submit() string {
	text := strings.TrimSpace(flatten(l.Text()))
	if text == "" {
		return ""
	}
	if l.history != nil {
		l.history.AddCommand(text)
	}
	l.ResetHistory()
	l.Clear()
	return text
}

// ResetHistory leaves history navigation mode.
func (l *Line) ResetHistory() {
	if l.history != nil {
		l.history.ResetNavigation()
	}
	l.navigating = false
	l.savedInput = ""
}

// Window returns what fits in width cells: the visible text, the cursor
// column within it, and the ghost suggestion truncated to the remaining
// room. The scroll offset keeps the cursor visible with one cell of margin.
func (l *Line) Window(width int) (visible string, cursorX int, ghost string) {
	if width <= 0 {
		return "", 0, ""
	}
	runes := []rune(l.Text())
	cur := l.Cursor()
	if cur < l.scroll {
		l.scroll = cur
	} else if cur >= l.scroll+width-1 {
		l.scroll = max(cur-width+2, 0)
	}
	end := min(l.scroll+width, len(runes))
	visible = string(runes[l.scroll:end])
	cursorX = clamp(cur-l.scroll, 0, width-1)
	if room := width - (cur - l.scroll); room > 0 {
		ghost = truncateRunes(l.Suggestion(), room)
	}
	return visible, cursorX, ghost
}

func (l *Line) len() int { return l.buf.LineLen(0) }

// edited leaves history navigation once the user types.
func (l *Line) edited() {
	if l.navigating {
		l.ResetHistory()
	}
}

// flatten turns a multi-line command into one line for the input field.
func flatten(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
