package shell

import (
	"sort"
	"strings"
)

// CommandRegistry interface defines what the CommandSuggester needs from a registry
type CommandRegistry interface {
	GetCommandNames() []string
}

// CommandSuggester provides suggestions for internal commands (starting with :)
type CommandSuggester struct {
	registry CommandRegistry
}

// NewCommandSuggester creates a new command suggester with a command registry
func NewCommandSuggester(registry CommandRegistry) *CommandSuggester {
	return &CommandSuggester{
		registry: registry,
	}
}

// GetSuggestions returns suggestions based on the current input
func (cs *CommandSuggester) GetSuggestions(input string) []string {
	if !strings.HasPrefix(input, ":") {
		return nil
	}
	
	// Don't suggest anything for just ":"
	if input == ":" {
		return nil
	}
	
	var suggestions []string
	commandNames := cs.registry.GetCommandNames()
	
	for _, name := range commandNames {
		// Add ":" prefix back for suggestions
		fullCommand := ":" + name
		if strings.HasPrefix(fullCommand, input) && fullCommand != input {
			suggestions = append(suggestions, fullCommand)
		}
	}
	
	// Sort suggestions for consistency
	sort.Strings(suggestions)
	return suggestions
}

// ShouldSuggest returns true if this suggester should provide suggestions
func (cs *CommandSuggester) ShouldSuggest(input string) bool {
	// Only suggest for inputs starting with ":" and not containing spaces
	return strings.HasPrefix(input, ":") && !strings.Contains(input, " ")
}

// GetPrefix returns the prefix this suggester handles
func (cs *CommandSuggester) GetPrefix() string {
	return ":"
}