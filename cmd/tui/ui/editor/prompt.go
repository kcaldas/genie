package editor

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/history"
)

// Suggester proposes a completion for the text typed so far. It returns
// the full completed text, or "" when it has nothing to offer.
type Suggester interface {
	Suggest(input string) string
}

// Chips: a large paste is kept out of the buffer and shown as one
// "[Pasted N lines]" token. The token is a private-use rune so every
// cursor motion and deletion treats it as a single character.
const (
	chipBase      = 0xF0000 // first private-use rune used for chips
	chipMaxCount  = 0xFFF
	ChipMinLines  = 3   // pastes with at least this many lines collapse
	ChipMinLength = 400 // or at least this many runes
)

// Prompt is the multi-line input model: a rune buffer with inline
// completion, command history, collapsed pastes and a soft-wrapped,
// vertically scrolled window.
type Prompt struct {
	buf       *Buffer
	suggester Suggester
	history   history.ChatHistory

	navigating bool   // walking history with up/down
	savedInput string // what was typed before history navigation began

	chips    map[rune]string // chip rune -> pasted text
	nextChip rune
	scroll   int // first visual row shown
}

// NewPrompt returns an empty prompt. Either dependency may be nil.
func NewPrompt(suggester Suggester, hist history.ChatHistory) *Prompt {
	return &Prompt{buf: NewBuffer(""), suggester: suggester, history: hist, chips: map[rune]string{}, nextChip: chipBase}
}

// Text returns the typed text with every chip expanded to its paste.
func (p *Prompt) Text() string { return p.expand(p.buf.String()) }

// Raw returns the buffer text with chips still collapsed.
func (p *Prompt) Raw() string { return p.buf.String() }

// IsEmpty reports whether nothing has been typed.
func (p *Prompt) IsEmpty() bool { return p.buf.IsEmpty() }

// Lines returns the number of logical lines.
func (p *Prompt) Lines() int { return p.buf.LineCount() }

// Cursor returns the logical cursor row and column.
func (p *Prompt) Cursor() (row, col int) { return p.buf.Cursor() }

// Set replaces the text (chips dropped) and puts the cursor at the end.
func (p *Prompt) Set(text string) {
	p.buf.Set(text)
	p.chips = map[rune]string{}
	p.scroll = 0
}

// Clear empties the prompt.
func (p *Prompt) Clear() {
	p.buf.Clear()
	p.chips = map[rune]string{}
	p.scroll = 0
}

// Insert types one rune at the cursor; '\n' starts a new line.
func (p *Prompt) Insert(r rune) {
	if r == '\r' {
		r = '\n'
	}
	p.buf.Insert(r)
	p.edited()
}

// InsertString types text at the cursor, keeping its newlines.
func (p *Prompt) InsertString(s string) {
	for _, r := range s {
		p.Insert(r)
	}
}

// Newline starts a new line at the cursor.
func (p *Prompt) Newline() {
	p.buf.Newline()
	p.edited()
}

// Paste inserts text, collapsing it into a chip when it is large enough
// (ChipMinLines lines or ChipMinLength runes). It reports whether a chip
// was made.
func (p *Prompt) Paste(text string) (chip bool) {
	if strings.Count(text, "\n") < ChipMinLines-1 && len([]rune(text)) < ChipMinLength {
		p.InsertString(text)
		return false
	}
	if p.nextChip-chipBase >= chipMaxCount {
		p.InsertString(text)
		return false
	}
	r := p.nextChip
	p.nextChip++
	p.chips[r] = text
	p.buf.Insert(r)
	p.edited()
	return true
}

// ChipLabel is how a collapsed paste is shown.
func ChipLabel(text string) string {
	n := strings.Count(strings.TrimRight(text, "\n"), "\n") + 1
	if n == 1 {
		return fmt.Sprintf("[Pasted %d chars]", len([]rune(text)))
	}
	return fmt.Sprintf("[Pasted %d lines]", n)
}

// ChipLabels returns the labels of the chips currently in the text, for
// the view to highlight.
func (p *Prompt) ChipLabels() []string {
	var out []string
	for _, r := range p.buf.String() {
		if text, ok := p.chips[r]; ok {
			out = append(out, ChipLabel(text))
		}
	}
	return out
}

// Backspace deletes the rune before the cursor (a whole chip counts as one).
func (p *Prompt) Backspace() {
	if p.buf.Backspace() {
		p.edited()
	}
}

// Delete deletes the rune under the cursor.
func (p *Prompt) Delete() {
	if p.buf.Delete() {
		p.edited()
	}
}

// Left moves the cursor one rune left.
func (p *Prompt) Left() { p.buf.Left() }

// Right moves the cursor one rune right. At the end of the text it accepts
// the pending suggestion instead, and reports whether it did.
func (p *Prompt) Right() (accepted bool) {
	if p.atEnd() && p.Suggestion() != "" {
		return p.AcceptSuggestion()
	}
	p.buf.Right()
	return false
}

// Up moves the cursor up a logical line and reports whether it could.
func (p *Prompt) Up() bool { return p.buf.Up() }

// Down moves the cursor down a logical line and reports whether it could.
func (p *Prompt) Down() bool { return p.buf.Down() }

// Home moves the cursor to the start of its line.
func (p *Prompt) Home() { p.buf.LineStart() }

// End moves the cursor to the end of its line.
func (p *Prompt) End() { p.buf.LineEnd() }

// WordLeft moves the cursor to the previous word start.
func (p *Prompt) WordLeft() { p.buf.WordLeft() }

