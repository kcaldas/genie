// Package editor holds the text-editing models behind the input line and
// the write-mode editor. They are plain Go with no terminal dependency, so
// every key sequence can be tested by calling methods and reading state.
package editor

import "strings"

// Buffer is a multi-line rune buffer with a cursor. Row and column are
// zero-based; the column may equal the line length, meaning "after the
// last rune". A buffer always has at least one line.
type Buffer struct {
	lines [][]rune
	row   int
	col   int
}

// NewBuffer returns a buffer holding text with the cursor at the end.
func NewBuffer(text string) *Buffer {
	b := &Buffer{}
	b.Set(text)
	return b
}

// Set replaces the content and moves the cursor to the end.
func (b *Buffer) Set(text string) {
	parts := strings.Split(text, "\n")
	b.lines = make([][]rune, len(parts))
	for i, p := range parts {
		b.lines[i] = []rune(p)
	}
	b.DocEnd()
}

// Clear empties the buffer.
func (b *Buffer) Clear() {
	b.lines = [][]rune{{}}
	b.row, b.col = 0, 0
}

// String returns the content with lines joined by newlines.
func (b *Buffer) String() string {
	var sb strings.Builder
	for i, l := range b.lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(string(l))
	}
	return sb.String()
}

// Lines returns a copy of every line as a string.
func (b *Buffer) Lines() []string {
	out := make([]string, len(b.lines))
	for i, l := range b.lines {
		out[i] = string(l)
	}
	return out
}

// Line returns the text of one row, or "" when row is out of range.
func (b *Buffer) Line(row int) string {
	if row < 0 || row >= len(b.lines) {
		return ""
	}
	return string(b.lines[row])
}

// LineCount returns the number of lines; never less than one.
func (b *Buffer) LineCount() int { return len(b.lines) }

// LineLen returns the rune length of one row, or 0 when out of range.
func (b *Buffer) LineLen(row int) int {
	if row < 0 || row >= len(b.lines) {
		return 0
	}
	return len(b.lines[row])
}

// IsEmpty reports whether the buffer holds no runes at all.
func (b *Buffer) IsEmpty() bool {
	return len(b.lines) == 1 && len(b.lines[0]) == 0
}

// Cursor returns the cursor row and column.
func (b *Buffer) Cursor() (row, col int) { return b.row, b.col }

// SetCursor moves the cursor, clamping both coordinates into range.
func (b *Buffer) SetCursor(row, col int) {
	b.row = clamp(row, 0, len(b.lines)-1)
	b.col = clamp(col, 0, len(b.lines[b.row]))
}

// Insert inserts one rune at the cursor. A newline splits the line.
func (b *Buffer) Insert(r rune) {
	if r == '\n' {
		b.Newline()
		return
	}
	line := b.lines[b.row]
	line = append(line, 0)
	copy(line[b.col+1:], line[b.col:])
	line[b.col] = r
	b.lines[b.row] = line
	b.col++
}

// InsertString inserts text at the cursor, splitting on newlines.
func (b *Buffer) InsertString(s string) {
	for _, r := range s {
		b.Insert(r)
	}
}

// Newline splits the current line at the cursor.
func (b *Buffer) Newline() {
	line := b.lines[b.row]
	head := append([]rune{}, line[:b.col]...)
	tail := append([]rune{}, line[b.col:]...)
	b.lines[b.row] = head
	b.lines = append(b.lines, nil)
	copy(b.lines[b.row+2:], b.lines[b.row+1:])
	b.lines[b.row+1] = tail
	b.row++
	b.col = 0
}

// Backspace deletes the rune before the cursor, joining lines at column
// zero. It reports whether anything changed.
func (b *Buffer) Backspace() bool {
	switch {
	case b.col > 0:
		line := b.lines[b.row]
		b.lines[b.row] = append(line[:b.col-1], line[b.col:]...)
		b.col--
		return true
	case b.row > 0:
		prev := b.lines[b.row-1]
		b.col = len(prev)
		b.lines[b.row-1] = append(prev, b.lines[b.row]...)
		b.lines = append(b.lines[:b.row], b.lines[b.row+1:]...)
		b.row--
		return true
	}
	return false
}

// Delete deletes the rune under the cursor, joining with the next line at
// end of line. It reports whether anything changed.
func (b *Buffer) Delete() bool {
	line := b.lines[b.row]
	switch {
	case b.col < len(line):
		b.lines[b.row] = append(line[:b.col], line[b.col+1:]...)
		return true
	case b.row < len(b.lines)-1:
		b.lines[b.row] = append(line, b.lines[b.row+1]...)
		b.lines = append(b.lines[:b.row+1], b.lines[b.row+2:]...)
		return true
	}
	return false
}

