package commands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockHelpController for testing
type MockHelpController struct {
	mock.Mock
}

func (m *MockHelpController) ShowHelp() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockHelpController) ShowSlashCommandsHelp() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockHelpController) ToggleHelp() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockHelpController) IsVisible() bool {
	args := m.Called()
	return args.Bool(0)
}

func TestHelpCommand_Execute(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedMethod string
		description    string
	}{
		{
			name:           "no arguments",
			args:           []string{},
			expectedMethod: "ToggleHelp",
			description:    "Should call ToggleHelp when no arguments provided",
		},
		{
			name:           "slash argument",
			args:           []string{"/"},
			expectedMethod: "ShowSlashCommandsHelp",
			description:    "Should call ShowSlashCommandsHelp when '/' argument provided",
		},
		{
			name:           "slash word argument",
			args:           []string{"slash"},
			expectedMethod: "ShowSlashCommandsHelp",
			description:    "Should call ShowSlashCommandsHelp when 'slash' argument provided",
		},
		{
			name:           "other argument",
			args:           []string{"config"},
			expectedMethod: "ToggleHelp",
			description:    "Should call ToggleHelp for other arguments",
		},
		{
			name:           "multiple arguments with slash",
			args:           []string{"/", "extra"},
			expectedMethod: "ShowSlashCommandsHelp",
			description:    "Should call ShowSlashCommandsHelp when first argument is '/'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockController := new(MockHelpController)
			
			// Set up expectation based on which method should be called
			if tt.expectedMethod == "ShowSlashCommandsHelp" {
				mockController.On("ShowSlashCommandsHelp").Return(nil)
			} else {
				mockController.On("ToggleHelp").Return(nil)
			}

			helpCommand := NewHelpCommand(mockController)
			
			err := helpCommand.Execute(tt.args)
			
			assert.NoError(t, err, tt.description)
			mockController.AssertExpectations(t)
		})
	}
}

func TestHelpCommand_Metadata(t *testing.T) {
	mockController := new(MockHelpController)
	helpCommand := NewHelpCommand(mockController)

	// Test command metadata
	assert.Equal(t, "help", helpCommand.GetName())
	assert.Equal(t, "Show help message with available commands and shortcuts", helpCommand.GetDescription())
	assert.Equal(t, ":help [command]", helpCommand.GetUsage())
	assert.Contains(t, helpCommand.GetAliases(), "h")
	assert.Contains(t, helpCommand.GetAliases(), "?")
	assert.Equal(t, "General", helpCommand.GetCategory())

	// Test examples include slash command help
	examples := helpCommand.GetExamples()
	assert.Contains(t, examples, ":help /")
	assert.Contains(t, examples, ":help slash")
	
	// Verify the examples contain slash help options
	hasSlashExample := false
	for _, example := range examples {
		if strings.Contains(example, "/") || strings.Contains(example, "slash") {
			hasSlashExample = true
			break
		}
	}
	assert.True(t, hasSlashExample, "Examples should include slash command help options")
}