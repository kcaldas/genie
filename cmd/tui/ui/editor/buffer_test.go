package editor

import (
	"strings"
	"testing"
)

func cursorIs(t *testing.T, b *Buffer, row, col int) {
	t.Helper()
	r, c := b.Cursor()
	if r != row || c != col {
		t.Fatalf("cursor = (%d,%d), want (%d,%d); text %q", r, c, row, col, b.String())
	}
}

func textIs(t *testing.T, b *Buffer, want string) {
	t.Helper()
	if got := b.String(); got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestBufferStartsEmptyWithOneLine(t *testing.T) {
	b := NewBuffer("")
	if !b.IsEmpty() || b.LineCount() != 1 {
		t.Fatalf("empty buffer: IsEmpty=%v lines=%d", b.IsEmpty(), b.LineCount())
	}
	cursorIs(t, b, 0, 0)
}

func TestBufferInsertAndNewlineSplitLines(t *testing.T) {
	b := NewBuffer("")
	b.InsertString("héllo wörld")
	cursorIs(t, b, 0, 11)
	b.SetCursor(0, 5)
	b.Newline()
	textIs(t, b, "héllo\n wörld")
	cursorIs(t, b, 1, 0)
	b.InsertString("x\ny")
	textIs(t, b, "héllo\nx\ny wörld")
	cursorIs(t, b, 2, 1)
}

func TestBufferBackspaceJoinsLines(t *testing.T) {
	b := NewBuffer("ab\ncd")
	b.SetCursor(1, 0)
	if !b.Backspace() {
		t.Fatal("backspace at line start should join")
	}
	textIs(t, b, "abcd")
	cursorIs(t, b, 0, 2)
	b.DocStart()
	if b.Backspace() {
		t.Fatal("backspace at document start should do nothing")
	}
}

func TestBufferDeleteJoinsLines(t *testing.T) {
	b := NewBuffer("ab\ncd")
	b.SetCursor(0, 2)
	if !b.Delete() {
		t.Fatal("delete at line end should join")
	}
	textIs(t, b, "abcd")
	cursorIs(t, b, 0, 2)
	b.DocEnd()
	if b.Delete() {
		t.Fatal("delete at document end should do nothing")
	}
}

func TestBufferArrowsWrapAcrossLines(t *testing.T) {
	b := NewBuffer("ab\ncde")
	b.SetCursor(0, 2)
	b.Right()
	cursorIs(t, b, 1, 0)
	b.Left()
	cursorIs(t, b, 0, 2)
	b.SetCursor(1, 3)
	b.Up()
	cursorIs(t, b, 0, 2) // clamped to the shorter line
	b.Down()
	cursorIs(t, b, 1, 2)
	if b.Down() {
		t.Fatal("down on last line should report false")
	}
}

func TestBufferWordMotion(t *testing.T) {
	b := NewBuffer("one  two three")
	b.LineEnd()
	b.WordLeft()
	cursorIs(t, b, 0, 9)
	b.WordLeft()
	cursorIs(t, b, 0, 5)
	b.WordLeft()
	cursorIs(t, b, 0, 0)
	b.WordRight()
	cursorIs(t, b, 0, 5)
	b.WordRight()
	cursorIs(t, b, 0, 9)
	b.WordRight()
	cursorIs(t, b, 0, 14)
}

func TestBufferDeleteWordLeft(t *testing.T) {
	b := NewBuffer("hello big world")
	b.LineEnd()
	b.DeleteWordLeft()
	textIs(t, b, "hello big ")
	b.SetCursor(0, 6)
	b.DeleteWordLeft()
	textIs(t, b, "big ")
	cursorIs(t, b, 0, 0)
	if b.DeleteWordLeft() {
		t.Fatal("nothing to delete at column 0")
	}
}

func TestBufferLineDeletes(t *testing.T) {
	b := NewBuffer("first\nsecond\nthird")
	b.SetCursor(1, 3)
	b.DeleteToLineEnd()
	textIs(t, b, "first\nsec\nthird")
	b.DeleteToLineStart()
	textIs(t, b, "first\n\nthird")
	cursorIs(t, b, 1, 0)
	b.DeleteLine()
	textIs(t, b, "first\nthird")
	cursorIs(t, b, 1, 0)
	b.DeleteLine()
	b.DeleteLine()
	textIs(t, b, "")
	if b.LineCount() != 1 {
		t.Fatalf("buffer must keep one line, has %d", b.LineCount())
	}
}

func TestBufferSetCursorClamps(t *testing.T) {
	b := NewBuffer("abc\nde")
	b.SetCursor(9, 9)
	cursorIs(t, b, 1, 2)
	b.SetCursor(-1, -1)
	cursorIs(t, b, 0, 0)
}

func TestBufferLinesAreCopies(t *testing.T) {
	b := NewBuffer("abc")
	lines := b.Lines()
	lines[0] = "zzz"
	if b.Line(0) != "abc" {
		t.Fatal("Lines must not alias the buffer")
	}
	if b.Line(5) != "" || b.LineLen(5) != 0 {
		t.Fatal("out-of-range rows read as empty")
	}
}

func TestBufferLargePasteKeepsStructure(t *testing.T) {
	text := strings.Repeat("line of text\n", 500) + "end"
	b := NewBuffer(text)
	if b.LineCount() != 501 {
		t.Fatalf("lines = %d", b.LineCount())
	}
	textIs(t, b, text)
}
