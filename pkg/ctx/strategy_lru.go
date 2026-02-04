package ctx

// FileEntry represents a file in the file context provider.
type FileEntry struct {
	Path    string
	Content string
}

// LRUStrategy keeps the N most recently accessed items that fit within the token budget.
// Items are expected in recency order (most recent first).
type LRUStrategy struct {
	maxItems int
}

// NewLRUStrategy creates a new LRU strategy.
// maxItems limits the number of items kept. Pass 0 for no item limit (budget-only).
func NewLRUStrategy(maxItems int) *LRUStrategy {
	return &LRUStrategy{maxItems: maxItems}
}

func (s *LRUStrategy) Name() string {
	return "lru"
}

// ApplyToCollection keeps the most recent files that fit within the budget.
// Items arrive in recency order (most recent first). Returns items in the same order.
func (s *LRUStrategy) ApplyToCollection(items []FileEntry, budgetTokens int, formatItem func(FileEntry) string) ([]FileEntry, int) {
	if len(items) == 0 || budgetTokens <= 0 {
		return nil, 0
	}

	var kept []FileEntry
	tokensUsed := 0

	for i, item := range items {
		// Check item count limit
		if s.maxItems > 0 && i >= s.maxItems {
			break
		}

		formatted := formatItem(item)
		itemTokens := EstimateTokens(formatted)

		if tokensUsed+itemTokens > budgetTokens {
			break
		}

		kept = append(kept, item)
		tokensUsed += itemTokens
	}

	return kept, tokensUsed
}
