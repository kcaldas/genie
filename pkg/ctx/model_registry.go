package ctx

import "strings"

// Default context window sizes for known models (tokens).
// These are fallback values — explicit budget configuration always takes priority.
// Uses prefix matching so "claude-sonnet-4" matches "claude-sonnet-4-20250514".
var defaultContextWindows = map[string]int{
	// Anthropic
	"claude-fable-5":    1000000,
	"claude-opus-5":     1000000,
	"claude-sonnet-5":   1000000,
	"claude-opus-4-8":   1000000,
	"claude-opus-4-7":   1000000,
	"claude-opus-4-6":   1000000,
	"claude-sonnet-4-6": 1000000,
	"claude-haiku-4-5":  200000,
	"claude-opus-4":     200000,
	"claude-sonnet-4":   200000,
	"claude-3-5-sonnet": 200000,
	"claude-3-5-haiku":  200000,
	"claude-3-opus":     200000,
	"claude-3-sonnet":   200000,
	"claude-3-haiku":    200000,

	// OpenAI
	"gpt-5.6":             1050000,
	"gpt-5.5":             1050000,
	"gpt-5.4-mini":        400000,
	"gpt-5.4-nano":        400000,
	"gpt-5.4":             1050000,
	"gpt-5.3-chat-latest": 128000,
	"gpt-5.3-codex":       400000,
	"gpt-5.2":             400000,
	"gpt-5.1-chat-latest": 128000,
	"gpt-5.1":             400000,
	"gpt-5-chat-latest":   128000,
	"gpt-5":               400000,
	"chat-latest":         400000,
	"gpt-4o":              128000,
	"gpt-4o-mini":         128000,
	"gpt-4-turbo":         128000,
	"gpt-4.1":             1047576,
	"gpt-4":               8192,
	"o1":                  200000,
	"o1-mini":             128000,
	"o3":                  200000,
	"o3-mini":             200000,
	"o4-mini":             200000,

	// Google
	"gemini-3.6-flash":               1048576,
	"gemini-3.5-flash-lite":          1048576,
	"gemini-3.5-flash":               1048576,
	"gemini-3.1-flash-image-preview": 131072,
	"gemini-3.1-flash-image":         131072,
	"gemini-3.1-flash-lite":          1048576,
	"gemini-3.1-pro-preview":         1048576,
	"gemini-3-pro-image-preview":     65536,
	"gemini-3-pro-image":             65536,
	"gemini-3-flash-preview":         1048576,
	"gemini-3-pro-preview":           1048576,
	"gemini-2.5-flash":               1048576,
	"gemini-2.5-pro":                 1048576,
	"gemini-2.0-flash":               1048576,
	"gemini-1.5-flash":               1048576,
	"gemini-1.5-pro":                 2097152,

	// Local models (conservative defaults)
	"llama":     8192,
	"mistral":   32768,
	"codellama": 16384,
	"deepseek":  32768,
	"qwen":      32768,
}

// Local model names commonly concatenate the family and generation (for
// example, llama3.1). Hosted model names must match on a model-name boundary.
var concatenatedModelPrefixes = map[string]struct{}{
	"llama":     {},
	"mistral":   {},
	"codellama": {},
	"deepseek":  {},
	"qwen":      {},
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
		if matchesModelPrefix(modelName, prefix) && len(prefix) > len(bestMatch) {
			bestMatch = prefix
			bestTokens = tokens
		}
	}

	if bestMatch != "" {
		return bestTokens
	}

	return FallbackContextWindow
}

func matchesModelPrefix(modelName, prefix string) bool {
	if !strings.HasPrefix(modelName, prefix) {
		return false
	}
	if len(modelName) == len(prefix) {
		return true
	}
	if _, ok := concatenatedModelPrefixes[prefix]; ok {
		return true
	}

	switch modelName[len(prefix)] {
	case '-', '@':
		return true
	default:
		return false
	}
}

// ContextBudget calculates the token budget for context.
// Priority: explicitBudget (if > 0) → model lookup × ratio.
func ContextBudget(explicitBudget int, modelName string, budgetRatio float64) int {
	return ContextBudgetForWindow(explicitBudget, LookupContextWindow(modelName), budgetRatio)
}

// ContextBudgetForWindow calculates a synthetic text budget from an
// authoritative model input window. Explicit budgets are host policy and are
// intentionally not clamped here.
func ContextBudgetForWindow(explicitBudget, contextWindow int, budgetRatio float64) int {
	if explicitBudget > 0 {
		return explicitBudget
	}

	if budgetRatio <= 0 || budgetRatio > 1.0 {
		budgetRatio = DefaultBudgetRatio
	}

	if contextWindow <= 0 {
		contextWindow = FallbackContextWindow
	}
	return int(float64(contextWindow) * budgetRatio)
}
