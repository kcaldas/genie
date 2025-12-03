package genie

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/tools"
	"github.com/stretchr/testify/require"
)

// Mock tool for testing
type mockTool struct {
	name string
}

func newMockTool(name string) tools.Tool {
	return &mockTool{name: name}
}

func (t *mockTool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        t.name,
		Description: "A mock tool for testing",
		Parameters: &ai.Schema{
			Type:       ai.TypeObject,
			Properties: map[string]*ai.Schema{},
		},
	}
}

func (t *mockTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "mock"}, nil
	}
}

func (t *mockTool) FormatOutput(result map[string]interface{}) string {
	return "mock output"
}

func TestWithCustomTools(t *testing.T) {
	// Create mock tools
	tool1 := newMockTool("custom_tool_1")
	tool2 := newMockTool("custom_tool_2")

	// Apply option
	opts := applyOptions(WithCustomTools(tool1, tool2))

	// Verify tools were added
	require.Len(t, opts.CustomTools, 2)
	require.Equal(t, tool1, opts.CustomTools[0])
	require.Equal(t, tool2, opts.CustomTools[1])
}

func TestWithCustomTools_Multiple(t *testing.T) {
	// Create mock tools
	tool1 := newMockTool("custom_tool_1")
	tool2 := newMockTool("custom_tool_2")
	tool3 := newMockTool("custom_tool_3")

	// Apply multiple WithCustomTools options
	opts := applyOptions(
		WithCustomTools(tool1),
		WithCustomTools(tool2, tool3),
	)

	// Verify all tools were added
	require.Len(t, opts.CustomTools, 3)
	require.Equal(t, tool1, opts.CustomTools[0])
	require.Equal(t, tool2, opts.CustomTools[1])
	require.Equal(t, tool3, opts.CustomTools[2])
}

func TestWithToolRegistry(t *testing.T) {
	// Create a custom registry
	registry := tools.NewRegistry()
	tool1 := newMockTool("custom_tool")
	err := registry.Register(tool1)
	require.NoError(t, err)

	// Apply option
	opts := applyOptions(WithToolRegistry(registry))

	// Verify registry was set
	require.NotNil(t, opts.CustomRegistry)
	require.Equal(t, registry, opts.CustomRegistry)
}

func TestWithCustomRegistryFactory(t *testing.T) {
	// Create a factory function
	factoryCalled := false
	factory := func(eventBus events.EventBus, todoMgr tools.TodoManager) tools.Registry {
		factoryCalled = true
		registry := tools.NewDefaultRegistry(eventBus, todoMgr, nil) // nil skillManager for tests
		return registry
	}

	// Apply option
	opts := applyOptions(WithCustomRegistryFactory(factory))

	// Verify factory was set
	require.NotNil(t, opts.CustomRegistryFactory)

	// Call the factory to verify it works
	eventBus := events.NewEventBus()
	todoMgr := tools.NewTodoManager()
	registry := opts.CustomRegistryFactory(eventBus, todoMgr)

	require.True(t, factoryCalled)
	require.NotNil(t, registry)
}

func TestNewRegistryWithOptions_CustomRegistry(t *testing.T) {
	// Create a custom registry
	customRegistry := tools.NewRegistry()
	tool1 := newMockTool("custom_tool")
	err := customRegistry.Register(tool1)
	require.NoError(t, err)

	// Create options with custom registry
	opts := &GenieOptions{
		CustomRegistry: customRegistry,
	}

	// Call the provider function
	eventBus := events.NewEventBus()
	todoMgr := tools.NewTodoManager()
	var mcpClient tools.MCPClient = nil // nil MCP client

	registry, err := newRegistryWithOptions(eventBus, todoMgr, nil, mcpClient, opts) // nil skillManager for tests
	require.NoError(t, err)

	// Verify it returns the custom registry
	require.Equal(t, customRegistry, registry)

	// Verify the custom tool is accessible
	retrievedTool, exists := registry.Get("custom_tool")
	require.True(t, exists)
	require.Equal(t, tool1, retrievedTool)
}

func TestNewRegistryWithOptions_CustomFactory(t *testing.T) {
	// Create a factory that adds a custom tool
	tool1 := newMockTool("factory_tool")
	factory := func(eventBus events.EventBus, todoMgr tools.TodoManager) tools.Registry {
		registry := tools.NewDefaultRegistry(eventBus, todoMgr, nil) // nil skillManager for tests
		_ = registry.Register(tool1)
		return registry
	}

	// Create options with factory
	opts := &GenieOptions{
		CustomRegistryFactory: factory,
	}

	// Call the provider function
	eventBus := events.NewEventBus()
	todoMgr := tools.NewTodoManager()
	var mcpClient tools.MCPClient = nil

	registry, err := newRegistryWithOptions(eventBus, todoMgr, nil, mcpClient, opts) // nil skillManager for tests
	require.NoError(t, err)

	// Verify the factory was used and custom tool is present
	retrievedTool, exists := registry.Get("factory_tool")
	require.True(t, exists)
	require.Equal(t, tool1, retrievedTool)

	// Verify default tools are also present (from the factory)
	_, exists = registry.Get("readFile")
	require.True(t, exists)
}

