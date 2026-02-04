package ctx

import "fmt"

// SoftTrimStrategy preserves the head and tail of content, dropping the middle.
// Useful for large file contents and tool outputs where the beginning (imports, declarations)
// and end (recent code) are most informative.
type SoftTrimStrategy struct {
	headChars int
	tailChars int
}

// NewSoftTrimStrategy creates a new SoftTrim strategy.
// headChars and tailChars control how many characters to keep from each end.
func NewSoftTrimStrategy(headChars, tailChars int) *SoftTrimStrategy {
	return &SoftTrimStrategy{
		headChars: headChars,
		tailChars: tailChars,
	}
}

func (s *SoftTrimStrategy) Name() string {
	return "soft_trim"
}

func (s *SoftTrimStrategy) Apply(content string, budgetTokens int) (string, int) {
	if budgetTokens <= 0 {
		return "", 0
	}

	tokens := EstimateTokens(content)
	if tokens <= budgetTokens {
		return content, tokens
	}

	contentLen := len(content)
	keepChars := s.headChars + s.tailChars

	// If content is shorter than head+tail, just hard-cap it
	if contentLen <= keepChars {
		capped := content
		cappedTokens := EstimateTokens(capped)
		if cappedTokens <= budgetTokens {
			return capped, cappedTokens
		}
		// Still too big, hard-cap to budget
		maxChars := budgetTokens * 4
		if maxChars >= contentLen {
			return content, EstimateTokens(content)
		}
		return content[:maxChars], budgetTokens
	}

	// Keep head and tail, drop middle
	head := content[:s.headChars]
	tail := content[contentLen-s.tailChars:]
	droppedChars := contentLen - keepChars
	droppedTokens := EstimateTokens(content[s.headChars : contentLen-s.tailChars])

	trimmed := fmt.Sprintf("%s\n\n... [%d characters / ~%d tokens omitted] ...\n\n%s", head, droppedChars, droppedTokens, tail)

	resultTokens := EstimateTokens(trimmed)

	// If the trimmed version still exceeds budget, hard-cap it
	if resultTokens > budgetTokens {
		maxChars := budgetTokens * 4
		if maxChars >= len(trimmed) {
			return trimmed, resultTokens
		}
		return trimmed[:maxChars], budgetTokens
	}

	return trimmed, resultTokens
}
