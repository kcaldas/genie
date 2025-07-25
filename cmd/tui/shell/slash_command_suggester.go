package shell

import (
	"sort"
	"strings"
)

// SlashCommandManager interface defines what the SlashCommandSuggester needs from a slash command manager
type SlashCommandManager interface {
	GetCommandNames() []string
}

// SlashCommandSuggester provides suggestions for slash commands (starting with /)
type SlashCommandSuggester struct {
	manager SlashCommandManager
}

// NewSlashCommandSuggester creates a new slash command suggester with a slash command manager
func NewSlashCommandSuggester(manager SlashCommandManager) *SlashCommandSuggester {
	return &SlashCommandSuggester{
		manager: manager,
	}
}

// GetSuggestions returns suggestions based on the current input
func (scs *SlashCommandSuggester) GetSuggestions(input string) []string {
	if !strings.HasPrefix(input, "/") {
		return []string{}
	}
	
	// Don't suggest anything for just "/"
	if input == "/" {
		return []string{}
	}
	
	var suggestions []string
	commandNames := scs.manager.GetCommandNames()
	
	for _, name := range commandNames {
		// Add "/" prefix back for suggestions
		fullCommand := "/" + name
		if strings.HasPrefix(fullCommand, input) && fullCommand != input {
			suggestions = append(suggestions, fullCommand)
		}
	}
	
	// Sort suggestions for consistency
	sort.Strings(suggestions)
	if suggestions == nil {
		return []string{}
	}
	return suggestions
}

// ShouldSuggest returns true if this suggester should provide suggestions
func (scs *SlashCommandSuggester) ShouldSuggest(input string) bool {
	// Only suggest for inputs starting with "/" and not containing spaces
	return strings.HasPrefix(input, "/") && !strings.Contains(input, " ")
}

// GetPrefix returns the prefix this suggester handles
func (scs *SlashCommandSuggester) GetPrefix() string {
	return "/"
}