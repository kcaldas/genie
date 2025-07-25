package shell

// Suggester provides suggestions based on input prefix
type Suggester interface {
	// GetSuggestions returns suggestions for the given input
	// Returns empty slice if no suggestions available
	GetSuggestions(input string) []string
	
	// ShouldSuggest returns true if this suggester should provide suggestions for the input
	ShouldSuggest(input string) bool
	
	// GetPrefix returns the prefix this suggester handles (e.g., ":", "/")
	GetPrefix() string
}

// Completer provides a clean, simple API for suggestions with complete key handling
type Completer struct {
	suggesters       []Suggester                // Legacy: all suggesters
	prefixSuggesters map[string]Suggester      // New: prefix-mapped suggesters
}

// NewCompleter creates a new completer
func NewCompleter() *Completer {
	return &Completer{
		suggesters:       make([]Suggester, 0),
		prefixSuggesters: make(map[string]Suggester),
	}
}

// RegisterSuggester adds a suggester to the completer
func (c *Completer) RegisterSuggester(suggester Suggester) {
	c.suggesters = append(c.suggesters, suggester)
	
	// Also map by prefix for efficient lookup
	prefix := suggester.GetPrefix()
	if prefix != "" {
		c.prefixSuggesters[prefix] = suggester
	}
}

// Suggest returns the first available suggestion for the input, or empty string if none
func (c *Completer) Suggest(input string) string {
	// First try prefix-based lookup for efficiency
	if len(input) > 0 {
		// Check for single character prefixes first
		if suggester, exists := c.prefixSuggesters[input[:1]]; exists {
			if suggester.ShouldSuggest(input) {
				suggestions := suggester.GetSuggestions(input)
				if len(suggestions) > 0 {
					return suggestions[0]
				}
			}
		}
		
		// Check for longer prefixes (e.g., "//", "::")
		for prefix, suggester := range c.prefixSuggesters {
			if len(prefix) > 1 && len(input) >= len(prefix) && input[:len(prefix)] == prefix {
				if suggester.ShouldSuggest(input) {
					suggestions := suggester.GetSuggestions(input)
					if len(suggestions) > 0 {
						return suggestions[0]
					}
				}
			}
		}
	}
	
	// Fallback to legacy behavior for suggesters without prefixes
	for _, suggester := range c.suggesters {
		// Skip if already handled by prefix mapping
		if suggester.GetPrefix() != "" {
			continue
		}
		
		if suggester.ShouldSuggest(input) {
			suggestions := suggester.GetSuggestions(input)
			if len(suggestions) > 0 {
				return suggestions[0]
			}
		}
	}
	
	return "" // No suggestions available
}