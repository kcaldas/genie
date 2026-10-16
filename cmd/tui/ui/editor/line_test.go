package editor

import (
	"testing"

	"github.com/kcaldas/genie/cmd/history"
)

// prefixSuggester completes against a fixed command list, like the shell
// completer does for ':' commands.
type prefixSuggester []string

func (p prefixSuggester) Suggest(input string) string {
	if input == "" {
		return ""
	}
	for _, c := range p {
		if len(c) >= len(input) && c[:len(input)] == input {
			return c
		}
	}
	return ""
}

var testCommands = prefixSuggester{":write", ":yank", ":help"}

func newTestLine() *Line {
	return NewLine(testCommands, history.NewChatHistory("", false))
}

func TestLineSuggestionOnlyAtEnd(t *testing.T) {
	l := newTestLine()
	l.InsertString(":w")
	if got := l.Suggestion(); got != "rite" {
		t.Fatalf("ghost = %q, want %q", got, "rite")
	}
	l.Left()
	if got := l.Suggestion(); got != "" {
		t.Fatalf("ghost with cursor mid-text = %q, want empty", got)
	}
	l.End()
	l.InsertString("rite")
	if got := l.Suggestion(); got != "" {
		t.Fatalf("ghost for complete command = %q, want empty", got)
	}
	l.Set(":xyz")
	if got := l.Suggestion(); got != "" {
		t.Fatalf("ghost for unknown command = %q, want empty", got)
	}
}

func TestLineTabAndRightAcceptSuggestion(t *testing.T) {
	l := newTestLine()
	l.InsertString(":y")
	if !l.Complete() || l.Text() != ":yank" || l.Cursor() != 5 {
		t.Fatalf("tab: text=%q cursor=%d", l.Text(), l.Cursor())
	}
	l.Clear()
	l.InsertString(":h")
	if !l.Right() || l.Text() != ":help" {
		t.Fatalf("right at end: text=%q", l.Text())
	}
	l.Home()
	if l.Right() || l.Cursor() != 1 {
		t.Fatalf("right mid-text should move: cursor=%d", l.Cursor())
	}
}

func TestLineEditing(t *testing.T) {
	l := newTestLine()
	l.InsertString("héllo wörld")
	l.Backspace()
	l.Backspace()
	if l.Text() != "héllo wör" {
		t.Fatalf("backspace: %q", l.Text())
	}
	l.Home()
	l.Delete()
	if l.Text() != "éllo wör" || l.Cursor() != 0 {
		t.Fatalf("delete at home: %q cursor %d", l.Text(), l.Cursor())
	}
	l.End()
	l.DeleteWordLeft()
	if l.Text() != "éllo " {
		t.Fatalf("delete word: %q", l.Text())
	}
	l.WordLeft()
	if l.Cursor() != 0 {
		t.Fatalf("word left cursor %d", l.Cursor())
	}
	l.WordRight()
	if l.Cursor() != 5 {
		t.Fatalf("word right cursor %d", l.Cursor())
	}
	l.Insert('\n')
	if l.Text() != "éllo  " {
		t.Fatalf("newline typed into a line becomes a space: %q", l.Text())
	}
}

func TestLineHistoryNavigationRestoresDraft(t *testing.T) {
	h := history.NewChatHistory("", false)
	h.AddCommand("first")
	h.AddCommand("multi\nline\n  entry")
	l := NewLine(nil, h)
	l.InsertString("draft")

	l.HistoryPrev()
	if l.Text() != "multi line entry" || l.Cursor() != 16 {
		t.Fatalf("prev flattens: %q cursor %d", l.Text(), l.Cursor())
	}
	l.HistoryPrev()
	if l.Text() != "first" {
		t.Fatalf("prev again: %q", l.Text())
	}
	l.HistoryNext()
	if l.Text() != "multi line entry" {
		t.Fatalf("next: %q", l.Text())
	}
	l.HistoryNext()
	if l.Text() != "draft" {
		t.Fatalf("next past newest restores draft: %q", l.Text())
	}
}

func TestLineTypingLeavesHistoryMode(t *testing.T) {
	h := history.NewChatHistory("", false)
	h.AddCommand("old")
	l := NewLine(nil, h)
	l.HistoryPrev()
	l.Insert('!')
	if l.navigating {
		t.Fatal("typing should leave history navigation")
	}
	l.HistoryPrev()
	if l.Text() != "old" {
		t.Fatalf("history restarts from the newest entry: %q", l.Text())
	}
}

func TestLineSubmit(t *testing.T) {
	h := history.NewChatHistory("", false)
	l := NewLine(nil, h)
	if got := l.Submit(); got != "" {
		t.Fatalf("blank submit = %q", got)
	}
	l.InsertString("   ")
	if got := l.Submit(); got != "" || l.Text() != "   " {
		t.Fatalf("whitespace-only submit = %q, text %q", got, l.Text())
	}
	l.Set("  say hi  ")
	if got := l.Submit(); got != "say hi" {
		t.Fatalf("submit = %q", got)
	}
	if !l.IsEmpty() {
		t.Fatalf("line not cleared: %q", l.Text())
	}
	if got := h.GetHistory(); len(got) != 1 || got[0] != "say hi" {
		t.Fatalf("history = %v", got)
	}
}

func TestLineWindowScrollsToKeepCursorVisible(t *testing.T) {
	l := newTestLine()
	l.InsertString("0123456789abcdef")
	vis, cx, ghost := l.Window(10)
	if vis != "89abcdef" || cx != 8 || ghost != "" {
		t.Fatalf("window at end: vis=%q cx=%d ghost=%q", vis, cx, ghost)
	}
	l.Home()
	vis, cx, _ = l.Window(10)
	if vis != "0123456789" || cx != 0 {
		t.Fatalf("window at home: vis=%q cx=%d", vis, cx)
	}
	l.Clear()
	l.InsertString(":w")
	vis, cx, ghost = l.Window(4)
	if vis != ":w" || cx != 2 || ghost != "ri" {
		t.Fatalf("ghost truncated to room: vis=%q cx=%d ghost=%q", vis, cx, ghost)
	}
	if v, c, g := l.Window(0); v != "" || c != 0 || g != "" {
		t.Fatal("zero width window must be empty")
	}
}

func TestLineNilDependenciesAreSafe(t *testing.T) {
	l := NewLine(nil, nil)
	l.InsertString("x")
	l.HistoryPrev()
	l.HistoryNext()
	if l.Suggestion() != "" || l.Submit() != "x" {
		t.Fatal("line must work without suggester or history")
	}
}
