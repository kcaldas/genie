package shared

import (
	"context"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestConfiguredZeroDisablesTextCappingEndToEnd(t *testing.T) {
	t.Setenv("GENIE_MAX_TOOL_RESULT_BYTES", "0")
	cfg := LoopConfig{Limits: ToolResultLimitsFromEnv(config.NewConfigManager())}.withDefaults()
	assert.Equal(t, DisabledToolTextCap, cfg.Limits.MaxTextBytes)

	handler := func(context.Context, map[string]any) (ai.ToolOutput, error) {
		return ai.ContentToolOutput(nil, ai.TextContent{Text: strings.Repeat("y", 500_000)}), nil
	}
	cfg.Limits.MaxBatchTextBytes = -1
	results := executeToolCalls(context.Background(), nil, []ToolCall{{Name: "search"}}, map[string]ai.HandlerFunc{"search": handler}, cfg.Limits)
	assert.Len(t, outputText(t, results[0]), 500_000)
}

func TestConfiguredLimitBelowFloorIsRaised(t *testing.T) {
	t.Setenv("GENIE_MAX_TOOL_RESULT_BYTES", "64")
	assert.Equal(t, MinMaxToolTextBytes, MaxToolTextBytesFromEnv(config.NewConfigManager()))
}

func TestLoopConfigDefaultsAllToolResultLimits(t *testing.T) {
	limits := (LoopConfig{}).withDefaults().Limits
	assert.Equal(t, DefaultMaxToolTextBytes, limits.MaxTextBytes)
	assert.Equal(t, DefaultMaxBatchTextBytes, limits.MaxBatchTextBytes)
	assert.Equal(t, DefaultMaxToolBlobBytes, limits.MaxBlobBytes)
}

func TestConfiguredZeroDisablesEachToolResultCap(t *testing.T) {
	t.Setenv("GENIE_MAX_TOOL_RESULT_BYTES", "0")
	t.Setenv("GENIE_MAX_TOOL_BATCH_BYTES", "0")
	t.Setenv("GENIE_MAX_ATTACHMENT_BYTES", "0")
	limits := ToolResultLimitsFromEnv(config.NewConfigManager())
	assert.Equal(t, -1, limits.MaxTextBytes)
	assert.Equal(t, -1, limits.MaxBatchTextBytes)
	assert.Equal(t, -1, limits.MaxBlobBytes)
}

func TestDirectTextLimitUsesFloor(t *testing.T) {
	limits := (LoopConfig{Limits: ToolResultLimits{MaxTextBytes: 64}}).withDefaults().Limits
	assert.Equal(t, MinMaxToolTextBytes, limits.MaxTextBytes)
}
