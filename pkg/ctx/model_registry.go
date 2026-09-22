package ctx

import (
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
)

// ModelInfo is checked-in model metadata used synchronously on the request
// path. InputModalities is nil unless every value was deliberately verified.
type ModelInfo struct {
	ContextWindow   int
	MaxOutputTokens int // Zero until verified; the offline registry updater populates this field.
	InputModalities map[ai.Modality]bool
}

// Default context window sizes for known models (tokens).
// These are fallback values — explicit budget configuration always takes priority.
// Uses prefix matching so "claude-sonnet-4" matches "claude-sonnet-4-20250514".
var defaultModelRegistry = map[string]ModelInfo{
	// Anthropic (GET /v1/models, 2026-09-22: max_input_tokens / max_tokens)
	"claude-fable-5-1":  {ContextWindow: 1000000, MaxOutputTokens: 128000},
	"claude-fable-5":    {ContextWindow: 1000000, MaxOutputTokens: 128000},
	"claude-opus-5":     {ContextWindow: 1000000, MaxOutputTokens: 128000},
	"claude-sonnet-5":   {ContextWindow: 1000000, MaxOutputTokens: 128000},
	"claude-opus-4-8":   {ContextWindow: 1000000, MaxOutputTokens: 128000},
	"claude-opus-4-7":   {ContextWindow: 1000000, MaxOutputTokens: 128000},
	"claude-opus-4-6":   {ContextWindow: 1000000, MaxOutputTokens: 128000},
	"claude-sonnet-4-6": {ContextWindow: 1000000, MaxOutputTokens: 128000},
	"claude-opus-4-5":   {ContextWindow: 200000, MaxOutputTokens: 64000},
	"claude-sonnet-4-5": {ContextWindow: 1000000, MaxOutputTokens: 64000},
	"claude-haiku-4-5":  {ContextWindow: 200000, MaxOutputTokens: 64000},
	"claude-opus-4":     {ContextWindow: 200000},
	"claude-sonnet-4":   {ContextWindow: 200000},
	"claude-3-5-sonnet": {ContextWindow: 200000},
	"claude-3-5-haiku":  {ContextWindow: 200000},
	"claude-3-opus":     {ContextWindow: 200000},
	"claude-3-sonnet":   {ContextWindow: 200000},
	"claude-3-haiku":    {ContextWindow: 200000},

	// OpenAI (docs-sourced: developers.openai.com/api/docs/models/<id>, 2026-09-22)
	"gpt-6":               {ContextWindow: 1050000, MaxOutputTokens: 128000},
	"gpt-5.6":             {ContextWindow: 1050000, MaxOutputTokens: 128000},
	"gpt-5.5":             {ContextWindow: 1050000},
	"gpt-5.4-mini":        {ContextWindow: 400000},
	"gpt-5.4-nano":        {ContextWindow: 400000},
	"gpt-5.4":             {ContextWindow: 1050000},
	"gpt-5.3-chat-latest": {ContextWindow: 128000},
	"gpt-5.3-codex":       {ContextWindow: 400000},
	"gpt-5.2":             {ContextWindow: 400000},
	"gpt-5.1-chat-latest": {ContextWindow: 128000},
	"gpt-5.1":             {ContextWindow: 400000},
	"gpt-5-chat-latest":   {ContextWindow: 128000},
	"gpt-5":               {ContextWindow: 400000},
	"chat-latest":         {ContextWindow: 400000},
	"gpt-4o":              {ContextWindow: 128000},
	"gpt-4o-mini":         {ContextWindow: 128000},
	"gpt-4-turbo":         {ContextWindow: 128000},
	"gpt-4.1":             {ContextWindow: 1047576},
	"gpt-4":               {ContextWindow: 8192},
	"o1":                  {ContextWindow: 200000},
	"o1-mini":             {ContextWindow: 128000},
	"o3":                  {ContextWindow: 200000},
	"o3-mini":             {ContextWindow: 200000},
	"o4-mini":             {ContextWindow: 200000},

	// Google (GET /v1beta/models, 2026-09-22: inputTokenLimit / outputTokenLimit)
	"gemini-3.8-flash":               {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-3.7-flash":               {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-3.6-flash":               {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-3.5-flash-lite":          {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-3.5-flash":               {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-3.1-flash-image-preview": {ContextWindow: 65536, MaxOutputTokens: 65536},
	"gemini-3.1-flash-image":         {ContextWindow: 65536, MaxOutputTokens: 65536},
	"gemini-3.1-flash-lite-image":    {ContextWindow: 65536, MaxOutputTokens: 65536},
	"gemini-3.1-flash-lite":          {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-3.1-pro-preview":         {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-3-pro-image-preview":     {ContextWindow: 131072, MaxOutputTokens: 32768},
	"gemini-3-pro-image":             {ContextWindow: 131072, MaxOutputTokens: 32768},
	"gemini-3-flash-preview":         {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-3-pro-preview":           {ContextWindow: 1048576}, // stale: absent from provider catalog as of 2026-08-24
	"gemini-omni-1.1-flash":          {ContextWindow: 131072, MaxOutputTokens: 65536},
	"gemini-omni-flash-preview":      {ContextWindow: 131072, MaxOutputTokens: 65536},
	"gemini-flash-latest":            {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-flash-lite-latest":       {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-pro-latest":              {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-2.5-flash":               {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-2.5-pro":                 {ContextWindow: 1048576, MaxOutputTokens: 65536},
	"gemini-2.0-flash":               {ContextWindow: 1048576}, // stale: absent from provider catalog as of 2026-08-24
	"gemini-1.5-flash":               {ContextWindow: 1048576}, // stale: absent from provider catalog as of 2026-08-24
	"gemini-1.5-pro":                 {ContextWindow: 2097152}, // stale: absent from provider catalog as of 2026-08-24

	// DeepSeek (hosted API; 1M context / 384K max output per
	// api-docs.deepseek.com/quick_start/pricing as of 2026-09-22)
	"deepseek-flash":               {ContextWindow: 1048576, MaxOutputTokens: 393216},
	"deepseek-v4-flash":            {ContextWindow: 1048576, MaxOutputTokens: 393216}, // stale: V4 Flash retired 2026-09-10, name temporarily routed to V4.1 Flash per api-docs.deepseek.com/updates
	"deepseek-v4-flash-vision-exp": {ContextWindow: 1048576, MaxOutputTokens: 393216}, // stale: V4 Flash retired 2026-09-10, name temporarily routed to V4.1 Flash per api-docs.deepseek.com/updates
	"deepseek-v4-pro":              {ContextWindow: 1048576, MaxOutputTokens: 393216},
	"deepseek-chat":                {ContextWindow: 131072, MaxOutputTokens: 8192},  // stale: legacy alias discontinued 2026-07-24 per api-docs.deepseek.com/updates
	"deepseek-reasoner":            {ContextWindow: 131072, MaxOutputTokens: 65536}, // stale: legacy alias discontinued 2026-07-24 per api-docs.deepseek.com/updates

	// Local models (conservative defaults)
	"llama":     {ContextWindow: 8192},
	"mistral":   {ContextWindow: 32768},
	"codellama": {ContextWindow: 16384},
	"deepseek":  {ContextWindow: 32768},
	"qwen":      {ContextWindow: 32768},
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
	if info, ok := LookupModelInfo(modelName); ok {
		return info.ContextWindow
	}
	return FallbackContextWindow
}

// LookupModelInfo returns the longest prefix-matched registry entry. Unknown
// models return false so callers can preserve provider-native behavior.
func LookupModelInfo(modelName string) (ModelInfo, bool) {
	modelName = strings.TrimSpace(strings.ToLower(modelName))
	if modelName == "" {
		return ModelInfo{}, false
	}

	// Exact match first
	if info, ok := defaultModelRegistry[modelName]; ok {
		return info, true
	}

	// Prefix matching — longest prefix wins
	bestMatch := ""
	var bestInfo ModelInfo
	for prefix, info := range defaultModelRegistry {
		if matchesModelPrefix(modelName, prefix) && len(prefix) > len(bestMatch) {
			bestMatch = prefix
			bestInfo = info
		}
	}

	if bestMatch != "" {
		return bestInfo, true
	}
	return ModelInfo{}, false
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
	if explicitBudget > 0 {
		return explicitBudget
	}

	if budgetRatio <= 0 || budgetRatio > 1.0 {
		budgetRatio = DefaultBudgetRatio
	}

	window := LookupContextWindow(modelName)
	return int(float64(window) * budgetRatio)
}