// WordRight moves the cursor to the next word start.
func (p *Prompt) WordRight() { p.buf.WordRight() }

// DeleteWordLeft deletes the word before the cursor.
func (p *Prompt) DeleteWordLeft() {
	if p.buf.DeleteWordLeft() {
		p.edited()
	}
}

// OnFirstLine reports whether the cursor is on the first logical line.
func (p *Prompt) OnFirstLine() bool { row, _ := p.buf.Cursor(); return row == 0 }

// OnLastLine reports whether the cursor is on the last logical line.
func (p *Prompt) OnLastLine() bool {
	row, _ := p.buf.Cursor()
	return row == p.buf.LineCount()-1
}

func (p *Prompt) atEnd() bool {
	row, col := p.buf.Cursor()
	return row == p.buf.LineCount()-1 && col == p.buf.LineLen(row)
}

// Suggestion returns the ghost text to show after the cursor: the part of
// the completion not yet typed. Completion only applies to a single-line
// prompt with the cursor at its end.
func (p *Prompt) Suggestion() string {
	if p.suggester == nil || p.buf.LineCount() != 1 || !p.atEnd() || len(p.chips) > 0 {
		return ""
	}
	text := p.buf.String()
	full := p.suggester.Suggest(text)
	if full == "" || full == text || !strings.HasPrefix(full, text) {
		return ""
	}
	return full[len(text):]
}

// AcceptSuggestion appends the ghost text and reports whether there was any.
func (p *Prompt) AcceptSuggestion() bool {
	ghost := p.Suggestion()
	if ghost == "" {
		return false
	}
	p.buf.InsertString(ghost)
	return true
}

// Complete is the Tab action: accept the suggestion when there is one.
func (p *Prompt) Complete() bool { return p.AcceptSuggestion() }

// HistoryPrev replaces the text with the previous history entry, saving
// what was typed so HistoryNext can restore it.
func (p *Prompt) HistoryPrev() {
	if p.history == nil {
		return
	}
	if !p.navigating {
		p.savedInput = p.Text()
		p.navigating = true
	}
	p.Set(p.history.NavigatePrev())
}

// HistoryNext replaces the text with the next history entry, or with the
// saved input once the newest entry has been passed.
func (p *Prompt) HistoryNext() {
	if p.history == nil {
		return
	}
	cmd := p.history.NavigateNext()
	if cmd == "" && p.navigating {
		cmd = p.savedInput
		p.navigating = false
	}
	p.Set(cmd)
}

// Submit returns the trimmed text with chips expanded, records it in
// history and empties the prompt. It returns "" for blank input.
func (p *Prompt) Submit() string {
	text := strings.TrimSpace(p.Text())
	if text == "" {
		return ""
	}
	if p.history != nil {
		p.history.AddCommand(text)
	}
	p.ResetHistory()
	p.Clear()
	return text
}

// ResetHistory leaves history navigation mode.
func (p *Prompt) ResetHistory() {
	if p.history != nil {
		p.history.ResetNavigation()
	}
	p.navigating = false
	p.savedInput = ""
}

// Window is the prompt laid out for display.
type Window struct {
	Rows      []string // visible rows, chips shown as their labels
	CursorRow int      // within Rows
	CursorCol int
	Ghost     string // suggestion to draw after the cursor, "" if none
	Total     int    // visual rows in the whole text
}

// Layout soft-wraps the text to width, keeps the cursor inside a window
// of at most maxRows rows, and returns what to draw.
func (p *Prompt) Layout(width, maxRows int) Window {
	if width <= 0 || maxRows <= 0 {
		return Window{}
	}
	var rows []string
	cursorRow, cursorCol := 0, 0
	curRow, curCol := p.buf.Cursor()
	for i, line := range p.buf.Lines() {
		display, cols := p.display(line)
		if i == curRow {
			dcol := cols[min(curCol, len(cols)-1)]
			cursorRow = len(rows) + dcol/width
			cursorCol = dcol % width
		}
		for start := 0; ; start += width {
			end := min(start+width, len(display))
			rows = append(rows, string(display[start:end]))
			if end >= len(display) {
				break
			}
		}
	}
	if cursorRow < p.scroll {
		p.scroll = cursorRow
	} else if cursorRow >= p.scroll+maxRows {
		p.scroll = cursorRow - maxRows + 1
	}
	p.scroll = min(p.scroll, max(len(rows)-maxRows, 0))
	end := min(p.scroll+maxRows, len(rows))
	w := Window{Rows: rows[p.scroll:end], CursorRow: cursorRow - p.scroll, CursorCol: cursorCol, Total: len(rows)}
	if room := width - cursorCol; room > 0 {
		w.Ghost = truncateRunes(p.Suggestion(), room)
	}
	return w
}

// display expands chips in a line to their labels and returns the display
// runes plus, for every buffer column (0..len+1), its display column.
func (p *Prompt) display(line string) (display []rune, cols []int) {
	for _, r := range line {
		cols = append(cols, len(display))
		if text, ok := p.chips[r]; ok {
			display = append(display, []rune(ChipLabel(text))...)
			continue
		}
		display = append(display, r)
	}
	cols = append(cols, len(display))
	return display, cols
}

// expand replaces chip runes with their pasted text.
func (p *Prompt) expand(s string) string {
	if len(p.chips) == 0 {
		return s
	}
	var sb strings.Builder
	for _, r := range s {
		if text, ok := p.chips[r]; ok {
			sb.WriteString(text)
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// edited leaves history navigation once the user types.
func (p *Prompt) edited() {
	if p.navigating {
		p.ResetHistory()
	}
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
