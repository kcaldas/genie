package ctx

// EstimateTokens provides a conservative token estimate for a string.
// Uses the chars/4 heuristic which slightly overestimates for English text.
// This is intentionally conservative — better to leave room than to overflow.
func EstimateTokens(content string) int {
	if len(content) == 0 {
		return 0
	}
	return (len(content) + 3) / 4 // ceiling division
}
