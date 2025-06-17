package history

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatHistoryManager_AddCommand(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	historyFile := filepath.Join(tempDir, "history")
	
	manager := NewChatHistoryManager(historyFile)
	
	// Test adding commands
	manager.AddCommand("first command")
	manager.AddCommand("second command") 
	manager.AddCommand("third command")
	
	// Get history and verify
	history := manager.GetHistory()
	assert.Len(t, history, 3)
	assert.Equal(t, "first command", history[0])
	assert.Equal(t, "second command", history[1])
	assert.Equal(t, "third command", history[2])
}

func TestChatHistoryManager_AvoidDuplicates(t *testing.T) {
	tempDir := t.TempDir()
	historyFile := filepath.Join(tempDir, "history")
	
	manager := NewChatHistoryManager(historyFile)
	
	// Add same command multiple times
	manager.AddCommand("duplicate command")
	manager.AddCommand("duplicate command")
	manager.AddCommand("different command")
	manager.AddCommand("duplicate command")
	
	// Should only have 2 unique commands, most recent last
	history := manager.GetHistory()
	assert.Len(t, history, 2)
	assert.Equal(t, "different command", history[0])
	assert.Equal(t, "duplicate command", history[1]) // Most recent
}

func TestChatHistoryManager_LimitTo50Entries(t *testing.T) {
	tempDir := t.TempDir()
	historyFile := filepath.Join(tempDir, "history")
	
	manager := NewChatHistoryManager(historyFile)
	
	// Add 60 commands
	for i := 0; i < 60; i++ {
		manager.AddCommand(fmt.Sprintf("command %d", i))
	}
	
	// Should only keep last 50
	history := manager.GetHistory()
	assert.Len(t, history, 50)
	assert.Equal(t, "command 10", history[0]) // First kept command
	assert.Equal(t, "command 59", history[49]) // Last command
}

func TestChatHistoryManager_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	historyFile := filepath.Join(tempDir, "history")
	
	// Create first manager and add commands
	manager1 := NewChatHistoryManager(historyFile)
	manager1.AddCommand("persistent command 1")
	manager1.AddCommand("persistent command 2")
	manager1.Save()
	
	// Create second manager and load
	manager2 := NewChatHistoryManager(historyFile)
	err := manager2.Load()
	require.NoError(t, err)
	
	// Verify commands were loaded
	history := manager2.GetHistory()
	assert.Len(t, history, 2)
	assert.Equal(t, "persistent command 1", history[0])
	assert.Equal(t, "persistent command 2", history[1])
}

func TestChatHistoryManager_LoadNonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	historyFile := filepath.Join(tempDir, "nonexistent", "history")
	
	manager := NewChatHistoryManager(historyFile)
	err := manager.Load()
	
	// Should not error when file doesn't exist
	require.NoError(t, err)
	assert.Len(t, manager.GetHistory(), 0)
}

func TestChatHistoryManager_AutoSaveOnAdd(t *testing.T) {
	tempDir := t.TempDir()
	historyFile := filepath.Join(tempDir, "history")
	
	// Create manager and add command
	manager1 := NewChatHistoryManager(historyFile)
	manager1.AddCommand("auto saved command")
	
	// Create new manager and load
	manager2 := NewChatHistoryManager(historyFile)
	err := manager2.Load()
	require.NoError(t, err)
	
	// Command should be persisted automatically
	history := manager2.GetHistory()
	assert.Len(t, history, 1)
	assert.Equal(t, "auto saved command", history[0])
}

func TestChatHistoryManager_CreateDirectoryIfNeeded(t *testing.T) {
	tempDir := t.TempDir()
	nestedDir := filepath.Join(tempDir, "nested", "deep", "directory")
	historyFile := filepath.Join(nestedDir, "history")
	
	manager := NewChatHistoryManager(historyFile)
	manager.AddCommand("test command")
	
	// Directory should be created automatically
	assert.DirExists(t, nestedDir)
	assert.FileExists(t, historyFile)
}

func TestChatHistoryManager_ProjectSpecificHistory(t *testing.T) {
	// Create two different project directories
	project1Dir := t.TempDir()
	project2Dir := t.TempDir()
	
	// Create history managers for each project
	history1File := filepath.Join(project1Dir, ".genie", "history")
	history2File := filepath.Join(project2Dir, ".genie", "history")
	
	manager1 := NewChatHistoryManager(history1File)
	manager2 := NewChatHistoryManager(history2File)
	
	// Add different commands to each
	manager1.AddCommand("project1 command")
	manager2.AddCommand("project2 command")
	
	// Verify each has its own distinct history
	history1 := manager1.GetHistory()
	history2 := manager2.GetHistory()
	
	assert.Len(t, history1, 1)
	assert.Len(t, history2, 1)
	assert.Equal(t, "project1 command", history1[0])
	assert.Equal(t, "project2 command", history2[0])
	
	// Verify files exist in correct locations
	assert.FileExists(t, history1File)
	assert.FileExists(t, history2File)
	assert.DirExists(t, filepath.Join(project1Dir, ".genie"))
	assert.DirExists(t, filepath.Join(project2Dir, ".genie"))
}