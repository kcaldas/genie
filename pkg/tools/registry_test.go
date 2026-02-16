package tools

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
)

// MockTool implements Tool for testing
type MockTool struct {
	name string
}

func (m *MockTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        m.name,
		Description: "Mock tool for testing",
		Parameters: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"input": {
					Type:        ai.TypeString,
					Description: "Test input",
				},
			},
		},
	}
}

func (m *MockTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		return map[string]any{"result": "mock result"}, nil
	}
}

func (m *MockTool) FormatOutput(result map[string]interface{}) string {
	return "Mock formatted output"
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	assert.NotNil(t, registry)
	assert.Empty(t, registry.GetAll())
	assert.Empty(t, registry.Names())
}

func TestNewDefaultRegistry(t *testing.T) {
	// Create mock dependencies
	todoManager := NewTodoManager()
	registry := NewDefaultRegistry(nil, todoManager, nil, nil) // nil eventBus, skillManager, mcpClient for tests
	assert.NotNil(t, registry)
	
	tools := registry.GetAll()
	assert.Greater(t, len(tools), 0, "Default registry should have tools")
	
	names := registry.Names()
	assert.Greater(t, len(names), 0, "Default registry should have tool names")
	
	// Check for expected standard tools
	expectedTools := []string{"listFiles", "findFiles", "readFile", "searchInFiles", "bash"}
	for _, expected := range expectedTools {
		_, exists := registry.Get(expected)
		assert.True(t, exists, "Should have standard tool: %s", expected)
	}
	
	// Check for todo tools
	
	todoWriteTool, exists := registry.Get("TodoWrite")
	assert.True(t, exists, "Should have TodoWrite tool")
	assert.NotNil(t, todoWriteTool)
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()
	mockTool := &MockTool{name: "testTool"}
	
	// Test successful registration
	err := registry.Register(mockTool)
	assert.NoError(t, err)
	
	// Verify tool was registered
	tool, exists := registry.Get("testTool")
	assert.True(t, exists)
	assert.Equal(t, mockTool, tool)
	
	// Test duplicate registration
	err = registry.Register(mockTool)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_RegisterNilTool(t *testing.T) {
	registry := NewRegistry()
	
	err := registry.Register(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot register nil tool")
}

func TestRegistry_RegisterToolWithEmptyName(t *testing.T) {
	registry := NewRegistry()
	mockTool := &MockTool{name: ""}
	
	err := registry.Register(mockTool)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool name cannot be empty")
}

func TestRegistry_GetAll(t *testing.T) {
	registry := NewRegistry()
	
	// Initially empty
	tools := registry.GetAll()
	assert.Empty(t, tools)
	
	// Add some tools
	tool1 := &MockTool{name: "tool1"}
	tool2 := &MockTool{name: "tool2"}
	
	registry.Register(tool1)
	registry.Register(tool2)
	
	tools = registry.GetAll()
	assert.Len(t, tools, 2)
	
	// Verify we get copies, not references to internal state
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Declaration().Name] = true
	}
	assert.True(t, toolNames["tool1"])
	assert.True(t, toolNames["tool2"])
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()
	mockTool := &MockTool{name: "testTool"}
	
	// Test getting non-existent tool
	tool, exists := registry.Get("nonExistent")
	assert.False(t, exists)
	assert.Nil(t, tool)
	
	// Register and get tool
	registry.Register(mockTool)
	tool, exists = registry.Get("testTool")
	assert.True(t, exists)
	assert.Equal(t, mockTool, tool)
}

func TestRegistry_Names(t *testing.T) {
	registry := NewRegistry()
	
	// Initially empty
	names := registry.Names()
	assert.Empty(t, names)
	
	// Add tools
	tool1 := &MockTool{name: "tool1"}
	tool2 := &MockTool{name: "tool2"}
	
	registry.Register(tool1)
	registry.Register(tool2)
	
	names = registry.Names()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "tool1")
	assert.Contains(t, names, "tool2")
}

func TestRegistry_ThreadSafety(t *testing.T) {
	registry := NewRegistry()
	var wg sync.WaitGroup
	
	// Test concurrent registrations
	numGoroutines := 10
	wg.Add(numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			tool := &MockTool{name: fmt.Sprintf("tool%d", id)}
			registry.Register(tool)
		}(i)
	}
	
	wg.Wait()
	
	// Verify all tools were registered
	tools := registry.GetAll()
	assert.Len(t, tools, numGoroutines)
	
	names := registry.Names()
	assert.Len(t, names, numGoroutines)
	
	// Test concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			toolName := fmt.Sprintf("tool%d", id)
			tool, exists := registry.Get(toolName)
			assert.True(t, exists)
			assert.NotNil(t, tool)
		}(i)
	}
	
	wg.Wait()
}