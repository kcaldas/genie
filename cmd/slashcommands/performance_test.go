package slashcommands

import (
	"os"
	"path/filepath"
	"testing"
	
	"github.com/stretchr/testify/assert"
)

func TestManager_GetCommandNames_Performance(t *testing.T) {
	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "genie_perf_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	projectCommands := filepath.Join(tempDir, ".genie", "commands")
	os.MkdirAll(projectCommands, 0755)

	// Create several test command files
	commandFiles := []string{"command1.md", "command2.md", "command3.md", "command4.md", "command5.md"}
	for _, file := range commandFiles {
		os.WriteFile(filepath.Join(projectCommands, file), []byte("Test command content"), 0644)
	}

	// Mock getUserHomeDir function
	mockUserHomeDir := func() (string, error) {
		return filepath.Join(tempDir, "user_home"), nil
	}

	manager := NewManager()
	err = manager.DiscoverCommands(tempDir, mockUserHomeDir)
	assert.NoError(t, err)

	// Test that GetCommandNames returns cached results
	names1 := manager.GetCommandNames()
	names2 := manager.GetCommandNames()
	
	// Both calls should return the same slice (same underlying array)
	assert.Equal(t, names1, names2)
	assert.Len(t, names1, 5)
	
	// Verify all expected commands are present
	expectedCommands := []string{"command1", "command2", "command3", "command4", "command5"}
	for _, expected := range expectedCommands {
		assert.Contains(t, names1, expected)
	}
}

func TestManager_GetCommandNames_IsOptimized(t *testing.T) {
	manager := NewManager()
	
	// Initially empty
	names := manager.GetCommandNames()
	assert.Empty(t, names)
	
	// Manually add a command to test cache behavior
	manager.commands["test"] = SlashCommand{
		Name:        "test",
		Description: "Test command",
		Source:      "test",
	}
	
	// Names should still be empty because cache hasn't been rebuilt
	names = manager.GetCommandNames()
	assert.Empty(t, names, "Cache should not automatically update when commands map is modified directly")
	
	// Rebuild cache manually
	manager.rebuildCommandNames()
	names = manager.GetCommandNames()
	assert.Contains(t, names, "test", "After cache rebuild, command should be present")
}