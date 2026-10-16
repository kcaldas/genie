package presentation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kcaldas/genie/cmd/tui/types"
)

// Keys tools use for their primary argument, tried in order for the
// one-line summary.
var primaryArgKeys = []string{"command", "path", "file_path", "pattern", "query", "url", "name"}

// ToolCallFor turns a tool execution event into the structured record the
// UI renders as a card.
func ToolCallFor(name string, params map[string]any, success bool, result map[string]any, todos *TodoFormatter) *types.ToolCall {
	call := &types.ToolCall{Name: name, Args: ToolArgs(name, params), Success: success}
	if name == "TodoWrite" && todos != nil {
		call.Summary, _ = result["message"].(string)
		call.Output = strings.TrimSpace(todos.FormatTodoToolResult(result))
		if call.Summary == "" {
			call.Summary = "Updated todos"
		}
		return call
	}
	call.Diff, _ = result["diff"].(string)
	if ok, _ := result["success"].(bool); result["success"] != nil && !ok {
		call.Success = false
	}
	if errText, _ := result["error"].(string); errText != "" && !call.Success {
		call.Summary = firstLine(errText)
		call.Output = errText
		return call
	}
	for _, key := range []string{"results", "message", "content", "output", "data"} {
		if text, _ := result[key].(string); strings.TrimSpace(text) != "" {
			call.Output = strings.TrimRight(text, "\n")
			break
		}
	}
	call.Summary = summarise(name, call.Output, result)
	return call
}

// ToolArgs summarises the arguments on one line: the primary argument
// alone when there is one, else "key: value" pairs in key order. Internal
// display parameters are skipped.
func ToolArgs(name string, params map[string]any) string {
	for _, key := range primaryArgKeys {
		if v, ok := params[key].(string); ok && v != "" {
			return truncate(firstLine(v), 80)
		}
	}
	var pairs []string
	for key, value := range params {
		if strings.HasPrefix(key, "_") {
			continue
		}
		var text string
		switch v := value.(type) {
		case string:
			text = fmt.Sprintf("%q", truncate(firstLine(v), 40))
		case nil:
			text = "null"
		default:
			text = fmt.Sprintf("%v", v)
		}
		pairs = append(pairs, key+": "+text)
	}
	sort.Strings(pairs)
	return truncate(strings.Join(pairs, ", "), 80)
}

// summarise picks the one-line outcome: the results text when it is a
// single line, else a line count, with a hint for well-known tools.
func summarise(name, output string, result map[string]any) string {
	if output == "" {
		if name == "writeFile" || name == "editFile" {
			if r, _ := result["results"].(string); r != "" {
				return firstLine(r)
			}
		}
		return "done"
	}
	lines := strings.Count(output, "\n") + 1
	if lines == 1 {
		return truncate(output, 80)
	}
	return fmt.Sprintf("%d lines", lines)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
