package editor

import "unicode"

// Vi word motions over a Buffer, ported from the gocui editor. A "word"
// (w/b) is letters, digits and underscore; a "WORD" (W/B) is anything
// that is not whitespace.

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// ViWordForward moves to the start of the next word (w) or WORD (W).
// Past the last word on a line it jumps to the first word of the next
// line, or to the line end on the last line.
func (b *Buffer) ViWordForward(big bool) {
	line := b.lines[b.row]
	cx := b.col
	switch {
	case big:
		for cx < len(line) && !isSpace(line[cx]) {
			cx++
		}
	case cx < len(line) && isWordRune(line[cx]):
		for cx < len(line) && isWordRune(line[cx]) {
			cx++
		}
	default:
		for cx < len(line) && !isWordRune(line[cx]) && !isSpace(line[cx]) {
			cx++
		}
	}
	for cx < len(line) && isSpace(line[cx]) {
		cx++
	}
	if cx < len(line) {
		b.col = cx
		return
	}
	if b.row+1 < len(b.lines) {
		b.row++
		next := b.lines[b.row]
		b.col = 0
		for i, r := range next {
			if !isSpace(r) {
				b.col = i
				break
			}
		}
		return
	}
	b.col = len(line)
}

// ViWordBackward moves to the start of the previous word (b) or WORD (B),
// crossing line boundaries.
func (b *Buffer) ViWordBackward(big bool) {
	row, cx := b.row, b.col
	for {
		line := b.lines[row]
		if cx == 0 {
			if row == 0 {
				b.row, b.col = 0, 0
				return
			}
			row--
			cx = len(b.lines[row])
			continue
		}
		cx--
		for cx >= 0 && isSpace(line[cx]) {
			cx--
		}
		if cx < 0 {
			if row == 0 {
				b.row, b.col = 0, 0
				return
			}
			row--
			cx = len(b.lines[row])
			continue
		}
		switch {
		case big:
			for cx >= 0 && !isSpace(line[cx]) {
				cx--
			}
		case isWordRune(line[cx]):
			for cx >= 0 && isWordRune(line[cx]) {
				cx--
			}
		default:
			for cx >= 0 && !isWordRune(line[cx]) && !isSpace(line[cx]) {
				cx--
			}
		}
		b.row, b.col = row, cx+1
		return
	}
}
