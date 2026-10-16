package editor

import (
	"strings"
	"testing"

	"github.com/kcaldas/genie/cmd/history"
)

type prefixSuggester []string

func (p prefixSuggester) Suggest(input string) string {
	if input == "" {
		return ""
	}
	for _, c := range p {
		if strings.HasPrefix(c, input) {
			return c
		}
	}
	return ""
}

var testCommands = prefixSuggester{":write", ":yank", ":help"}

func newTestPrompt() *Prompt {
	return NewPrompt(testCommands, history.NewChatHistory("", false))
}

func TestPromptSuggestionOnlyOnSingleLineAtEnd(t *testing.T) {
	p := newTestPrompt()
	p.InsertString(":w")
	if got := p.Suggestion(); got != "rite" {
		t.Fatalf("ghost = %q, want %q", got, "rite")
	}
	p.Left()
	if p.Suggestion() != "" {
		t.Fatal("no ghost with the cursor mid-text")
	}
	p.End()
	p.Newline()
	if p.Suggestion() != "" {
		t.Fatal("no ghost on a multi-line prompt")
	}
	p.Set(":xyz")
	if p.Suggestion() != "" {
		t.Fatal("no ghost for an unknown command")
	}
}

func TestPromptTabAndRightAcceptSuggestion(t *testing.T) {
	p := newTestPrompt()
	p.InsertString(":y")
	if !p.Complete() || p.Text() != ":yank" {
		t.Fatalf("tab: %q", p.Text())
	}
	p.Clear()
	p.InsertString(":h")
	if !p.Right() || p.Text() != ":help" {
		t.Fatalf("right at end: %q", p.Text())
	}
	p.Home()
	if p.Right() {
		t.Fatal("right mid-text moves instead of accepting")
	}
}

func TestPromptNewlinesAndEditing(t *testing.T) {
	p := newTestPrompt()
	p.InsertString("héllo")
	p.Newline()
	p.InsertString("wörld")
	if p.Text() != "héllo\nwörld" || p.Lines() != 2 {
		t.Fatalf("text = %q lines = %d", p.Text(), p.Lines())
	}
	if p.OnFirstLine() || !p.OnLastLine() {
		t.Fatal("cursor should be on the last line")
	}
	p.Up()
	if !p.OnFirstLine() {
		t.Fatal("up moves to the first line")
	}
	p.Home()
	p.Delete()
	p.End()
	p.DeleteWordLeft()
	if p.Text() != "\nwörld" {
		t.Fatalf("after edits: %q", p.Text())
	}
	p.Insert('\r')
	if p.Lines() != 3 {
		t.Fatal("carriage return starts a line too")
	}
}

func TestPromptHistoryKeepsMultilineEntries(t *testing.T) {
	h := history.NewChatHistory("", false)
	h.AddCommand("first")
	h.AddCommand("multi\nline")
	p := NewPrompt(nil, h)
	p.InsertString("draft")
	p.HistoryPrev()
	if p.Text() != "multi\nline" || p.Lines() != 2 {
		t.Fatalf("prev = %q", p.Text())
	}
	p.HistoryPrev()
	if p.Text() != "first" {
		t.Fatalf("prev again = %q", p.Text())
	}
	p.HistoryNext()
	p.HistoryNext()
	if p.Text() != "draft" {
		t.Fatalf("restoring the draft: %q", p.Text())
	}
	p.HistoryPrev()
	p.Insert('!')
	if p.navigating {
		t.Fatal("typing leaves history mode")
	}
}

func TestPromptSubmitExpandsChipsAndRecordsHistory(t *testing.T) {
	h := history.NewChatHistory("", false)
	p := NewPrompt(nil, h)
	if p.Submit() != "" {
		t.Fatal("blank submit")
	}
	p.InsertString("see: ")
	paste := "line1\nline2\nline3\nline4"
	if !p.Paste(paste) {
		t.Fatal("a four-line paste must become a chip")
	}
	if p.Raw() == p.Text() || !strings.Contains(p.Text(), paste) {
		t.Fatalf("raw=%q text=%q", p.Raw(), p.Text())
	}
	if labels := p.ChipLabels(); len(labels) != 1 || labels[0] != "[Pasted 4 lines]" {
		t.Fatalf("labels = %v", labels)
	}
	got := p.Submit()
	if got != "see: "+paste || !p.IsEmpty() {
		t.Fatalf("submit = %q empty=%v", got, p.IsEmpty())
	}
	if hist := h.GetHistory(); len(hist) != 1 || hist[0] != got {
		t.Fatalf("history = %v", hist)
	}
}

func TestPromptSmallPastesStayInline(t *testing.T) {
	p := newTestPrompt()
	if p.Paste("two\nlines") {
		t.Fatal("two lines stay inline")
	}
	if p.Text() != "two\nlines" {
		t.Fatalf("inline paste = %q", p.Text())
	}
	p.Clear()
	if !p.Paste(strings.Repeat("x", ChipMinLength)) {
		t.Fatal("a very long single line becomes a chip")
	}
	if labels := p.ChipLabels(); labels[0] != "[Pasted 400 chars]" {
		t.Fatalf("label = %q", labels[0])
	}
}

