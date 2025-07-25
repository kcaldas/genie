package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompleter_PrefixBasedSuggestion(t *testing.T) {
	// Create a completer and register both types of suggesters
	completer := NewCompleter()

	// Create mock managers
	commandRegistry := CreateTestCommandRegistry()
	slashCommandManager := CreateTestSlashCommandManager()

	// Create suggesters
	commandSuggester := NewCommandSuggester(commandRegistry)
	slashCommandSuggester := NewSlashCommandSuggester(slashCommandManager)

	// Register both suggesters
	completer.RegisterSuggester(commandSuggester)
	completer.RegisterSuggester(slashCommandSuggester)

	tests := []struct {
		name     string
		input    string
		expected string
		desc     string
	}{
		{
			name:     "command_suggestion",
			input:    ":w",
			expected: ":write",
			desc:     "Should suggest command with : prefix",
		},
		{
			name:     "slash_command_suggestion",
			input:    "/c",
			expected: "/compact",
			desc:     "Should suggest slash command with / prefix",
		},
		{
			name:     "command_partial",
			input:    ":y",
			expected: ":yank",
			desc:     "Should suggest partial command match",
		},
		{
			name:     "slash_command_partial",
			input:    "/v",
			expected: "/verbose",
			desc:     "Should suggest partial slash command match",
		},
		{
			name:     "no_suggestion_for_complete_command",
			input:    ":write",
			expected: "",
			desc:     "No suggestion for complete command",
		},
		{
			name:     "no_suggestion_for_complete_slash_command",
			input:    "/compact",
			expected: "",
			desc:     "No suggestion for complete slash command",
		},
		{
			name:     "no_suggestion_for_unknown",
			input:    ":unknown",
			expected: "",
			desc:     "No suggestion for unknown command",
		},
		{
			name:     "no_suggestion_for_unknown_slash",
			input:    "/unknown",
			expected: "",
			desc:     "No suggestion for unknown slash command",
		},
		{
			name:     "no_suggestion_for_plain_text",
			input:    "hello",
			expected: "",
			desc:     "No suggestion for plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestion := completer.Suggest(tt.input)
			assert.Equal(t, tt.expected, suggestion, "%s - Input: %s", tt.desc, tt.input)
		})
	}
}

func TestCompleter_PrefixMappingEfficiency(t *testing.T) {
	// This test verifies that prefix mapping works correctly
	// by ensuring the right suggester is called for each prefix
	completer := NewCompleter()

	commandRegistry := CreateTestCommandRegistry()
	slashCommandManager := CreateTestSlashCommandManager()

	commandSuggester := NewCommandSuggester(commandRegistry)
	slashCommandSuggester := NewSlashCommandSuggester(slashCommandManager)

	completer.RegisterSuggester(commandSuggester)
	completer.RegisterSuggester(slashCommandSuggester)

	// Verify prefix mappings exist
	assert.Equal(t, ":", commandSuggester.GetPrefix())
	assert.Equal(t, "/", slashCommandSuggester.GetPrefix())

	// Test that prefix lookup works correctly
	colonSuggestion := completer.Suggest(":w")
	slashSuggestion := completer.Suggest("/c")

	assert.Equal(t, ":write", colonSuggestion)
	assert.Equal(t, "/compact", slashSuggestion)
}