// Left moves the cursor one rune left, wrapping to the previous line end.
func (b *Buffer) Left() bool {
	switch {
	case b.col > 0:
		b.col--
	case b.row > 0:
		b.row--
		b.col = len(b.lines[b.row])
	default:
		return false
	}
	return true
}

// Right moves the cursor one rune right, wrapping to the next line start.
func (b *Buffer) Right() bool {
	switch {
	case b.col < len(b.lines[b.row]):
		b.col++
	case b.row < len(b.lines)-1:
		b.row++
		b.col = 0
	default:
		return false
	}
	return true
}

// Up moves the cursor one line up, clamping the column.
func (b *Buffer) Up() bool {
	if b.row == 0 {
		return false
	}
	b.row--
	b.col = clamp(b.col, 0, len(b.lines[b.row]))
	return true
}

// Down moves the cursor one line down, clamping the column.
func (b *Buffer) Down() bool {
	if b.row >= len(b.lines)-1 {
		return false
	}
	b.row++
	b.col = clamp(b.col, 0, len(b.lines[b.row]))
	return true
}

// LineStart moves the cursor to column zero.
func (b *Buffer) LineStart() { b.col = 0 }

// LineEnd moves the cursor after the last rune of the line.
func (b *Buffer) LineEnd() { b.col = len(b.lines[b.row]) }

// DocStart moves the cursor to the first rune of the buffer.
func (b *Buffer) DocStart() { b.row, b.col = 0, 0 }

// DocEnd moves the cursor after the last rune of the buffer.
func (b *Buffer) DocEnd() {
	b.row = len(b.lines) - 1
	b.col = len(b.lines[b.row])
}

// WordLeft moves to the start of the previous whitespace-delimited word on
// the current line.
func (b *Buffer) WordLeft() {
	line := b.lines[b.row]
	pos := b.col
	for pos > 0 && isSpace(line[pos-1]) {
		pos--
	}
	for pos > 0 && !isSpace(line[pos-1]) {
		pos--
	}
	b.col = pos
}

// WordRight moves to the start of the next whitespace-delimited word on
// the current line, or to the line end.
func (b *Buffer) WordRight() {
	line := b.lines[b.row]
	pos := b.col
	for pos < len(line) && !isSpace(line[pos]) {
		pos++
	}
	for pos < len(line) && isSpace(line[pos]) {
		pos++
	}
	b.col = pos
}

// DeleteWordLeft deletes from the previous word start to the cursor.
func (b *Buffer) DeleteWordLeft() bool {
	if b.col == 0 {
		return false
	}
	end := b.col
	b.WordLeft()
	line := b.lines[b.row]
	b.lines[b.row] = append(line[:b.col], line[end:]...)
	return true
}

// DeleteLine removes the current line entirely, keeping one empty line
// when the buffer would otherwise be empty.
func (b *Buffer) DeleteLine() {
	if len(b.lines) == 1 {
		b.lines[0] = b.lines[0][:0]
		b.col = 0
		return
	}
	b.lines = append(b.lines[:b.row], b.lines[b.row+1:]...)
	b.row = clamp(b.row, 0, len(b.lines)-1)
	b.col = clamp(b.col, 0, len(b.lines[b.row]))
}

// DeleteToLineEnd removes everything from the cursor to the end of line.
func (b *Buffer) DeleteToLineEnd() {
	b.lines[b.row] = b.lines[b.row][:b.col]
}

// DeleteToLineStart removes everything before the cursor on the line.
func (b *Buffer) DeleteToLineStart() {
	b.lines[b.row] = append([]rune{}, b.lines[b.row][b.col:]...)
	b.col = 0
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Snapshot is a copy of a buffer's text and cursor, for undo.
type Snapshot struct {
	lines    []string
	row, col int
}

// Snapshot captures the current content and cursor.
func (b *Buffer) Snapshot() Snapshot {
	return Snapshot{lines: b.Lines(), row: b.row, col: b.col}
}

// Restore replaces the content and cursor with a snapshot.
func (b *Buffer) Restore(s Snapshot) {
	b.lines = make([][]rune, len(s.lines))
	for i, l := range s.lines {
		b.lines[i] = []rune(l)
	}
	if len(b.lines) == 0 {
		b.lines = [][]rune{{}}
	}
	b.SetCursor(s.row, s.col)
}
