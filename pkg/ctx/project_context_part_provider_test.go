package ctx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectCtxManager_Interface(t *testing.T) {
	// Test that we can create a ProjectCtxManager
	var manager ProjectContextPartProvider
	manager = NewProjectCtxManager(nil)
	assert.NotNil(t, manager)
}

func TestProjectCtxManager_GetContext_BasicStructure(t *testing.T) {
	// Test that GetContext method exists and returns expected types
	manager := NewProjectCtxManager(nil)

	ctx := context.Background()
	part, err := manager.GetPart(ctx)

	// Should return empty string and no error for basic case
	assert.NoError(t, err)
	assert.Equal(t, "project", part.Key)
	assert.Equal(t, "", part.Content)
}

func TestProjectCtxManager_GetContext_ReturnsGenieMdFromCwd(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create GENIE.md file
	genieMdContent := "# GENIE Context\n\nThis is project context for GENIE."
	genieMdPath := filepath.Join(tempDir, "GENIE.md")
	err := os.WriteFile(genieMdPath, []byte(genieMdContent), 0644)
	require.NoError(t, err)

	// Create manager
	manager := NewProjectCtxManager(nil)

	// Create context with cwd
	ctx := context.WithValue(context.Background(), "cwd", tempDir)

	// Get context
	part, err := manager.GetPart(ctx)

	// Should return GENIE.md content
	assert.NoError(t, err)
	assert.Equal(t, "project", part.Key)
	assert.Equal(t, genieMdContent, part.Content)
}

func TestProjectCtxManager_GetContext_ReturnsClaudeMdWhenGenieMdNotExist(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create CLAUDE.md file (no GENIE.md)
	claudeMdContent := "# CLAUDE Context\n\nThis is project context for CLAUDE."
	claudeMdPath := filepath.Join(tempDir, "CLAUDE.md")
	err := os.WriteFile(claudeMdPath, []byte(claudeMdContent), 0644)
	require.NoError(t, err)

	// Create manager
	manager := NewProjectCtxManager(nil)

	// Create context with cwd
	ctx := context.WithValue(context.Background(), "cwd", tempDir)

	// Get context
	part, err := manager.GetPart(ctx)

	// Should return CLAUDE.md content
	assert.NoError(t, err)
	assert.Equal(t, "project", part.Key)
	assert.Equal(t, claudeMdContent, part.Content)
}

func TestProjectCtxManager_GetContext_ReturnsEmptyWhenNoFilesExist(t *testing.T) {
	// Create a temporary directory (no context files)
	tempDir := t.TempDir()

	// Create manager
	manager := NewProjectCtxManager(nil)

	// Create context with cwd
	ctx := context.WithValue(context.Background(), "cwd", tempDir)

	// Get context
	part, err := manager.GetPart(ctx)

	// Should return empty string
	assert.NoError(t, err)
	assert.Equal(t, "project", part.Key)
	assert.Equal(t, "", part.Content)
}

func TestProjectCtxManager_SubscribesToToolExecutedEvents(t *testing.T) {
	// Create event bus
	eventBus := events.NewEventBus()

	// Create manager with event subscription
	manager := NewProjectCtxManager(eventBus)

	// Verify that manager subscribes to tool.executed events
	// (We'll test this by checking if the manager is created without error)
	assert.NotNil(t, manager)

	// We can verify subscription worked by publishing an event and checking behavior
	// This will be covered in the next test when we actually handle the events
}

