package shared

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
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

// A failed call reaches the model as body text, so the same cap bounds
// it — no separate error path, and no predicting how an adapter will
// wrap the string.
func TestExecuteToolCallsCapsHandlerErrors(t *testing.T) {
	const limit = 8192
	handlers := map[string]ai.HandlerFunc{
		"bash": func(ctx context.Context, _ map[string]any) (map[string]any, error) {
			return nil, errors.New(strings.Repeat("stderr noise ", 100_000))
		},
	}

	results := executeToolCalls(context.Background(), nil,
		[]ToolCall{{Name: "bash"}}, handlers, ToolResultLimits{MaxBodyBytes: limit}.withDefaults())

	if n := serializedLen(results[0].Body); n > limit {
		t.Fatalf("error body reached the conversation at %d bytes, over the %d limit", n, limit)
	}
	text, _ := results[0].Body["error"].(string)
	if !strings.Contains(text, "stderr noise") || !strings.Contains(text, "INCOMPLETE") {
		t.Error("truncated error should keep its head and say it is partial")
	}
}

// The documented opt-out has to survive LoopConfig defaulting, where a
// plain 0 means "unset" and takes the default.
func TestConfiguredZeroDisablesCappingEndToEnd(t *testing.T) {
	t.Setenv("GENIE_MAX_TOOL_RESULT_BYTES", "0")

	cfg := LoopConfig{Limits: ToolResultLimitsFromEnv(config.NewConfigManager())}.withDefaults()

	if cfg.Limits.MaxBodyBytes != DisabledToolResultCap {
		t.Fatalf("MaxBodyBytes = %d, want the disabled sentinel %d",
			cfg.Limits.MaxBodyBytes, DisabledToolResultCap)
	}

	handlers := map[string]ai.HandlerFunc{
		"searchInFiles": func(ctx context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{"results": strings.Repeat("y", 500_000)}, nil
		},
	}
	cfg.Limits.MaxBatchBytes = -1
	results := executeToolCalls(context.Background(), nil,
		[]ToolCall{{Name: "searchInFiles"}}, handlers, cfg.Limits)

	if len(results[0].Body["results"].(string)) != 500_000 {
		t.Error("configured zero did not disable capping")
	}
}

// A limit below the floor could not fit the replacement message, so the
// configured bound would be violated rather than honoured.
func TestConfiguredLimitBelowFloorIsRaised(t *testing.T) {
	t.Setenv("GENIE_MAX_TOOL_RESULT_BYTES", "64")

	got := MaxToolResultBytesFromEnv(config.NewConfigManager())

	if got != MinMaxToolResultBytes {
		t.Fatalf("limit = %d, want the %d-byte floor", got, MinMaxToolResultBytes)
	}
}

// A provider building LoopConfig directly must not be able to set a
// limit too small to honour — the floor belongs to every path, not just
// the environment-derived one.
func TestLoopConfigFloorsDirectlySetLimits(t *testing.T) {
	cfg := LoopConfig{Limits: ToolResultLimits{MaxBodyBytes: 64}}.withDefaults()

	if cfg.Limits.MaxBodyBytes != MinMaxToolResultBytes {
		t.Fatalf("MaxBodyBytes = %d, want the %d-byte floor",
			cfg.Limits.MaxBodyBytes, MinMaxToolResultBytes)
	}

	handlers := map[string]ai.HandlerFunc{
		"someTool": func(ctx context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{"results": strings.Repeat("q", 500_000)}, nil
		},
	}
	results := executeToolCalls(context.Background(), nil,
		[]ToolCall{{Name: "someTool"}}, handlers, cfg.Limits)

	if n := serializedLen(results[0].Body); n > cfg.Limits.MaxBodyBytes {
		t.Fatalf("result is %d bytes, over the %d floor", n, cfg.Limits.MaxBodyBytes)
	}
}

func TestUntrimmableReplacementFitsTheFloor(t *testing.T) {
	rows := make([]any, 0, 20_000)
	for i := 0; i < 20_000; i++ {
		rows = append(rows, map[string]any{"n": i})
	}

	got := capToolResult("someTool", map[string]any{"rows": rows}, MinMaxToolResultBytes)

	if n := serializedLen(got); n > MinMaxToolResultBytes {
		t.Fatalf("replacement is %d bytes, over the %d floor", n, MinMaxToolResultBytes)
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

	if cfg.Limits.MaxBodyBytes != DefaultMaxToolResultBytes {
		t.Fatalf("MaxBodyBytes = %d, want %d", cfg.Limits.MaxBodyBytes, DefaultMaxToolResultBytes)
	}
	if cfg.Limits.MaxAttachmentBytes != DefaultMaxAttachmentBytes {
		t.Fatalf("MaxAttachmentBytes = %d, want %d", cfg.Limits.MaxAttachmentBytes, DefaultMaxAttachmentBytes)
	}
	if cfg.Limits.MaxBatchBytes != DefaultMaxBatchBytes {
		t.Fatalf("MaxBatchBytes = %d, want %d", cfg.Limits.MaxBatchBytes, DefaultMaxBatchBytes)
	}
}

func TestExecuteToolCallsCapsHandlerOutput(t *testing.T) {
	const limit = 8192
	handlers := map[string]ai.HandlerFunc{
		"searchInFiles": func(ctx context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{"success": true, "results": strings.Repeat("y", 900_000)}, nil
		},
	}

	results := executeToolCalls(context.Background(), nil,
		[]ToolCall{{Name: "searchInFiles"}}, handlers, ToolResultLimits{MaxBodyBytes: limit, MaxBatchBytes: -1}.withDefaults())

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if n := serializedLen(results[0].Body); n > limit {
		t.Fatalf("handler output reached the conversation at %d bytes, over the %d limit", n, limit)
	}
}
