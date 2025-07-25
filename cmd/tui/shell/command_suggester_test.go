package shell_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/kcaldas/genie/cmd/tui/shell"
)


// NewTestCommandSuggester creates a suggester for testing with known commands
func NewTestCommandSuggester() *TestCommandSuggester {
	return &TestCommandSuggester{
		commands: []string{
			":write",
			":w",      // alias for write
			":yank",
			":y",      // alias for yank
		},
	}
}

// TestCommandSuggester is a test-specific suggester with predictable commands
type TestCommandSuggester struct {
	commands []string
}

// GetSuggestions returns suggestions based on the current input
func (tcs *TestCommandSuggester) GetSuggestions(input string) []string {
	if !strings.HasPrefix(input, ":") {
		return nil
	}
	
	// Don't suggest anything for just ":"
	if input == ":" {
		return nil
	}
	
	var suggestions []string
	for _, cmd := range tcs.commands {
		if strings.HasPrefix(cmd, input) && cmd != input {
			suggestions = append(suggestions, cmd)
		}
	}
	
	// Sort suggestions for consistency
	sort.Strings(suggestions)
	return suggestions
}

// ShouldSuggest returns true if this suggester should provide suggestions
func (tcs *TestCommandSuggester) ShouldSuggest(input string) bool {
	// Only suggest for inputs starting with ":" and not containing spaces
	return strings.HasPrefix(input, ":") && !strings.Contains(input, " ")
}

func TestCommandSuggester_Suggestions(t *testing.T) {
	registry := shell.CreateTestCommandRegistry()
	cs := shell.NewCommandSuggester(registry)
	
	tests := []struct {
		input          string
		shouldSuggest  bool
		expectedCount  int
		expectedFirst  string
		expectedSuffix string
	}{
		// Test basic cases
		{":w", true, 1, ":write", "rite"},
		{":wr", true, 1, ":write", "ite"},
		{":wri", true, 1, ":write", "te"},
		{":writ", true, 1, ":write", "e"},
		{":write", true, 0, "", ""}, // Exact match, no suggestions
		
		// Test :y -> :yank
		{":y", true, 1, ":yank", "ank"},
		{":ya", true, 1, ":yank", "nk"},
		{":yan", true, 1, ":yank", "k"},
		{":yank", true, 0, "", ""}, // Exact match
		
		// Test edge cases
		{":", true, 0, "", ""}, // Just ":" should show no suggestions
		{":unknown", true, 0, "", ""},
		{"hello", false, 0, "", ""},
		{":write something", false, 0, "", ""}, // Space disables suggestions
		{"", false, 0, "", ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// Test ShouldSuggest
			if got := cs.ShouldSuggest(tt.input); got != tt.shouldSuggest {
				t.Errorf("ShouldSuggest(%q) = %v, want %v", tt.input, got, tt.shouldSuggest)
			}
			
			// Test GetSuggestions
			suggestions := cs.GetSuggestions(tt.input)
			if len(suggestions) != tt.expectedCount {
				t.Errorf("GetSuggestions(%q) returned %d suggestions, want %d: %v", 
					tt.input, len(suggestions), tt.expectedCount, suggestions)
			}
			
			// Test first suggestion if any
			if tt.expectedFirst != "" && len(suggestions) > 0 {
				if suggestions[0] != tt.expectedFirst {
					t.Errorf("First suggestion for %q = %q, want %q", 
						tt.input, suggestions[0], tt.expectedFirst)
				}
				
				// Test suffix calculation
				if len(tt.input) < len(suggestions[0]) {
					suffix := suggestions[0][len(tt.input):]
					if suffix != tt.expectedSuffix {
						t.Errorf("Suffix for %q = %q, want %q", 
							tt.input, suffix, tt.expectedSuffix)
					}
				}
			}
		})
	}
}

func TestCommandSuggester_AllCommands(t *testing.T) {
	registry := shell.CreateTestCommandRegistry()
	cs := shell.NewCommandSuggester(registry)
	
	// When typing just ":", no commands should be suggested (need at least one more character)
	suggestions := cs.GetSuggestions(":")
	if len(suggestions) != 0 {
		t.Errorf("Expected 0 suggestions for ':', got %d: %v", len(suggestions), suggestions)
	}
	
	// But when typing something after ":", we should get matching suggestions
	suggestions = cs.GetSuggestions(":w")
	if len(suggestions) != 1 || suggestions[0] != ":write" {
		t.Errorf("Expected [':write'] for ':w', got %v", suggestions)
	}
	
	suggestions = cs.GetSuggestions(":y")
	if len(suggestions) != 1 || suggestions[0] != ":yank" {
		t.Errorf("Expected [':yank'] for ':y', got %v", suggestions)
	}
}