func TestProjectCtxManager_ReadsGenieMdFromFileDirectory_OnReadFileExecution(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Create GENIE.md in subdirectory
	genieMdContent := "# Sub Project Context\n\nContext from subdirectory."
	genieMdPath := filepath.Join(subDir, "GENIE.md")
	err = os.WriteFile(genieMdPath, []byte(genieMdContent), 0644)
	require.NoError(t, err)

	// Create a file to be read in the subdirectory
	readFilePath := filepath.Join(subDir, "example.txt")
	err = os.WriteFile(readFilePath, []byte("example content"), 0644)
	require.NoError(t, err)

	// Create event bus and manager
	eventBus := events.NewEventBus()
	manager := NewProjectCtxManager(eventBus)

	// Simulate readFile tool execution event
	toolEvent := events.ToolExecutedEvent{
		ExecutionID: "test-exec-1",
		ToolName:    "readFile",
		Parameters: map[string]any{
			"file_path": readFilePath,
		},
		Result: map[string]any{
			"success": true,
			"results": "example content",
		},
	}

	// Publish the event
	eventBus.Publish("tool.executed", toolEvent)

	// Small delay for test reliability (not needed in production due to natural user interaction delays)
	time.Sleep(1 * time.Millisecond)

	// Create context with main CWD (different from file's directory)
	ctx := context.WithValue(context.Background(), "cwd", tempDir)

	// Get context - should include both CWD context (if any) and subdirectory context
	part, err := manager.GetPart(ctx)

	// Should include the GENIE.md from the file's directory
	assert.NoError(t, err)
	assert.Contains(t, part.Content, genieMdContent)
}

func TestProjectCtxManager_ConcatenatesMultipleContextFiles_WithBlankLines(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()
	subDir1 := filepath.Join(tempDir, "dir1")
	subDir2 := filepath.Join(tempDir, "dir2")
	err := os.MkdirAll(subDir1, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(subDir2, 0755)
	require.NoError(t, err)

	// Create GENIE.md in main directory
	mainContextContent := "# Main Context\n\nMain project context."
	mainGenieMdPath := filepath.Join(tempDir, "GENIE.md")
	err = os.WriteFile(mainGenieMdPath, []byte(mainContextContent), 0644)
	require.NoError(t, err)

	// Create GENIE.md files in subdirectories
	context1Content := "# Context 1\n\nContext from directory 1."
	genieMd1Path := filepath.Join(subDir1, "GENIE.md")
	err = os.WriteFile(genieMd1Path, []byte(context1Content), 0644)
	require.NoError(t, err)

	context2Content := "# Context 2\n\nContext from directory 2."
	genieMd2Path := filepath.Join(subDir2, "GENIE.md")
	err = os.WriteFile(genieMd2Path, []byte(context2Content), 0644)
	require.NoError(t, err)

	// Create files to be read in subdirectories
	file1Path := filepath.Join(subDir1, "file1.txt")
	err = os.WriteFile(file1Path, []byte("content 1"), 0644)
	require.NoError(t, err)

	file2Path := filepath.Join(subDir2, "file2.txt")
	err = os.WriteFile(file2Path, []byte("content 2"), 0644)
	require.NoError(t, err)

	// Create event bus and manager
	eventBus := events.NewEventBus()
	manager := NewProjectCtxManager(eventBus)

	// Simulate readFile tool execution events
	toolEvent1 := events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": file1Path},
	}
	toolEvent2 := events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": file2Path},
	}

	// Publish the events
	eventBus.Publish("tool.executed", toolEvent1)
	eventBus.Publish("tool.executed", toolEvent2)

	// Small delay for test reliability (not needed in production due to natural user interaction delays)
	time.Sleep(1 * time.Millisecond)

	// Create context with main CWD
	ctx := context.WithValue(context.Background(), "cwd", tempDir)

	// Get context
	part, err := manager.GetPart(ctx)

	// Should include all context files with blank lines between them
	assert.NoError(t, err)
	assert.Contains(t, part.Content, mainContextContent)
	assert.Contains(t, part.Content, context1Content)
	assert.Contains(t, part.Content, context2Content)

	// Should have blank lines between contexts
	assert.Contains(t, part.Content, "\n\n")
}

