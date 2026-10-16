package editor

import "unicode"

// Vi word motions over a Buffer. A "word" (w/b) is a run of letters,
// digits and underscores or a run of other non-space runes; a "WORD"
// (W/B) is any run of non-space runes.

// runeClass groups runes for word motions.
type runeClass int

const (
	classSpace runeClass = iota
	classWord
	classPunct
)

func classOf(r rune, big bool) runeClass {
	switch {
	case isSpace(r):
		return classSpace
	case big || unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
		return classWord
	default:
		return classPunct
	}
}

// ViWordForward moves to the start of the next word (w) or WORD (W):
// skip the run under the cursor, then any spaces. Past the last word on a
// line it jumps to the first word of the next line, or to the line end on
// the last line.
func (b *Buffer) ViWordForward(big bool) {
	line := b.lines[b.row]
	cx := b.col
	if cx < len(line) && classOf(line[cx], big) != classSpace {
		cx = skipClass(line, cx, classOf(line[cx], big), big)
	}
	cx = skipClass(line, cx, classSpace, big)
	if cx < len(line) {
		b.col = cx
		return
	}
	if b.row+1 < len(b.lines) {
		b.row++
		b.col = skipClass(b.lines[b.row], 0, classSpace, big)
		if b.col >= len(b.lines[b.row]) {
			b.col = 0
		}
		return
	}
	b.col = len(line)
}

// skipClass advances cx over runes of class c and returns the new index.
func skipClass(line []rune, cx int, c runeClass, big bool) int {
	for cx < len(line) && classOf(line[cx], big) == c {
		cx++
	}
	return cx
}

// ViWordBackward moves to the start of the previous word (b) or WORD (B),
// crossing line boundaries: step back over spaces (and line ends), then
// back over the run that precedes the cursor.
func (b *Buffer) ViWordBackward(big bool) {
	row, cx := b.row, b.col
	for {
		if cx == 0 {
			if row == 0 {
				b.row, b.col = 0, 0
				return
			}
			row--
			cx = len(b.lines[row])
			continue
		}
		line := b.lines[row]
		cx--
		for cx >= 0 && isSpace(line[cx]) {
			cx--
		}
		if cx < 0 {
			cx = 0 // line was all spaces: keep walking up
			continue
		}
		c := classOf(line[cx], big)
		for cx >= 0 && classOf(line[cx], big) == c {
			cx--
		}
		b.row, b.col = row, cx+1
		return
	}
}
