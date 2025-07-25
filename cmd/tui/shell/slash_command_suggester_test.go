package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlashCommandSuggester_Suggestions(t *testing.T) {
	manager := &MockSlashCommandManager{
		commandNames: []string{"compact", "verbose", "test:command", "help"},
	}
	suggester := NewSlashCommandSuggester(manager)

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "/c",
			input:    "/c",
			expected: []string{"/compact"},
		},
		{
			name:     "/co",
			input:    "/co",
			expected: []string{"/compact"},
		},
		{
			name:     "/com",
			input:    "/com",
			expected: []string{"/compact"},
		},
		{
			name:     "/comp",
			input:    "/comp",
			expected: []string{"/compact"},
		},
		{
			name:     "/compact",
			input:    "/compact",
			expected: []string{},
		},
		{
			name:     "/v",
			input:    "/v",
			expected: []string{"/verbose"},
		},
		{
			name:     "/ve",
			input:    "/ve",
			expected: []string{"/verbose"},
		},
		{
			name:     "/ver",
			input:    "/ver",
			expected: []string{"/verbose"},
		},
		{
			name:     "/verbose",
			input:    "/verbose",
			expected: []string{},
		},
		{
			name:     "/",
			input:    "/",
			expected: []string{},
		},
		{
			name:     "/unknown",
			input:    "/unknown",
			expected: []string{},
		},
		{
			name:     "hello",
			input:    "hello",
			expected: []string{},
		},
		{
			name:     "/compact something",
			input:    "/compact something",
			expected: []string{},
		},
		{
			name:     "/t",
			input:    "/t",
			expected: []string{"/test:command"},
		},
		{
			name:     "/h",
			input:    "/h",
			expected: []string{"/help"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := suggester.GetSuggestions(tt.input)
			assert.Equal(t, tt.expected, suggestions, "Input: %s", tt.input)
		})
	}
}

func TestSlashCommandSuggester_AllCommands(t *testing.T) {
	manager := &MockSlashCommandManager{
		commandNames: []string{"command1", "command2", "test:nested"},
	}
	suggester := NewSlashCommandSuggester(manager)

	// Test that all commands can be suggested with proper prefix
	suggestions := suggester.GetSuggestions("/c")
	expected := []string{"/command1", "/command2"}
	assert.Equal(t, expected, suggestions)
}

func TestSlashCommandSuggester_ShouldSuggest(t *testing.T) {
	manager := &MockSlashCommandManager{
		commandNames: []string{"compact"},
	}
	suggester := NewSlashCommandSuggester(manager)

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "/c",
			input:    "/c",
			expected: true,
		},
		{
			name:     "/compact",
			input:    "/compact",
			expected: true,
		},
		{
			name:     "/",
			input:    "/",
			expected: true,
		},
		{
			name:     "hello",
			input:    "hello",
			expected: false,
		},
		{
			name:     ":command",
			input:    ":command",
			expected: false,
		},
		{
			name:     "/compact arg",
			input:    "/compact arg",
			expected: false,
		},
		{
			name:     "/compact  ",
			input:    "/compact  ",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := suggester.ShouldSuggest(tt.input)
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		})
	}
}

func TestSlashCommandSuggester_GetPrefix(t *testing.T) {
	manager := &MockSlashCommandManager{
		commandNames: []string{},
	}
	suggester := NewSlashCommandSuggester(manager)

	assert.Equal(t, "/", suggester.GetPrefix())
}

func TestSlashCommandSuggester_EmptyManager(t *testing.T) {
	manager := &MockSlashCommandManager{
		commandNames: []string{},
	}
	suggester := NewSlashCommandSuggester(manager)

	suggestions := suggester.GetSuggestions("/test")
	assert.Empty(t, suggestions)
}