func TestNewRegistryWithOptions_CustomTools(t *testing.T) {
	// Create custom tools
	tool1 := newMockTool("custom_tool_1")
	tool2 := newMockTool("custom_tool_2")

	// Create options with custom tools
	opts := &GenieOptions{
		CustomTools: []tools.Tool{tool1, tool2},
	}

	// Call the provider function
	eventBus := events.NewEventBus()
	todoMgr := tools.NewTodoManager()
	var mcpClient tools.MCPClient = nil

	registry, err := newRegistryWithOptions(eventBus, todoMgr, nil, mcpClient, opts) // nil skillManager for tests
	require.NoError(t, err)

	// Verify custom tools were added
	retrievedTool1, exists := registry.Get("custom_tool_1")
	require.True(t, exists)
	require.Equal(t, tool1, retrievedTool1)

	retrievedTool2, exists := registry.Get("custom_tool_2")
	require.True(t, exists)
	require.Equal(t, tool2, retrievedTool2)

	// Verify default tools are also present
	_, exists = registry.Get("readFile")
	require.True(t, exists)
}

func TestNewRegistryWithOptions_DefaultRegistry(t *testing.T) {
	// Create empty options
	opts := &GenieOptions{}

	// Call the provider function
	eventBus := events.NewEventBus()
	todoMgr := tools.NewTodoManager()
	var mcpClient tools.MCPClient = nil

	registry, err := newRegistryWithOptions(eventBus, todoMgr, nil, mcpClient, opts) // nil skillManager for tests
	require.NoError(t, err)

	// Verify default tools are present
	_, exists := registry.Get("readFile")
	require.True(t, exists)

	_, exists = registry.Get("bash")
	require.True(t, exists)

	_, exists = registry.Get("writeFile")
	require.True(t, exists)
}

func TestNewRegistryWithOptions_PriorityOrder(t *testing.T) {
	// Test that CustomRegistry takes priority over CustomRegistryFactory and CustomTools

	customRegistry := tools.NewRegistry()
	tool1 := newMockTool("priority_test")
	_ = customRegistry.Register(tool1)

	factory := func(eventBus events.EventBus, todoMgr tools.TodoManager) tools.Registry {
		t.Fatal("Factory should not be called when CustomRegistry is set")
		return nil
	}

	opts := &GenieOptions{
		CustomRegistry:        customRegistry,
		CustomRegistryFactory: factory,
		CustomTools:           []tools.Tool{newMockTool("should_not_be_added")},
	}

	eventBus := events.NewEventBus()
	todoMgr := tools.NewTodoManager()
	var mcpClient tools.MCPClient = nil

	registry, err := newRegistryWithOptions(eventBus, todoMgr, nil, mcpClient, opts) // nil skillManager for tests
	require.NoError(t, err)

	// Should return the custom registry
	require.Equal(t, customRegistry, registry)
}

func TestNewRegistryWithOptions_FactoryOverCustomTools(t *testing.T) {
	// Test that CustomRegistryFactory takes priority over CustomTools

	factoryCalled := false
	factory := func(eventBus events.EventBus, todoMgr tools.TodoManager) tools.Registry {
		factoryCalled = true
		return tools.NewDefaultRegistry(eventBus, todoMgr, nil) // nil skillManager for tests
	}

	opts := &GenieOptions{
		CustomRegistryFactory: factory,
		CustomTools:           []tools.Tool{newMockTool("should_not_be_processed")},
	}

	eventBus := events.NewEventBus()
	todoMgr := tools.NewTodoManager()
	var mcpClient tools.MCPClient = nil

	registry, err := newRegistryWithOptions(eventBus, todoMgr, nil, mcpClient, opts) // nil skillManager for tests
	require.NoError(t, err)
	require.True(t, factoryCalled)

	// CustomTools should be ignored when factory is present
	_, exists := registry.Get("should_not_be_processed")
	require.False(t, exists)
}

func TestNewRegistryWithOptions_DuplicateToolName(t *testing.T) {
	// Test that duplicate tool names cause a loud failure

	// Create a custom tool with the same name as a default tool
	duplicateTool := newMockTool("readFile") // Same name as default tool

	opts := &GenieOptions{
		CustomTools: []tools.Tool{duplicateTool},
	}

	eventBus := events.NewEventBus()
	todoMgr := tools.NewTodoManager()
	var mcpClient tools.MCPClient = nil

	// Should return error for duplicate tool name
	_, err := newRegistryWithOptions(eventBus, todoMgr, nil, mcpClient, opts) // nil skillManager for tests
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to register custom tool")
	require.Contains(t, err.Error(), "readFile")
}

func TestApplyOptions(t *testing.T) {
	tool1 := newMockTool("tool1")
	tool2 := newMockTool("tool2")
	registry := tools.NewRegistry()

	opts := applyOptions(
		WithCustomTools(tool1),
		WithToolRegistry(registry),
		WithCustomTools(tool2),
	)

	// Verify all options were applied
	require.Len(t, opts.CustomTools, 2)
	require.Equal(t, registry, opts.CustomRegistry)
}

func TestApplyOptions_Empty(t *testing.T) {
	opts := applyOptions()

	require.Nil(t, opts.CustomRegistry)
	require.Nil(t, opts.CustomRegistryFactory)
	require.Nil(t, opts.CustomTools)
}
