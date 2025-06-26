package ctx

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEventBus is a mock implementation of the EventBus interface for testing.
type MockEventBus struct {
	mock.Mock
}

// Publish mocks the Publish method of the EventBus.
func (m *MockEventBus) Publish(topic string, event interface{}) {
	m.Called(topic, event)
}

// Subscribe mocks the Subscribe method of the EventBus.
func (m *MockEventBus) Subscribe(topic string, handler events.EventHandler) {
	m.Called(topic, handler)
}

func TestFileContextPartsProvider_New(t *testing.T) {
	mockBus := new(MockEventBus)
	// We expect Subscribe to be called once with "tool.executed"
	mockBus.On("Subscribe", "tool.executed", mock.Anything).Return().Once()

	provider := NewFileContextPartsProvider(mockBus)
	if provider == nil {
		t.Errorf("Expected a non-nil FileContextPartsProvider, got nil")
	}

	// Assert that the expected methods were called on the mock
	mockBus.AssertExpectations(t)
}

func TestFileContextPartsProvider_HandleToolExecutedEvent_ReadFile(t *testing.T) {
	mockBus := new(MockEventBus)
	// Expect Subscribe to be called, but we'll manually call the handler for testing
	mockBus.On("Subscribe", "tool.executed", mock.Anything).Return().Once()

	provider := NewFileContextPartsProvider(mockBus)
	assert.NotNil(t, provider)

	// Simulate a readFile event
	toolEvent1 := events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": "path/to/file1.txt"},
		Result:     map[string]any{"results": "content of file1"},
	}

	// Manually call the event handler
	provider.handleToolExecutedEvent(toolEvent1)

	// Assert that the file content is stored
	files := provider.GetStoredFiles()
	assert.Len(t, files, 1)
	assert.Equal(t, "content of file1", files["path/to/file1.txt"])

	// Assert the order
	orderedFiles := provider.GetOrderedFiles()
	assert.Len(t, orderedFiles, 1)
	assert.Equal(t, "path/to/file1.txt", orderedFiles[0])

	// Simulate another readFile event for a different file
	toolEvent2 := events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": "path/to/file2.txt"},
		Result:     map[string]any{"results": "content of file2"},
	}
	provider.handleToolExecutedEvent(toolEvent2)

	// Assert both files are stored
	files = provider.GetStoredFiles()
	assert.Len(t, files, 2)
	assert.Equal(t, "content of file1", files["path/to/file1.txt"])
	assert.Equal(t, "content of file2", files["path/to/file2.txt"])

	// Assert the order: file2 should be first
	orderedFiles = provider.GetOrderedFiles()
	assert.Len(t, orderedFiles, 2)
	assert.Equal(t, "path/to/file2.txt", orderedFiles[0])
	assert.Equal(t, "path/to/file1.txt", orderedFiles[1])

	// Simulate readFile event for file1 again (should move to front)
	provider.handleToolExecutedEvent(toolEvent1)

	// Assert content remains, but order changes
	files = provider.GetStoredFiles()
	assert.Len(t, files, 2)
	assert.Equal(t, "content of file1", files["path/to/file1.txt"])
	assert.Equal(t, "content of file2", files["path/to/file2.txt"])

	// Assert the new order: file1 should be first
	orderedFiles = provider.GetOrderedFiles()
	assert.Len(t, orderedFiles, 2)
	assert.Equal(t, "path/to/file1.txt", orderedFiles[0])
	assert.Equal(t, "path/to/file2.txt", orderedFiles[1])
}

func TestFileContextPartsProvider_GetPart(t *testing.T) {
	mockBus := new(MockEventBus)
	mockBus.On("Subscribe", "tool.executed", mock.Anything).Return().Once()
	provider := NewFileContextPartsProvider(mockBus)
	assert.NotNil(t, provider)

	// Simulate some file reads
	provider.handleToolExecutedEvent(events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": "fileA.txt"},
		Result:     map[string]any{"results": "Content of A"},
	})
	provider.handleToolExecutedEvent(events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": "fileB.txt"},
		Result:     map[string]any{"results": "Content of B"},
	})
	provider.handleToolExecutedEvent(events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": "fileC.txt"},
		Result:     map[string]any{"results": "Content of C"},
	})

	// fileC.txt, fileB.txt, fileA.txt
	expectedContent := "File: fileC.txt\n```\nContent of C\n```\n\nFile: fileB.txt\n```\nContent of B\n```\n\nFile: fileA.txt\n```\nContent of A\n```"

	part, err := provider.GetPart(context.Background()) // context.Background() is a fmt.Stringer
	assert.NoError(t, err)
	assert.Equal(t, "files", part.Key)
	assert.Equal(t, expectedContent, part.Content)

	// Read fileA.txt again to change order
	provider.handleToolExecutedEvent(events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": "fileA.txt"},
		Result:     map[string]any{"results": "Content of A"}, // Content remains same
	})

	// fileA.txt, fileC.txt, fileB.txt
	expectedContentAfterReorder := "File: fileA.txt\n```\nContent of A\n```\n\nFile: fileC.txt\n```\nContent of C\n```\n\nFile: fileB.txt\n```\nContent of B\n```"

	part, err = provider.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "files", part.Key)
	assert.Equal(t, expectedContentAfterReorder, part.Content)
}

func TestFileContextPartsProvider_ClearPart(t *testing.T) {
	mockBus := new(MockEventBus)
	mockBus.On("Subscribe", "tool.executed", mock.Anything).Return().Once()
	provider := NewFileContextPartsProvider(mockBus)
	assert.NotNil(t, provider)

	// Add some files
	provider.handleToolExecutedEvent(events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": "file1.txt"},
		Result:     map[string]any{"results": "Content 1"},
	})
	provider.handleToolExecutedEvent(events.ToolExecutedEvent{
		ToolName:   "readFile",
		Parameters: map[string]any{"file_path": "file2.txt"},
		Result:     map[string]any{"results": "Content 2"},
	})

	// Verify files are stored
	assert.Len(t, provider.GetStoredFiles(), 2)
	assert.Len(t, provider.GetOrderedFiles(), 2)

	// Clear all files
	err := provider.ClearPart()
	assert.NoError(t, err)

	// Verify files are cleared
	assert.Len(t, provider.GetStoredFiles(), 0)
	assert.Len(t, provider.GetOrderedFiles(), 0)

	// Verify GetPart returns empty content
	part, err := provider.GetPart(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "files", part.Key)
	assert.Equal(t, "", part.Content)
}