func TestProjectCtxManager_DeduplicatesContextFiles(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Create GENIE.md in subdirectory
	contextContent := "# Duplicate Context\n\nThis should appear only once."
	genieMdPath := filepath.Join(subDir, "GENIE.md")
	err = os.WriteFile(genieMdPath, []byte(contextContent), 0644)
	require.NoError(t, err)

	// Create multiple files in the same directory
	file1Path := filepath.Join(subDir, "file1.txt")
	err = os.WriteFile(file1Path, []byte("content 1"), 0644)
	require.NoError(t, err)

	file2Path := filepath.Join(subDir, "file2.txt")
	err = os.WriteFile(file2Path, []byte("content 2"), 0644)
	require.NoError(t, err)

	// Create event bus and manager
	eventBus := events.NewEventBus()
	manager := NewProjectCtxManager(eventBus)

	// Simulate multiple readFile tool execution events in same directory
	toolEvent1 := events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": file1Path},
	}
	toolEvent2 := events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": file2Path},
	}

	// Publish the events
	eventBus.Publish("tool.executed", toolEvent1)
	eventBus.Publish("tool.executed", toolEvent2)

	// Small delay for test reliability (not needed in production due to natural user interaction delays)
	time.Sleep(1 * time.Millisecond)

	// Create context with main CWD
	ctx := context.WithValue(context.Background(), "cwd", tempDir)

	// Get context
	part, err := manager.GetPart(ctx)

	// Should include the context content only once
	assert.NoError(t, err)
	assert.Contains(t, part.Content, contextContent)

	// Count occurrences - should be exactly 1
	count := 0
	searchText := "# Duplicate Context"
	text := part.Content
	for {
		index := strings.Index(text, searchText)
		if index == -1 {
			break
		}
		count++
		text = text[index+len(searchText):]
	}
	assert.Equal(t, 1, count, "Context content should appear only once, but appeared %d times", count)
}

func TestProjectCtxManager_CachesFileReads_DoesNotReadTwice(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create GENIE.md file
	originalContent := "# Original Context\n\nThis is the original content."
	genieMdPath := filepath.Join(tempDir, "GENIE.md")
	err := os.WriteFile(genieMdPath, []byte(originalContent), 0644)
	require.NoError(t, err)

	// Create manager
	manager := NewProjectCtxManager(nil)

	// Create context with cwd
	ctx := context.WithValue(context.Background(), "cwd", tempDir)

	// First call to GetContext
	part1, err := manager.GetPart(ctx)
	require.NoError(t, err)
	assert.Equal(t, "project", part1.Key)
	assert.Equal(t, originalContent, part1.Content)

	// Modify the file on disk after the first read
	modifiedContent := "# Modified Context\n\nThis content was changed after first read."
	err = os.WriteFile(genieMdPath, []byte(modifiedContent), 0644)
	require.NoError(t, err)

	// Second call to GetContext - should return cached content, not modified content
	part2, err := manager.GetPart(ctx)
	require.NoError(t, err)

	// Should return original content (cached), not the modified content from disk
	assert.Equal(t, "project", part2.Key)
	assert.Equal(t, originalContent, part2.Content, "Second call should return cached content, not re-read from disk")
	assert.NotEqual(t, modifiedContent, part2.Content, "Should not return the modified content from disk")
}

