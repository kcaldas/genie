package ctx

import "strings"

// Default context window sizes for known models (tokens).
// These are fallback values — explicit budget configuration always takes priority.
// Uses prefix matching so "claude-sonnet-4" matches "claude-sonnet-4-20250514".
var defaultContextWindows = map[string]int{
	// Anthropic
	"claude-opus-5":     200000,
	"claude-opus-4":     200000,
	"claude-sonnet-4":   200000,
	"claude-3-5-sonnet": 200000,
	"claude-3-5-haiku":  200000,
	"claude-3-opus":     200000,
	"claude-3-sonnet":   200000,
	"claude-3-haiku":    200000,

	// OpenAI
	"gpt-4o":      128000,
	"gpt-4o-mini": 128000,
	"gpt-4-turbo": 128000,
	"gpt-4.1":     1047576,
	"gpt-4":       8192,
	"o1":          200000,
	"o1-mini":     128000,
	"o3":          200000,
	"o3-mini":     200000,
	"o4-mini":     200000,

	// Google
	"gemini-3.5-flash": 1048576,
	"gemini-3.5-pro":   1048576,
	"gemini-3":         1048576,
	"gemini-2.5-flash": 1048576,
	"gemini-2.5-pro":   1048576,
	"gemini-2.0-flash": 1048576,
	"gemini-1.5-flash": 1048576,
	"gemini-1.5-pro":   2097152,

	// Local models (conservative defaults)
	"llama":     8192,
	"mistral":   32768,
	"codellama": 16384,
	"deepseek":  32768,
	"qwen":      32768,
}

// FallbackContextWindow is used when the model is completely unknown.
const FallbackContextWindow = 128000

// DefaultBudgetRatio is the default fraction of context window used for context.
// The rest is reserved for system prompt and response generation.
const DefaultBudgetRatio = 0.7

// LookupContextWindow returns the context window size for a given model name.
// Uses prefix matching: "claude-sonnet-4-20250514" matches "claude-sonnet-4".
// Returns FallbackContextWindow for unknown models.
func LookupContextWindow(modelName string) int {
	modelName = strings.TrimSpace(strings.ToLower(modelName))
	if modelName == "" {
		return FallbackContextWindow
	}

	// Exact match first
	if tokens, ok := defaultContextWindows[modelName]; ok {
		return tokens
	}

	// Prefix matching — longest prefix wins
	bestMatch := ""
	bestTokens := 0
	for prefix, tokens := range defaultContextWindows {
		if strings.HasPrefix(modelName, strings.ToLower(prefix)) && len(prefix) > len(bestMatch) {
			bestMatch = prefix
			bestTokens = tokens
		}
	}

	if bestMatch != "" {
		return bestTokens
	}

	return FallbackContextWindow
}

// ContextBudget calculates the token budget for context.
// Priority: explicitBudget (if > 0) → model lookup × ratio.
func ContextBudget(explicitBudget int, modelName string, budgetRatio float64) int {
	if explicitBudget > 0 {
		return explicitBudget
	}

	if budgetRatio <= 0 || budgetRatio > 1.0 {
		budgetRatio = DefaultBudgetRatio
	}

	window := LookupContextWindow(modelName)
	return int(float64(window) * budgetRatio)
}
