package ctx

// BudgetStrategy controls how a context provider fits its content within a token budget.
// Different strategies can be swapped in per provider, per agent, per experiment.
type BudgetStrategy interface {
	// Name returns a human-readable identifier for logging/debugging.
	Name() string

	// Apply takes raw content and a token budget, returns trimmed content
	// and the estimated tokens used. The strategy decides HOW to fit.
	Apply(content string, budgetTokens int) (trimmed string, tokensUsed int)
}

// CollectionBudgetStrategy controls how a collection-based provider fits within a token budget.
// Used by providers that hold ordered items (chat messages, files).
type CollectionBudgetStrategy[T any] interface {
	// Name returns a human-readable identifier for logging/debugging.
	Name() string

	// ApplyToCollection takes ordered items and a budget, returns the items
	// to keep (possibly modified) and tokens used.
	ApplyToCollection(items []T, budgetTokens int, formatItem func(T) string) (kept []T, tokensUsed int)
}
