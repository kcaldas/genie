package ctx

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/stretchr/testify/assert"
)

func TestDefaultMaxTokensLeavesInputRoomForEveryRegistryEntry(t *testing.T) {
	t.Setenv("GENIE_MAX_TOKENS", "")
	maxTokens := config.NewConfigManager().GetModelConfig().MaxTokens
	for model, info := range defaultModelRegistry {
		t.Run(model, func(t *testing.T) {
			admission := llmshared.ModelInputAdmissionLimit(ai.Prompt{
				MaxTokens: maxTokens,
				ModelCapabilities: &ai.ModelCapabilities{
					Model: model, InputTokenLimit: info.ContextWindow,
					OutputTokenLimit: info.MaxOutputTokens, SharedContextWindow: true,
				},
			})
			assert.GreaterOrEqual(t, admission, info.ContextWindow/2,
				"default output reserve must leave usable request input")
		})
	}
}

func TestLookupContextWindow_KnownModels(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		// Anthropic
		{model: "claude-opus-5", want: 1000000},
		{model: "claude-sonnet-5-20260801", want: 1000000},
		{model: "claude-opus-4-8", want: 1000000},
		{model: "claude-opus-4-6-20260301", want: 1000000},
		{model: "claude-sonnet-4-6", want: 1000000},
		{model: "claude-opus-4-5-20251101", want: 200000},
		{model: "claude-haiku-4-5-20251001", want: 200000},

		// OpenAI
		{model: "gpt-5", want: 400000},
		{model: "gpt-5-mini", want: 400000},
		{model: "gpt-5-chat-latest", want: 128000},
		{model: "gpt-5.1", want: 400000},
		{model: "gpt-5.1-chat-latest", want: 128000},
		{model: "gpt-5.2-pro", want: 400000},
		{model: "gpt-5.3-codex", want: 400000},
		{model: "gpt-5.3-chat-latest", want: 128000},
		{model: "gpt-5.4", want: 1050000},
		{model: "gpt-5.4-pro", want: 1050000},
		{model: "gpt-5.4-mini", want: 400000},
		{model: "gpt-5.4-nano-2026-03-17", want: 400000},
		{model: "gpt-5.5-pro", want: 1050000},
		{model: "gpt-5.6", want: 1050000},
		{model: "gpt-5.6-sol", want: 1050000},
		{model: "gpt-5.6-terra", want: 1050000},
		{model: "gpt-5.6-luna", want: 1050000},
		{model: "gpt-4o-2024-05-13", want: 128000},

		// Google
		{model: "gemini-3.6-flash", want: 1048576},
		{model: "gemini-3.5-flash-lite", want: 1048576},
		{model: "gemini-3.1-pro-preview", want: 1048576},
		{model: "gemini-3.1-flash-image-preview", want: 131072},
		{model: "gemini-3-pro-image", want: 65536},
		{model: "gemini-3-flash-preview", want: 1048576},
		{model: "gemini-2.0-flash-latest", want: 1048576},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.want, LookupContextWindow(tt.model))
		})
	}
}

func TestLookupContextWindow_CaseInsensitive(t *testing.T) {
	assert.Equal(t, 200000, LookupContextWindow("Claude-Sonnet-4"))
	assert.Equal(t, 1050000, LookupContextWindow("GPT-5.6-LUNA"))
}

func TestLookupContextWindow_UnknownModel(t *testing.T) {
	assert.Equal(t, FallbackContextWindow, LookupContextWindow("some-unknown-model"))
	_, ok := LookupModelInfo("some-unknown-model")
	assert.False(t, ok)
}

func TestLookupModelInfoPreservesUnknownModalities(t *testing.T) {
	info, ok := LookupModelInfo("gpt-5.6-luna")
	assert.True(t, ok)
	assert.Equal(t, 1050000, info.ContextWindow)
	assert.Nil(t, info.InputModalities)
}

func TestLookupContextWindow_EmptyString(t *testing.T) {
	assert.Equal(t, FallbackContextWindow, LookupContextWindow(""))
}

func TestLookupContextWindow_LongestPrefixWins(t *testing.T) {
	assert.Equal(t, 400000, LookupContextWindow("gpt-5.4-mini"))
	assert.Equal(t, 128000, LookupContextWindow("gpt-5.3-chat-latest"))
	assert.Equal(t, 131072, LookupContextWindow("gemini-3.1-flash-image-preview"))
}

func TestLookupContextWindow_DoesNotMatchUnrelatedPrefix(t *testing.T) {
	assert.Equal(t, FallbackContextWindow, LookupContextWindow("gpt-40"))
	assert.Equal(t, FallbackContextWindow, LookupContextWindow("gemini-30"))
	assert.Equal(t, FallbackContextWindow, LookupContextWindow("claude-opus-40"))
}

func TestLookupContextWindow_LocalModelsAllowConcatenatedGeneration(t *testing.T) {
	assert.Equal(t, 8192, LookupContextWindow("llama3.1"))
	assert.Equal(t, 32768, LookupContextWindow("qwen2.5-coder"))
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