func TestPromptChipIsOneCharacterToTheCursor(t *testing.T) {
	p := newTestPrompt()
	p.InsertString("a")
	p.Paste("1\n2\n3\n")
	p.InsertString("b")
	p.Left()
	p.Left()
	_, col := p.Cursor()
	if col != 1 {
		t.Fatalf("cursor col = %d, want 1 (before the chip)", col)
	}
	p.Delete()
	if p.Text() != "ab" || len(p.ChipLabels()) != 0 {
		t.Fatalf("deleting the chip rune drops the paste: %q", p.Text())
	}
	p.Paste("1\n2\n3\n")
	p.Backspace()
	if p.Text() != "ab" {
		t.Fatalf("backspace over a chip: %q", p.Text())
	}
}

func TestPromptLayoutWrapsAndScrolls(t *testing.T) {
	p := newTestPrompt()
	p.InsertString("0123456789abcdef") // 16 runes, width 10 -> 2 rows
	w := p.Layout(10, 4)
	if len(w.Rows) != 2 || w.Rows[0] != "0123456789" || w.Rows[1] != "abcdef" || w.CursorRow != 1 || w.CursorCol != 6 {
		t.Fatalf("layout = %+v", w)
	}
	for i := 0; i < 6; i++ {
		p.Newline()
	}
	w = p.Layout(10, 4)
	if w.Total != 8 || len(w.Rows) != 4 || w.CursorRow != 3 || w.CursorCol != 0 {
		t.Fatalf("scrolled layout = %+v", w)
	}
	for i := 0; i < 6; i++ {
		p.Up()
	}
	w = p.Layout(10, 4)
	if w.CursorRow != 0 || w.CursorCol != 0 || w.Rows[0] != "0123456789" {
		t.Fatalf("scroll back up = %+v", w)
	}
	if w := p.Layout(0, 4); len(w.Rows) != 0 {
		t.Fatal("zero width lays out nothing")
	}
}

func TestPromptLayoutShowsChipLabelAndGhost(t *testing.T) {
	p := newTestPrompt()
	p.Paste("a\nb\nc\nd")
	p.InsertString("x")
	w := p.Layout(40, 4)
	if w.Rows[0] != "[Pasted 4 lines]x" || w.CursorCol != len("[Pasted 4 lines]x") {
		t.Fatalf("chip layout = %+v", w)
	}
	q := newTestPrompt()
	q.InsertString(":w")
	w = q.Layout(4, 1)
	if w.Ghost != "ri" || w.CursorCol != 2 {
		t.Fatalf("ghost truncated to room: %+v", w)
	}
}

func TestPromptNilDependenciesAreSafe(t *testing.T) {
	p := NewPrompt(nil, nil)
	p.InsertString("x")
	p.HistoryPrev()
	p.HistoryNext()
	if p.Suggestion() != "" || p.Submit() != "x" {
		t.Fatal("prompt must work without suggester or history")
	}
}

func TestPromptUndoGroupsTypingAndRedoes(t *testing.T) {
	p := newTestPrompt()
	p.InsertString("hello")
	p.Newline()
	p.InsertString("world")
	if !p.Undo() || p.Text() != "hello\n" {
		t.Fatalf("undo the typed word: %q", p.Text())
	}
	if !p.Undo() || p.Text() != "hello" {
		t.Fatalf("undo the newline: %q", p.Text())
	}
	if !p.Undo() || p.Text() != "" {
		t.Fatalf("undo the first word: %q", p.Text())
	}
	if p.Undo() {
		t.Fatal("nothing left to undo")
	}
	if !p.Redo() || p.Text() != "hello" || !p.Redo() || !p.Redo() || p.Text() != "hello\nworld" {
		t.Fatalf("redo all: %q", p.Text())
	}
	if p.Redo() {
		t.Fatal("nothing left to redo")
	}
	p.Undo()
	p.InsertString("!")
	if p.Redo() {
		t.Fatal("a new edit clears the redo stack")
	}
}

func TestPromptCursorMovesSplitUndoGroups(t *testing.T) {
	p := newTestPrompt()
	p.InsertString("ab")
	p.Left()
	p.InsertString("X")
	if p.Text() != "aXb" {
		t.Fatalf("text = %q", p.Text())
	}
	p.Undo()
	if p.Text() != "ab" {
		t.Fatalf("undo after a cursor move reverts only the later typing: %q", p.Text())
	}
	p.End()
	p.Backspace()
	p.Backspace()
	if p.Text() != "" || !p.Undo() || p.Text() != "ab" {
		t.Fatalf("a run of backspaces is one step: %q", p.Text())
	}
	p.Paste("1\n2\n3\n4")
	p.Undo()
	if len(p.ChipLabels()) != 0 || p.Text() != "ab" {
		t.Fatal("a paste is its own step")
	}
	p.Submit()
	if p.Undo() {
		t.Fatal("submit forgets the undo history")
	}
}
