package ctx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookupContextWindow_ExactMatch(t *testing.T) {
	assert.Equal(t, 200000, LookupContextWindow("claude-sonnet-4"))
	assert.Equal(t, 128000, LookupContextWindow("gpt-4o"))
	assert.Equal(t, 1048576, LookupContextWindow("gemini-2.5-flash"))
}

func TestLookupContextWindow_PrefixMatch(t *testing.T) {
	assert.Equal(t, 200000, LookupContextWindow("claude-sonnet-4-20250514"))
	assert.Equal(t, 200000, LookupContextWindow("claude-opus-4-5-20251101"))
	assert.Equal(t, 128000, LookupContextWindow("gpt-4o-2024-05-13"))
	assert.Equal(t, 1048576, LookupContextWindow("gemini-2.0-flash-latest"))
}

func TestLookupContextWindow_CaseInsensitive(t *testing.T) {
	assert.Equal(t, 200000, LookupContextWindow("Claude-Sonnet-4"))
	assert.Equal(t, 128000, LookupContextWindow("GPT-4o"))
}

func TestLookupContextWindow_UnknownModel(t *testing.T) {
	assert.Equal(t, FallbackContextWindow, LookupContextWindow("some-unknown-model"))
}

func TestLookupContextWindow_EmptyString(t *testing.T) {
	assert.Equal(t, FallbackContextWindow, LookupContextWindow(""))
}

func TestLookupContextWindow_LongestPrefixWins(t *testing.T) {
	// "gpt-4o-mini" should match "gpt-4o-mini" not "gpt-4o" or "gpt-4"
	assert.Equal(t, 128000, LookupContextWindow("gpt-4o-mini"))
	// "gpt-4" should match "gpt-4" (8192), not "gpt-4o" (128000)
	assert.Equal(t, 8192, LookupContextWindow("gpt-4"))
}

func TestContextBudget_ExplicitBudgetTakesPriority(t *testing.T) {
	budget := ContextBudget(50000, "claude-sonnet-4", 0.7)
	assert.Equal(t, 50000, budget)
}

func TestContextBudget_FallsBackToModelLookup(t *testing.T) {
	// 200000 * 0.7 = 140000
	budget := ContextBudget(0, "claude-sonnet-4", 0.7)
	assert.Equal(t, 140000, budget)
}

func TestContextBudget_CustomRatio(t *testing.T) {
	budget := ContextBudget(0, "claude-sonnet-4", 0.5)
	assert.Equal(t, 100000, budget)
}

func TestContextBudget_InvalidRatioDefaultsTo07(t *testing.T) {
	budget := ContextBudget(0, "claude-sonnet-4", 0)
	assert.Equal(t, 140000, budget)

	budget = ContextBudget(0, "claude-sonnet-4", -1)
	assert.Equal(t, 140000, budget)

	budget = ContextBudget(0, "claude-sonnet-4", 1.5)
	assert.Equal(t, 140000, budget)
}

func TestContextBudget_UnknownModelUsesFallback(t *testing.T) {
	// FallbackContextWindow (128000) * 0.7 = 89600
	budget := ContextBudget(0, "mystery-model", 0.7)
	assert.Equal(t, 89600, budget)
}
