package shared

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/llm/shared/toolpayload"
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

// Images run to 5 MiB and documents to 20 MiB, far past any text cap.
// That data leaves as a native media message and is never text the
// model reads, so capping must not touch it — a trimmed base64 field
// fails to decode and takes the whole turn down with it.
func TestCapToolResultLeavesNativePayloadsIntact(t *testing.T) {
	const limit = 128 * 1024
	raw := bytes.Repeat([]byte{0x89, 0x50, 0x4e, 0x47}, 300_000) // 1.2 MB
	encoded := base64.StdEncoding.EncodeToString(raw)
	result := map[string]any{
		"success":     true,
		"path":        "diagram.png",
		"mime_type":   "image/png",
		"size_bytes":  len(raw),
		"data_base64": encoded,
	}

	got := capToolResult("viewImage", result, limit)

	if got["data_base64"] != encoded {
		t.Fatal("native payload was modified by the text cap")
	}
	payload, _, err := toolpayload.Extract(got)
	if err != nil {
		t.Fatalf("capped result no longer extracts: %v", err)
	}
	if !bytes.Equal(payload.Data, raw) {
		t.Error("payload does not round-trip after capping")
	}
}

// The exemption is a property of the field, not of a blessed tool name:
// any tool, MCP servers included, may return binary data this way.
func TestCapToolResultExemptsNativeFieldsForAnyTool(t *testing.T) {
	const limit = 8192
	encoded := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x01}, 200_000))
	result := map[string]any{
		"success":     true,
		"mime_type":   "image/png",
		"data_base64": encoded,
	}

	got := capToolResult("some_mcp_screenshot", result, limit)

	if got["data_base64"] != encoded {
		t.Error("native payload was trimmed for a tool outside the built-in set")
	}
}

// Text alongside a native payload is still capped — the exemption
// covers the binary field, not the whole result.
func TestCapToolResultStillCapsTextBesideNativePayload(t *testing.T) {
	const limit = 8192
	encoded := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x02}, 200_000))
	result := map[string]any{
		"success":     true,
		"data_base64": encoded,
		"content":     strings.Repeat("z", 500_000),
	}

	got := capToolResult("viewDocument", result, limit)

	if got["data_base64"] != encoded {
		t.Error("native payload was trimmed")
	}
	text, _ := got["content"].(string)
	if len(text) > limit {
		t.Errorf("text field is %d bytes, over the %d limit", len(text), limit)
	}
}

// Providers discard a failed call's result and build the payload from
// the error string, so that path needs the same bound.
func TestExecuteToolCallsCapsHandlerErrors(t *testing.T) {
	const limit = 8192
	handlers := map[string]ai.HandlerFunc{
		"bash": func(ctx context.Context, _ map[string]any) (map[string]any, error) {
			return nil, errors.New(strings.Repeat("stderr noise ", 100_000))
		},
	}

	results := executeToolCalls(context.Background(),
		[]ToolCall{{Name: "bash"}}, handlers, limit)

	if results[0].Err == nil {
		t.Fatal("error was swallowed")
	}
	if n := len(results[0].Err.Error()); n > limit {
		t.Fatalf("error text reached the conversation at %d bytes, over the %d limit", n, limit)
	}
	if !strings.Contains(results[0].Err.Error(), "INCOMPLETE") {
		t.Error("truncated error carries no notice")
	}
}

// The documented opt-out has to survive LoopConfig defaulting, where a
// plain 0 means "unset" and takes the default.
func TestConfiguredZeroDisablesCappingEndToEnd(t *testing.T) {
	t.Setenv("GENIE_MAX_TOOL_RESULT_BYTES", "0")

	cfg := LoopConfig{
		MaxToolResultBytes: MaxToolResultBytesFromEnv(config.NewConfigManager()),
	}.withDefaults()

	if cfg.MaxToolResultBytes != DisabledToolResultCap {
		t.Fatalf("MaxToolResultBytes = %d, want the disabled sentinel %d",
			cfg.MaxToolResultBytes, DisabledToolResultCap)
	}

	handlers := map[string]ai.HandlerFunc{
		"searchInFiles": func(ctx context.Context, _ map[string]any) (map[string]any, error) {
			return map[string]any{"results": strings.Repeat("y", 500_000)}, nil
		},
	}
	results := executeToolCalls(context.Background(),
		[]ToolCall{{Name: "searchInFiles"}}, handlers, cfg.MaxToolResultBytes)

	if len(results[0].Result["results"].(string)) != 500_000 {
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
