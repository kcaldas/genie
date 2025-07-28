package commands

import (
	"testing"

	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/stretchr/testify/assert"
)

func TestPersonaCommand_Execute(t *testing.T) {
	// Create mock notification
	mockNotification := &types.MockNotification{}
	
	// Create mock genie service
	mockGenie := &MockGenieService{}
	
	// Create persona command
	cmd := NewPersonaCommand(mockNotification, mockGenie)
	
	// Test basic metadata
	assert.Equal(t, "persona", cmd.GetName())
	assert.Equal(t, "Manage personas", cmd.GetDescription())
	assert.Contains(t, cmd.GetAliases(), "p")
	assert.Equal(t, "Persona", cmd.GetCategory())
	assert.Contains(t, cmd.GetUsage(), ":persona list")
	assert.Contains(t, cmd.GetUsage(), ":p -l")
	
	t.Run("list subcommand", func(t *testing.T) {
		// Create mock personas
		mockPersonas := []MockPersona{
			{id: "engineer", name: "Engineer", source: "internal"},
			{id: "custom", name: "Custom Persona", source: "user"},
			{id: "project-specific", name: "Project Assistant", source: "project"},
		}
		
		// Convert to genie.Persona slice
		geniePersonas := make([]genie.Persona, len(mockPersonas))
		for i, p := range mockPersonas {
			geniePersonas[i] = &p
		}
		
		mockGenie.mockPersonas = geniePersonas
		
		// Test ":persona list"
		err := cmd.Execute([]string{"list"})
		assert.NoError(t, err)
		assert.Len(t, mockNotification.SystemMessages, 1)
		
		message := mockNotification.SystemMessages[0]
		assert.Contains(t, message, "Available personas:")
		assert.Contains(t, message, "engineer")
		assert.Contains(t, message, "Engineer")
		assert.Contains(t, message, "internal")
		assert.Contains(t, message, "custom")
		assert.Contains(t, message, "Custom Persona")
		assert.Contains(t, message, "user")
		assert.Contains(t, message, "project-specific")
		assert.Contains(t, message, "Project Assistant")
		assert.Contains(t, message, "project")
		
		// Test ":p -l" (alias and short flag)
		mockNotification.SystemMessages = []string{} // Reset
		err = cmd.Execute([]string{"-l"})
		assert.NoError(t, err)
		assert.Len(t, mockNotification.SystemMessages, 1)
		assert.Contains(t, mockNotification.SystemMessages[0], "Available personas:")
	})
	
	t.Run("no personas", func(t *testing.T) {
		mockGenie.mockPersonas = []genie.Persona{}
		mockNotification.SystemMessages = []string{} // Reset
		
		err := cmd.Execute([]string{"list"})
		assert.NoError(t, err)
		assert.Len(t, mockNotification.SystemMessages, 1)
		assert.Contains(t, mockNotification.SystemMessages[0], "No personas found")
	})
	
	t.Run("error from genie service", func(t *testing.T) {
		mockGenie.mockPersonasError = assert.AnError
		mockNotification.SystemMessages = []string{} // Reset
		
		err := cmd.Execute([]string{"list"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list personas")
	})
	
	t.Run("invalid subcommand", func(t *testing.T) {
		mockGenie.mockPersonasError = nil // Reset error
		
		err := cmd.Execute([]string{"invalid"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown subcommand")
	})
	
	t.Run("no arguments defaults to list", func(t *testing.T) {
		mockGenie.mockPersonas = []genie.Persona{&MockPersona{id: "test", name: "Test", source: "internal"}}
		mockNotification.SystemMessages = []string{} // Reset
		
		err := cmd.Execute([]string{})
		assert.NoError(t, err)
		assert.Len(t, mockNotification.SystemMessages, 1)
		assert.Contains(t, mockNotification.SystemMessages[0], "Available personas:")
	})
}

// MockPersona implements genie.Persona for testing
type MockPersona struct {
	id     string
	name   string
	source string
}

func (m *MockPersona) GetID() string     { return m.id }
func (m *MockPersona) GetName() string   { return m.name }
func (m *MockPersona) GetSource() string { return m.source }