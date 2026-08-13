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

func TestTransientAllowanceUsesLatestPhysicalUsage(t *testing.T) {
	limits := ToolResultLimits{MaxTextBytes: -1, MaxBatchTextBytes: -1}
	admitted := limits.withTransientAllowance(1_000_000, &ai.TokenCount{
		InputTokens:  100_000,
		OutputTokens: 10_000,
	})
	assert.True(t, admitted.hasTransientTextAllowance)
	assert.Equal(t, 2_670_000, admitted.transientTextAllowance)
}

func TestTransientAllowanceIncludesReasoningReportedOnlyInTotal(t *testing.T) {
	limits := ToolResultLimits{MaxTextBytes: -1, MaxBatchTextBytes: -1}
	admitted := limits.withTransientAllowance(1_000, &ai.TokenCount{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  400,
	})
	assert.True(t, admitted.hasTransientTextAllowance)
	assert.Equal(t, 1_800, admitted.transientTextAllowance)
}

func TestTransientAllowanceDoesNotReplaceStricterExplicitBatchGuard(t *testing.T) {
	limits := ToolResultLimits{MaxTextBytes: -1, MaxBatchTextBytes: 64_000}
	admitted := limits.withTransientAllowance(1_000_000, &ai.TokenCount{InputTokens: 100_000})
	assert.Equal(t, 64_000, admitted.MaxBatchTextBytes)
	assert.Equal(t, 2_700_000, admitted.transientTextAllowance)
	budget := newBatchBudget(admitted)
	assert.Equal(t, 64_000, budget.textRemaining)
}

func TestExhaustedTransientAllowanceSurvivesDefaulting(t *testing.T) {
	limits := ToolResultLimits{MaxTextBytes: -1, MaxBatchTextBytes: -1}
	limits = limits.withTransientAllowance(100, &ai.TokenCount{TotalTokens: 100})
	limits = limits.withDefaults()

	assert.True(t, limits.hasTransientTextAllowance)
	assert.Zero(t, limits.transientTextAllowance)
	budget := newBatchBudget(limits)
	assert.False(t, budget.textUnlimited)
	assert.Less(t, budget.textRemaining, DefaultMaxToolTextBytes)
}

func TestDirectTextLimitUsesFloor(t *testing.T) {
	limits := (LoopConfig{Limits: ToolResultLimits{MaxTextBytes: 64}}).withDefaults().Limits
	assert.Equal(t, MinMaxToolTextBytes, limits.MaxTextBytes)
}

func TestConfiguredZeroDisablesOnlyItsOwnCap(t *testing.T) {
	t.Setenv("GENIE_MAX_TOOL_RESULT_BYTES", "0")
	t.Setenv("GENIE_MAX_TOOL_BATCH_BYTES", "4096")
	t.Setenv("GENIE_MAX_ATTACHMENT_BYTES", "0")
	limits := ToolResultLimitsFromEnv(config.NewConfigManager())
	assert.Equal(t, DisabledToolTextCap, limits.MaxTextBytes)
	assert.Equal(t, 4096, limits.MaxBatchTextBytes)
	assert.Equal(t, DisabledToolTextCap, limits.MaxBlobBytes)
}

func TestConfiguredBatchLimitBelowFloorIsRaised(t *testing.T) {
	t.Setenv("GENIE_MAX_TOOL_BATCH_BYTES", "1")
	limits := ToolResultLimitsFromEnv(config.NewConfigManager())
	assert.Equal(t, MinMaxToolTextBytes, limits.MaxBatchTextBytes)
}
