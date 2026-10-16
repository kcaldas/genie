package editor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
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

	undo, redo []Snapshot
	lastEdit   editKind // groups a run of typing into one undo step
}

// editKind classifies edits so consecutive edits of the same kind merge
// into one undo step, while a newline, paste or deletion starts a new one.
type editKind int

const (
	editNone editKind = iota
	editType
	editDelete
	editOther
)

// maxUndo bounds the undo history.
const maxUndo = 200

// NewPrompt returns an empty prompt. Either dependency may be nil.
func NewPrompt(suggester Suggester, hist history.ChatHistory) *Prompt {
	return &Prompt{buf: NewBuffer(""), suggester: suggester, history: hist, chips: map[rune]string{}, nextChip: chipBase}
}

// Text returns the typed text with every chip expanded to its paste.
func (p *Prompt) Text() string { return p.expand(p.buf.String()) }

// Buffer exposes the underlying buffer for the vi state machine. Call
// Checkpoint before letting it edit so the change is undoable.
func (p *Prompt) Buffer() *Buffer { return p.buf }

// Checkpoint records an undo step for an edit made directly on Buffer.
func (p *Prompt) Checkpoint() { p.record(editOther) }

// DiscardCheckpoint drops the snapshot Checkpoint just took, for a key
// that turned out not to change the text.
func (p *Prompt) DiscardCheckpoint() {
	if n := len(p.undo); n > 0 {
		p.undo = p.undo[:n-1]
		p.lastEdit = editNone
	}
}

// Raw returns the buffer text with chips still collapsed.
func (p *Prompt) Raw() string { return p.buf.String() }

// IsEmpty reports whether nothing has been typed.
func (p *Prompt) IsEmpty() bool { return p.buf.IsEmpty() }

// Lines returns the number of logical lines.
func (p *Prompt) Lines() int { return p.buf.LineCount() }

// Cursor returns the logical cursor row and column.
func (p *Prompt) Cursor() (row, col int) { return p.buf.Cursor() }

// Set replaces the text and puts the cursor at the end. It is undoable.
// Chips already made stay known, so undoing back to them still expands.
func (p *Prompt) Set(text string) {
	p.record(editOther)
	p.set(text)
}

func (p *Prompt) set(text string) {
	p.buf.Set(text)
	p.scroll = 0
}

// Clear empties the prompt and forgets its undo history.
func (p *Prompt) Clear() {
	p.buf.Clear()
	p.chips = map[rune]string{}
	p.scroll = 0
	p.undo, p.redo, p.lastEdit = nil, nil, editNone
}

// record saves a snapshot before an edit of the given kind unless it
// continues the previous edit of the same mergeable kind.
func (p *Prompt) record(kind editKind) {
	p.redo = nil
	if kind == p.lastEdit && (kind == editType || kind == editDelete) {
		return
	}
	p.lastEdit = kind
	p.undo = append(p.undo, p.buf.Snapshot())
	if len(p.undo) > maxUndo {
		p.undo = p.undo[1:]
	}
}

// breakUndoGroup makes the next edit start a new undo step, for cursor
// moves between bursts of typing.
func (p *Prompt) breakUndoGroup() { p.lastEdit = editNone }

// Undo reverts the last edit step and reports whether there was one.
func (p *Prompt) Undo() bool {
	if len(p.undo) == 0 {
		return false
	}
	p.redo = append(p.redo, p.buf.Snapshot())
	p.buf.Restore(p.undo[len(p.undo)-1])
	p.undo = p.undo[:len(p.undo)-1]
	p.lastEdit = editNone
	return true
}

// Redo re-applies the last undone step and reports whether there was one.
func (p *Prompt) Redo() bool {
	if len(p.redo) == 0 {
		return false
	}
	p.undo = append(p.undo, p.buf.Snapshot())
	p.buf.Restore(p.redo[len(p.redo)-1])
	p.redo = p.redo[:len(p.redo)-1]
	p.lastEdit = editNone
	return true
}

// Insert types one rune at the cursor; '\n' starts a new line and a tab
// becomes a space, since the prompt has no tab stops.
func (p *Prompt) Insert(r rune) {
	switch r {
	case '\r':
		r = '\n'
	case '\t':
		r = ' '
	}
	if r == '\n' {
		p.record(editOther)
	} else {
		p.record(editType)
	}
	p.buf.Insert(r)
	p.edited()
}

