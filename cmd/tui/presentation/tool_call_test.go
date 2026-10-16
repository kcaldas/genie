package presentation

import (
	"testing"

	"github.com/kcaldas/genie/cmd/tui/types"
)

func TestToolCallForBash(t *testing.T) {
	call := ToolCallFor("bash", map[string]any{"command": "ls -la\nsecond", "_display_message": "Listing"}, true,
		map[string]any{"success": true, "results": "a.go\nb.go\nc.go"}, nil)
	if call.Args != "ls -la" || call.Summary != "3 lines" || call.Output != "a.go\nb.go\nc.go" || !call.Success {
		t.Fatalf("bash card = %+v", call)
	}
}

func TestToolCallForFailure(t *testing.T) {
	call := ToolCallFor("readFile", map[string]any{"path": "missing.go"}, true,
		map[string]any{"success": false, "results": "", "error": "open missing.go: no such file\nmore"}, nil)
	if call.Success || call.Summary != "open missing.go: no such file" || call.Args != "missing.go" {
		t.Fatalf("failure card = %+v", call)
	}
}

func TestToolCallForWriteFileKeepsDiff(t *testing.T) {
	call := ToolCallFor("writeFile", map[string]any{"path": "x.go", "content": "..."}, true,
		map[string]any{"success": true, "results": "Wrote 3 lines to x.go", "diff": "--- a\n+++ b\n+x"}, nil)
	if call.Diff == "" || call.Summary != "Wrote 3 lines to x.go" {
		t.Fatalf("writeFile card = %+v", call)
	}
}

func TestToolCallForTodoWriteUsesTheTodoFormatter(t *testing.T) {
	tf := NewTodoFormatter(&types.Theme{})
	call := ToolCallFor("TodoWrite", map[string]any{}, true, map[string]any{
		"success": true, "message": "Updated 2 todos",
		"todos": []any{map[string]any{"id": "1", "content": "write tests", "status": "pending", "priority": "high"}},
	}, tf)
	if call.Summary != "Updated 2 todos" || call.Output == "" {
		t.Fatalf("todo card = %+v", call)
	}
}

func TestToolArgsFallsBackToSortedPairs(t *testing.T) {
	got := ToolArgs("custom", map[string]any{"zeta": 1, "alpha": "long value here", "_display_message": "hidden"})
	if got != `alpha: "long value here", zeta: 1` {
		t.Fatalf("args = %q", got)
	}
	if got := ToolArgs("empty", nil); got != "" {
		t.Fatalf("no args = %q", got)
	}
}

func TestParseTodosAcceptsJSONShapes(t *testing.T) {
	items, err := ParseTodos([]any{map[string]any{"id": "1", "content": "a", "status": "pending", "priority": "high"}, "junk"})
	if err != nil || len(items) != 1 || items[0].Content != "a" {
		t.Fatalf("items=%v err=%v", items, err)
	}
	if _, err := ParseTodos("nope"); err == nil {
		t.Fatal("unknown shape must error")
	}
}
