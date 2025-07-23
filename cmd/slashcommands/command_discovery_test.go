package slashcommands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscoverCommands(t *testing.T) {
	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "genie_test_commands")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up after test

	projectGenieCommands := filepath.Join(tempDir, ".genie", "commands")
	projectClaudeCommands := filepath.Join(tempDir, ".claude", "commands")
	userGenieCommands := filepath.Join(tempDir, "user_home", ".genie", "commands")
	userClaudeCommands := filepath.Join(tempDir, "user_home", ".claude", "commands")

	os.MkdirAll(projectGenieCommands, 0755)
	os.MkdirAll(filepath.Join(projectGenieCommands, "sub"), 0755)
	os.MkdirAll(projectClaudeCommands, 0755)
	os.MkdirAll(userGenieCommands, 0755)
	os.MkdirAll(filepath.Join(userClaudeCommands, "frontend"), 0755)
	os.MkdirAll(userClaudeCommands, 0755)

	// Create dummy command files
	// Project Genie commands
	os.WriteFile(filepath.Join(projectGenieCommands, "test_cmd.md"), []byte("Test command content"), 0644)
	os.WriteFile(filepath.Join(projectGenieCommands, "sub", "sub_cmd.md"), []byte("Sub command content"), 0644)

	// Project Claude commands
	os.WriteFile(filepath.Join(projectClaudeCommands, "cli_cmd.md"), []byte("CLI command content"), 0644)

	// User Genie commands (simulated home directory)
	os.WriteFile(filepath.Join(userGenieCommands, "user_cmd.md"), []byte("User command content"), 0644)

	// User Claude commands (simulated home directory)
	os.WriteFile(filepath.Join(userClaudeCommands, "user_cli_cmd.md"), []byte("User CLI command content"), 0644)
	os.WriteFile(filepath.Join(userClaudeCommands, "frontend", "component.md"), []byte("Frontend component command"), 0644)

	// Mock getUserHomeDir function
	mockUserHomeDir := func() (string, error) {
		return filepath.Join(tempDir, "user_home"), nil
	}

	m := NewManager()
	err = m.DiscoverCommands(tempDir, mockUserHomeDir)
	assert.NoError(t, err)

	assert.Equal(t, 6, len(m.commands))

	cmd, ok := m.commands["test_cmd"]
	assert.True(t, ok)
	assert.Equal(t, "Test command content", cmd.Description)
	assert.Equal(t, "project", cmd.Source)

	cmd, ok = m.commands["sub:sub_cmd"]
	assert.True(t, ok)
	assert.Equal(t, "Sub command content", cmd.Description)
	assert.Equal(t, "project", cmd.Source)

	cmd, ok = m.commands["cli_cmd"]
	assert.True(t, ok)
	assert.Equal(t, "CLI command content", cmd.Description)
	assert.Equal(t, "project", cmd.Source)

	cmd, ok = m.commands["user_cmd"]
	assert.True(t, ok)
	assert.Equal(t, "User command content", cmd.Description)
	assert.Equal(t, "user", cmd.Source)

	cmd, ok = m.commands["frontend:component"]
	assert.True(t, ok)
	assert.Equal(t, "Frontend component command", cmd.Description)
	assert.Equal(t, "user", cmd.Source)

	// Test conflict scenario (not supported, but ensuring it doesn't crash)
	// Create a conflict (e.g., project and user have same command name)
	os.WriteFile(filepath.Join(projectGenieCommands, "user_cmd.md"), []byte("Project user command content"), 0644)

	m = NewManager() // Reset manager for re-discovery
	err = m.DiscoverCommands(tempDir, mockUserHomeDir)
	assert.NoError(t, err)

	// The existing user command should take precedence or one should be chosen (current implementation just overwrites)
	cmd, ok = m.commands["user_cmd"]
	assert.True(t, ok)
	assert.Equal(t, "User command content", cmd.Description) // Last one found wins
}