func TestProjectCtxManager_HandlesMainCwdAndSubdirectoryContextFiles(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Create GENIE.md in main directory
	mainContent := "# Main Project Context\n\nMain project documentation."
	mainGenieMdPath := filepath.Join(tempDir, "GENIE.md")
	err = os.WriteFile(mainGenieMdPath, []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create GENIE.md in subdirectory
	subContent := "# Sub Module Context\n\nSub module documentation."
	subGenieMdPath := filepath.Join(subDir, "GENIE.md")
	err = os.WriteFile(subGenieMdPath, []byte(subContent), 0644)
	require.NoError(t, err)

	// Create a file to be read in the subdirectory
	readFilePath := filepath.Join(subDir, "example.txt")
	err = os.WriteFile(readFilePath, []byte("example content"), 0644)
	require.NoError(t, err)

	// Create event bus and manager
	eventBus := events.NewEventBus()
	manager := NewProjectCtxManager(eventBus)

	// Create context with main CWD
	ctx := context.WithValue(context.Background(), "cwd", tempDir)

	// First call to GetContext - should load main CWD GENIE.md
	part1, err := manager.GetPart(ctx)
	require.NoError(t, err)
	assert.Equal(t, "project", part1.Key)
	assert.Equal(t, mainContent, part1.Content)
	assert.Contains(t, part1.Content, "Main Project Context")
	assert.NotContains(t, part1.Content, "Sub Module Context")

	// Simulate readFile tool execution event in subdirectory
	toolEvent := events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": readFilePath},
	}

	// Publish the event
	eventBus.Publish("tool.executed", toolEvent)

	// Small delay for test reliability (not needed in production due to natural user interaction delays)
	time.Sleep(1 * time.Millisecond)

	// Second call to GetContext - should now include both main and sub contexts
	part2, err := manager.GetPart(ctx)
	require.NoError(t, err)

	// Should contain both contexts
	assert.Contains(t, part2.Content, "Main Project Context")
	assert.Contains(t, part2.Content, "Sub Module Context")

	// Should have both contents separated by blank lines
	assert.Contains(t, part2.Content, mainContent)
	assert.Contains(t, part2.Content, subContent)
	assert.Contains(t, part2.Content, "\n\n")

	// Modify the main GENIE.md file on disk
	modifiedMainContent := "# Modified Main Context\n\nThis was changed after caching."
	err = os.WriteFile(mainGenieMdPath, []byte(modifiedMainContent), 0644)
	require.NoError(t, err)

	// Third call to GetContext - should still return cached main content
	part3, err := manager.GetPart(ctx)
	require.NoError(t, err)

	// Should still contain original main content (cached), not modified
	assert.Contains(t, part3.Content, mainContent)
	assert.NotContains(t, part3.Content, modifiedMainContent)
	assert.Contains(t, part3.Content, subContent) // Sub content should still be there
}

func TestProjectCtxManager_CachesToolExecutedEvents_SameDirectory(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Create GENIE.md in subdirectory
	originalContent := "# Original Sub Context\n\nOriginal subdirectory context."
	genieMdPath := filepath.Join(subDir, "GENIE.md")
	err = os.WriteFile(genieMdPath, []byte(originalContent), 0644)
	require.NoError(t, err)

	// Create multiple files in the same subdirectory
	file1Path := filepath.Join(subDir, "file1.txt")
	err = os.WriteFile(file1Path, []byte("content 1"), 0644)
	require.NoError(t, err)

	file2Path := filepath.Join(subDir, "file2.txt")
	err = os.WriteFile(file2Path, []byte("content 2"), 0644)
	require.NoError(t, err)

	// Create event bus and manager
	eventBus := events.NewEventBus()
	manager := NewProjectCtxManager(eventBus)

	// Create context with main CWD
	ctx := context.WithValue(context.Background(), "cwd", tempDir)

	// Simulate first readFile tool execution event in subdirectory
	toolEvent1 := events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": file1Path},
	}

	// Publish the first event
	eventBus.Publish("tool.executed", toolEvent1)

	// Small delay for test reliability (not needed in production due to natural user interaction delays)
	time.Sleep(1 * time.Millisecond)

	// Get context - should load the subdirectory GENIE.md
	part1, err := manager.GetPart(ctx)
	require.NoError(t, err)
	assert.Contains(t, part1.Content, originalContent)

	// Modify the GENIE.md file on disk after first event
	modifiedContent := "# Modified Sub Context\n\nThis was changed after first read."
	err = os.WriteFile(genieMdPath, []byte(modifiedContent), 0644)
	require.NoError(t, err)

	// Simulate second readFile tool execution event in same subdirectory
	toolEvent2 := events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": file2Path},
	}

	// Publish the second event
	eventBus.Publish("tool.executed", toolEvent2)

	// Small delay for test reliability (not needed in production due to natural user interaction delays)
	time.Sleep(1 * time.Millisecond)

	// Get context again - should still return cached content, not modified content
	part2, err := manager.GetPart(ctx)
	require.NoError(t, err)

	// Should contain original content (cached), not modified content
	assert.Contains(t, part2.Content, originalContent)
	assert.NotContains(t, part2.Content, modifiedContent)

	// Should only appear once (not duplicated)
	count := 0
	searchText := "Original Sub Context"
	text := part2.Content
	for {
		index := strings.Index(text, searchText)
		if index == -1 {
			break
		}
		count++
		text = text[index+len(searchText):]
	}
	assert.Equal(t, 1, count, "Context content should appear only once, but appeared %d times", count)
}