// InsertString inserts text at the cursor as one undo step, keeping its
// newlines.
func (p *Prompt) InsertString(s string) {
	if s == "" {
		return
	}
	p.record(editOther)
	for _, r := range s {
		switch r {
		case '\r':
			r = '\n'
		case '\t':
			r = ' '
		}
		p.buf.Insert(r)
	}
	p.edited()
}

// RuneBefore returns the rune before the cursor, or 0 at a line start.
func (p *Prompt) RuneBefore() rune {
	row, col := p.buf.Cursor()
	if col == 0 {
		return 0
	}
	return []rune(p.buf.Line(row))[col-1]
}

// Newline starts a new line at the cursor.
func (p *Prompt) Newline() {
	p.record(editOther)
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
	p.record(editOther)
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
	if row, col := p.buf.Cursor(); row == 0 && col == 0 {
		return
	}
	p.record(editDelete)
	if p.buf.Backspace() {
		p.edited()
	}
}

// Delete deletes the rune under the cursor.
func (p *Prompt) Delete() {
	if p.atEnd() {
		return
	}
	p.record(editDelete)
	if p.buf.Delete() {
		p.edited()
	}
}

// Left moves the cursor one rune left.
func (p *Prompt) Left() { p.breakUndoGroup(); p.buf.Left() }

// Right moves the cursor one rune right. At the end of the text it accepts
// the pending suggestion instead, and reports whether it did.
func (p *Prompt) Right() (accepted bool) {
	if p.atEnd() && p.Suggestion() != "" {
		return p.AcceptSuggestion()
	}
	p.breakUndoGroup()
	p.buf.Right()
	return false
}

// Up moves the cursor up a logical line and reports whether it could.
func (p *Prompt) Up() bool { p.breakUndoGroup(); return p.buf.Up() }

// Down moves the cursor down a logical line and reports whether it could.
func (p *Prompt) Down() bool { p.breakUndoGroup(); return p.buf.Down() }

// Home moves the cursor to the start of its line.
func (p *Prompt) Home() { p.breakUndoGroup(); p.buf.LineStart() }

// End moves the cursor to the end of its line.
func (p *Prompt) End() { p.breakUndoGroup(); p.buf.LineEnd() }

// WordLeft moves the cursor to the previous word start.
func (p *Prompt) WordLeft() { p.breakUndoGroup(); p.buf.WordLeft() }

// WordRight moves the cursor to the next word start.
func (p *Prompt) WordRight() { p.breakUndoGroup(); p.buf.WordRight() }

// DeleteWordLeft deletes the word before the cursor.
func (p *Prompt) DeleteWordLeft() {
	p.record(editOther)
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
	p.record(editOther)
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
		p.savedInput = p.Raw()
		p.navigating = true
	}
	p.set(p.history.NavigatePrev())
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
	p.set(cmd)
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

// Layout soft-wraps the text to width cells, keeps the cursor inside a
// window of at most maxRows rows, and returns what to draw. Wrapping and
// cursor placement count terminal cells, so wide runes stay in the frame.
func (p *Prompt) Layout(width, maxRows int) Window {
	if width <= 0 || maxRows <= 0 {
		return Window{}
	}
	var rows []string
	cursorRow, cursorCol := 0, 0
	curRow, curCol := p.buf.Cursor()
	for i, line := range p.buf.Lines() {
		display, cols := p.display(line)
		lineRows, cells := wrapCells(display, width)
		if i == curRow {
			dcol := cols[min(curCol, len(cols)-1)]
			r, c := cellPos(cells, dcol, width)
			cursorRow, cursorCol = len(rows)+r, c
		}
		rows = append(rows, lineRows...)
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

// wrapCells splits display runes into rows of at most width cells and
// returns the rows plus each rune's starting cell offset within the line.
// A line whose last row is exactly full gets one more empty row, so a
// cursor at its end has a cell of its own.
func wrapCells(display []rune, width int) (rows []string, cells []int) {
	cells = make([]int, len(display)+1)
	var sb strings.Builder
	used := 0
	for i, r := range display {
		w := max(ansi.StringWidth(string(r)), 0)
		if used+w > width && used > 0 {
			rows = append(rows, sb.String())
			sb.Reset()
			used = 0
		}
		cells[i] = len(rows)*width + used
		sb.WriteRune(r)
		used += w
	}
	cells[len(display)] = len(rows)*width + used
	rows = append(rows, sb.String())
	if used == width {
		rows = append(rows, "")
	}
	return rows, cells
}

// cellPos maps a display column to a row and cell within wrapped rows.
func cellPos(cells []int, dcol, width int) (row, col int) {
	c := cells[min(dcol, len(cells)-1)]
	return c / width, c % width
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
