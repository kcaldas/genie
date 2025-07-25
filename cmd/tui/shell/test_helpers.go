package shell

// MockRegistryForTesting implements CommandRegistry interface for testing
type MockRegistryForTesting struct {
	commandNames []string
}

func (m *MockRegistryForTesting) GetCommandNames() []string {
	return m.commandNames
}

// CreateTestCommandRegistry creates a mock command registry for testing
func CreateTestCommandRegistry() CommandRegistry {
	return &MockRegistryForTesting{
		commandNames: []string{"write", "w", "yank", "y"},
	}
}