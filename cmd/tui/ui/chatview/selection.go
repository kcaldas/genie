package chatview

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// pos is a point in the transcript: an absolute rendered line and a cell
// column within it.
type pos struct{ line, col int }

func (p pos) before(q pos) bool { return p.line < q.line || (p.line == q.line && p.col < q.col) }

// selection is a stream selection from anchor to head (either order).
type selection struct {
	anchor, head pos
	active       bool
}

func (s selection) bounds() (from, to pos) {
	if s.head.before(s.anchor) {
		return s.head, s.anchor
	}
	return s.anchor, s.head
}

// HasSelection reports whether text is selected.
func (m *Model) HasSelection() bool { return m.sel.active }

// ClearSelection drops the selection.
func (m *Model) ClearSelection() { m.sel = selection{} }

// SelectStart anchors a selection at visible row y, cell x.
func (m *Model) SelectStart(y, x int) {
	p := m.at(y, x)
	m.sel = selection{anchor: p, head: p, active: true}
}

// SelectExtend moves the selection head to visible row y, cell x.
func (m *Model) SelectExtend(y, x int) {
	if !m.sel.active {
		m.SelectStart(y, x)
		return
	}
	m.sel.head = m.at(y, x)
}

// SelectWord selects the word under visible row y, cell x.
func (m *Model) SelectWord(y, x int) {
	p := m.at(y, x)
	plain := []rune(ansi.Strip(m.line(p.line)))
	col := min(cellToRune(plain, p.col), max(len(plain)-1, 0))
	if len(plain) == 0 || unicode.IsSpace(plain[col]) {
		m.sel = selection{anchor: p, head: p, active: true}
		return
	}
	start, end := col, col
	for start > 0 && !unicode.IsSpace(plain[start-1]) {
		start--
	}
	for end < len(plain) && !unicode.IsSpace(plain[end]) {
		end++
	}
	m.sel = selection{anchor: pos{p.line, runeToCell(plain, start)}, head: pos{p.line, runeToCell(plain, end)}, active: true}
}

// SelectLine selects the whole visible row y.
func (m *Model) SelectLine(y int) {
	p := m.at(y, 0)
	m.sel = selection{anchor: p, head: pos{p.line, ansi.StringWidth(m.line(p.line))}, active: true}
}

// SelectedText returns the selected text, plain, lines joined by newlines
// and trailing spaces trimmed. A selection with no extent yields "".
func (m *Model) SelectedText() string {
	if !m.sel.active {
		return ""
	}
	from, to := m.sel.bounds()
	if from == to {
		return ""
	}
	var out []string
	for ln := from.line; ln <= to.line; ln++ {
		plain := ansi.Strip(m.line(ln))
		left, right := 0, ansi.StringWidth(plain)
		if ln == from.line {
			left = from.col
		}
		if ln == to.line {
			right = min(to.col, right)
		}
		if left >= right {
			out = append(out, "")
			continue
		}
		out = append(out, strings.TrimRight(ansi.Cut(plain, left, right), " "))
	}
	return strings.Join(out, "\n")
}

// at maps a visible row and cell to a transcript position, clamped.
func (m *Model) at(y, x int) pos {
	line := min(max(m.offset+y, 0), max(m.total-1, 0))
	return pos{line: line, col: max(x, 0)}
}

// line returns the rendered line at an absolute index, "" when out of range.
func (m *Model) line(n int) string {
	for _, it := range m.items {
		if n < len(it.lines) {
			return it.lines[n]
		}
		n -= len(it.lines)
	}
	return ""
}

// highlight reverses the selected span of the absolute line n, if any.
func (m *Model) highlight(n int, line string) string {
	if !m.sel.active {
		return line
	}
	from, to := m.sel.bounds()
	if n < from.line || n > to.line || from == to {
		return line
	}
	width := ansi.StringWidth(line)
	left, right := 0, width
	if n == from.line {
		left = min(from.col, width)
	}
	if n == to.line {
		right = min(to.col, width)
	}
	if left >= right {
		return line
	}
	mid := ansi.Strip(ansi.Cut(line, left, right))
	return ansi.Truncate(line, left, "") + "\x1b[7m" + mid + "\x1b[27m" + ansi.TruncateLeft(line, right, "")
}

func cellToRune(runes []rune, cell int) int {
	w := 0
	for i, r := range runes {
		rw := ansi.StringWidth(string(r))
		if w+rw > cell {
			return i
		}
		w += rw
	}
	return len(runes)
}

func runeToCell(runes []rune, idx int) int {
	w := 0
	for i, r := range runes {
		if i >= idx {
			break
		}
		w += ansi.StringWidth(string(r))
	}
	return w
}
