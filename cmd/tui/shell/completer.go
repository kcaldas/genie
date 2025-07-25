package shell

// Suggester provides suggestions based on input prefix
type Suggester interface {
	// GetSuggestions returns suggestions for the given input
	// Returns empty slice if no suggestions available
	GetSuggestions(input string) []string
	
	// ShouldSuggest returns true if this suggester should provide suggestions for the input
	ShouldSuggest(input string) bool
}

// Completer provides a clean, simple API for suggestions with complete key handling
type Completer struct {
	suggesters []Suggester
}

// NewCompleter creates a new completer
func NewCompleter() *Completer {
	return &Completer{
		suggesters: make([]Suggester, 0),
	}
}

// RegisterSuggester adds a suggester to the completer
func (c *Completer) RegisterSuggester(suggester Suggester) {
	c.suggesters = append(c.suggesters, suggester)
}

// Suggest returns the first available suggestion for the input, or empty string if none
func (c *Completer) Suggest(input string) string {
	// Try each suggester in order
	for _, suggester := range c.suggesters {
		if suggester.ShouldSuggest(input) {
			suggestions := suggester.GetSuggestions(input)
			if len(suggestions) > 0 {
				return suggestions[0] // Return first suggestion
			}
		}
	}
	
	return "" // No suggestions available
}