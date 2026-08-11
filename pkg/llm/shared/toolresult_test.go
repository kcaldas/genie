package shared

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
)

func TestCapToolResultLeavesSmallResultsUntouched(t *testing.T) {
	result := map[string]any{"success": true, "results": "one match"}

	got := capToolResult("searchInFiles", result, DefaultMaxToolResultBytes)

	if got["results"] != "one match" {
		t.Fatalf("small result was modified: %v", got["results"])
	}
}

// A 4.7MB grep result took a production agent's turn past the model's
// 1,048,576-token window: tool results bypass the context budget, so
// nothing else bounded it.
func TestCapToolResultBoundsPathologicalGrepOutput(t *testing.T) {
	const limit = 128 * 1024
	result := map[string]any{
		"success": true,
		"results": strings.Repeat("path/to/file.go:42:Hazardous materials\n", 120_000),
	}
	if serializedLen(result) < 4_000_000 {
		t.Fatalf("fixture too small: %d bytes", serializedLen(result))
	}

	got := capToolResult("searchInFiles", result, limit)

	if n := serializedLen(got); n > limit {
		t.Fatalf("capped result is %d bytes, over the %d limit", n, limit)
	}
	if got["success"] != true {
		t.Error("structural field was lost")
	}
	text, _ := got["results"].(string)
	if !strings.Contains(text, "truncated") || !strings.Contains(text, "INCOMPLETE") {
		t.Error("truncated result carries no notice telling the model it is partial")
	}
	if !strings.HasPrefix(text, "path/to/file.go:42:") {
		t.Error("kept excerpt should be the head of the real output")
	}
}

func TestCapToolResultTrimsTheLargestFieldOnly(t *testing.T) {
	const limit = 4096
	result := map[string]any{
		"error":   "",
		"summary": "12 files scanned",
		"results": strings.Repeat("x", 200_000),
	}

	got := capToolResult("searchInFiles", result, limit)

	if got["summary"] != "12 files scanned" {
		t.Errorf("small field was trimmed: %v", got["summary"])
	}
	if n := serializedLen(got); n > limit {
		t.Fatalf("capped result is %d bytes, over the %d limit", n, limit)
	}
}

// A result whose bulk is structural rather than textual cannot be
// trimmed field-by-field; passing it through would defeat the cap.
func TestCapToolResultReplacesUntrimmableResults(t *testing.T) {
	const limit = 2048
	rows := make([]any, 0, 20_000)
	for i := 0; i < 20_000; i++ {
		rows = append(rows, map[string]any{"n": i})
	}
	result := map[string]any{"success": true, "rows": rows}

	got := capToolResult("someTool", result, limit)

	if n := serializedLen(got); n > limit {
		t.Fatalf("replacement is %d bytes, over the %d limit", n, limit)
	}
	if got["success"] != false {
		t.Error("replacement should report failure so the model retries")
	}
	if !strings.Contains(got["error"].(string), "narrow the call") {
		t.Errorf("replacement gives the model no way forward: %v", got["error"])
	}
}

func TestCapToolResultKeepsUTF8Intact(t *testing.T) {
	const limit = 1024
	result := map[string]any{"results": strings.Repeat("café ☕ Hazardous\n", 5000)}

	got := capToolResult("searchInFiles", result, limit)

	payload, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("capped result no longer marshals: %v", err)
	}
	if !json.Valid(payload) {
		t.Error("capped result is not valid JSON")
	}
}

func TestCapToolResultDisabledByNonPositiveLimit(t *testing.T) {
	result := map[string]any{"results": strings.Repeat("x", 500_000)}

	got := capToolResult("searchInFiles", result, 0)

	if len(got["results"].(string)) != 500_000 {
		t.Error("a zero limit should opt out of capping")
	}
}

// The cap lives in withDefaults so a provider that never sets it — or a
// provider added later — is still bounded.
func TestLoopConfigDefaultsBoundToolResults(t *testing.T) {
	cfg := LoopConfig{}.withDefaults()

	if cfg.MaxToolResultBytes != DefaultMaxToolResultBytes {
		t.Fatalf("MaxToolResultBytes = %d, want %d", cfg.MaxToolResultBytes, DefaultMaxToolResultBytes)
	}
}

func TestExecuteToolCallsCapsHandlerOutput(t *testing.T) {
	const limit = 8192
	handlers := map[string]ai.HandlerFunc{
		"searchInFiles": func(ctx context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{"success": true, "results": strings.Repeat("y", 900_000)}, nil
		},
	}

	results := executeToolCalls(context.Background(),
		[]ToolCall{{Name: "searchInFiles"}}, handlers, limit)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if n := serializedLen(results[0].Result); n > limit {
		t.Fatalf("handler output reached the conversation at %d bytes, over the %d limit", n, limit)
	}
}
