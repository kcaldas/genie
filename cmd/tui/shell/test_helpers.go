package shell

import "strings"

// MockRegistryForTesting implements CommandRegistry interface for testing
type MockRegistryForTesting struct {
	commandNames []string
}

func (m *MockRegistryForTesting) GetCommandNames() []string {
	return m.commandNames
}

// MockSuggesterForTesting implements Suggester interface for testing
type MockSuggesterForTesting struct {
	prefix      string
	suggestions map[string][]string
}

func (m *MockSuggesterForTesting) GetSuggestions(input string) []string {
	if suggestions, exists := m.suggestions[input]; exists {
		return suggestions
	}
	return []string{}
}

func (m *MockSuggesterForTesting) ShouldSuggest(input string) bool {
	return strings.HasPrefix(input, m.prefix)
}

func (m *MockSuggesterForTesting) GetPrefix() string {
	return m.prefix
}

// CreateMockSuggester creates a mock suggester for testing
func CreateMockSuggester(prefix string, suggestions map[string][]string) Suggester {
	return &MockSuggesterForTesting{
		prefix:      prefix,
		suggestions: suggestions,
	}
}

// CreateTestCommandRegistry creates a mock command registry for testing
func CreateTestCommandRegistry() CommandRegistry {
	return &MockRegistryForTesting{
		commandNames: []string{"write", "w", "yank", "y"},
	}
}

// CreateTestSlashCommandManager creates a mock slash command manager for testing
func CreateTestSlashCommandManager() SlashCommandManager {
	return &MockSlashCommandManager{
		commandNames: []string{"compact", "verbose", "test:command"},
	}
}

// MockSlashCommandManager implements SlashCommandManager interface for testing
type MockSlashCommandManager struct {
	commandNames []string
}

func (m *MockSlashCommandManager) GetCommandNames() []string {
	return m.commandNames
}