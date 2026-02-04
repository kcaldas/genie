package ctx

// SlidingWindowStrategy keeps the most recent messages that fit within the token budget.
// Builds from newest to oldest, ensuring the most recent context is always preserved.
type SlidingWindowStrategy struct{}

func NewSlidingWindowStrategy() *SlidingWindowStrategy {
	return &SlidingWindowStrategy{}
}

func (s *SlidingWindowStrategy) Name() string {
	return "sliding_window"
}

// ApplyToCollection keeps the most recent messages that fit within the budget.
// Items arrive in chronological order (oldest first). Returns items in the same order.
func (s *SlidingWindowStrategy) ApplyToCollection(items []Message, budgetTokens int, formatItem func(Message) string) ([]Message, int) {
	if len(items) == 0 || budgetTokens <= 0 {
		return nil, 0
	}

	// Walk backwards (newest first), accumulate until budget is exhausted
	tokensUsed := 0
	startIdx := len(items) // will move backwards

	for i := len(items) - 1; i >= 0; i-- {
		formatted := formatItem(items[i])
		itemTokens := EstimateTokens(formatted)

		if tokensUsed+itemTokens > budgetTokens {
			break
		}

		tokensUsed += itemTokens
		startIdx = i
	}

	if startIdx >= len(items) {
		return nil, 0
	}

	kept := make([]Message, len(items)-startIdx)
	copy(kept, items[startIdx:])
	return kept, tokensUsed
}
