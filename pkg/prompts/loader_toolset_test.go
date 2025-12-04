package prompts

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/tools"
)

// MockTool is already defined in loader_test.go, so we'll reuse it

// MockRegistry for testing
type MockRegistry struct {
	tools    map[string]tools.Tool
	toolSets map[string][]tools.Tool
}

func NewMockRegistry() *MockRegistry {
	return &MockRegistry{
		tools:    make(map[string]tools.Tool),
		toolSets: make(map[string][]tools.Tool),
	}
}

func (m *MockRegistry) Register(tool tools.Tool) error {
	m.tools[tool.Declaration().Name] = tool
	return nil
}

func (m *MockRegistry) GetAll() []tools.Tool {
	var result []tools.Tool
	for _, tool := range m.tools {
		result = append(result, tool)
	}
	return result
}

func (m *MockRegistry) Get(name string) (tools.Tool, bool) {
	tool, exists := m.tools[name]
	return tool, exists
}

func (m *MockRegistry) Names() []string {
	var names []string
	for name := range m.tools {
		names = append(names, name)
	}
	return names
}

func (m *MockRegistry) RegisterToolSet(setName string, toolSet []tools.Tool) error {
	m.toolSets[setName] = toolSet
	return nil
}

func (m *MockRegistry) GetToolSet(setName string) ([]tools.Tool, bool) {
	toolSet, exists := m.toolSets[setName]
	return toolSet, exists
}

func (m *MockRegistry) GetToolSetNames() []string {
	var names []string
	for name := range m.toolSets {
		names = append(names, name)
	}
	return names
}

func (m *MockRegistry) Init(workingDir string) error {
	return nil
}

func TestAddToolsWithToolSets(t *testing.T) {
	// Create mock registry
	registry := NewMockRegistry()
	
	// Create mock tools
	tool1 := &MockTool{name: "readFile"}
	tool2 := &MockTool{name: "writeFile"}
	tool3 := &MockTool{name: "mcp_tool1"}
	tool4 := &MockTool{name: "mcp_tool2"}
	
	// Register individual tools
	registry.Register(tool1)
	registry.Register(tool2)
	registry.Register(tool3)
	registry.Register(tool4)
	
	// Register toolSets
	registry.RegisterToolSet("server1", []tools.Tool{tool3, tool4})
	
	// Create loader
	loader := &DefaultLoader{
		Publisher:    &events.NoOpPublisher{},
		ToolRegistry: registry,
	}
	
	t.Run("Individual tools only", func(t *testing.T) {
		prompt := ai.Prompt{
			RequiredTools: []string{"readFile", "writeFile"},
		}
		
		err := loader.AddTools(&prompt)
		if err != nil {
			t.Fatalf("Failed to add tools: %v", err)
		}
		
		if len(prompt.Functions) != 2 {
			t.Errorf("Expected 2 functions, got %d", len(prompt.Functions))
		}
		
		if len(prompt.Handlers) != 2 {
			t.Errorf("Expected 2 handlers, got %d", len(prompt.Handlers))
		}
		
		// Check function names
		functionNames := make(map[string]bool)
		for _, fn := range prompt.Functions {
			functionNames[fn.Name] = true
		}
		
		if !functionNames["readFile"] || !functionNames["writeFile"] {
			t.Error("Expected readFile and writeFile in functions")
		}
	})
	
	t.Run("ToolSet reference", func(t *testing.T) {
		prompt := ai.Prompt{
			RequiredTools: []string{"@server1"},
		}
		
		err := loader.AddTools(&prompt)
		if err != nil {
			t.Fatalf("Failed to add tools: %v", err)
		}
		
		if len(prompt.Functions) != 2 {
			t.Errorf("Expected 2 functions from toolSet, got %d", len(prompt.Functions))
		}
		
		if len(prompt.Handlers) != 2 {
			t.Errorf("Expected 2 handlers from toolSet, got %d", len(prompt.Handlers))
		}
		
		// Check function names
		functionNames := make(map[string]bool)
		for _, fn := range prompt.Functions {
			functionNames[fn.Name] = true
		}
		
		if !functionNames["mcp_tool1"] || !functionNames["mcp_tool2"] {
			t.Error("Expected mcp_tool1 and mcp_tool2 in functions from toolSet")
		}
	})
	
	t.Run("Mixed individual tools and toolSets", func(t *testing.T) {
		prompt := ai.Prompt{
			RequiredTools: []string{"readFile", "@server1", "writeFile"},
		}
		
		err := loader.AddTools(&prompt)
		if err != nil {
			t.Fatalf("Failed to add tools: %v", err)
		}
		
		if len(prompt.Functions) != 4 {
			t.Errorf("Expected 4 functions (2 individual + 2 from toolSet), got %d", len(prompt.Functions))
		}
		
		if len(prompt.Handlers) != 4 {
			t.Errorf("Expected 4 handlers (2 individual + 2 from toolSet), got %d", len(prompt.Handlers))
		}
		
		// Check function names
		functionNames := make(map[string]bool)
		for _, fn := range prompt.Functions {
			functionNames[fn.Name] = true
		}
		
		expected := []string{"readFile", "writeFile", "mcp_tool1", "mcp_tool2"}
		for _, name := range expected {
			if !functionNames[name] {
				t.Errorf("Expected %s in functions", name)
			}
		}
	})
	
	t.Run("Missing individual tool", func(t *testing.T) {
		prompt := ai.Prompt{
			RequiredTools: []string{"nonexistent"},
		}
		
		err := loader.AddTools(&prompt)
		if err == nil {
			t.Error("Expected error for missing tool")
		}
		
		if !containsString(err.Error(), "missing required tools") {
			t.Errorf("Error should mention missing required tools: %v", err)
		}
	})
	
	t.Run("Missing toolSet", func(t *testing.T) {
		prompt := ai.Prompt{
			RequiredTools: []string{"@nonexistent"},
		}
		
		err := loader.AddTools(&prompt)
		if err == nil {
			t.Error("Expected error for missing toolSet")
		}
		
		if !containsString(err.Error(), "missing required tools") {
			t.Errorf("Error should mention missing required tools: %v", err)
		}
		
		if !containsString(err.Error(), "@nonexistent") {
			t.Errorf("Error should mention the missing toolSet: %v", err)
		}
	})
	
	t.Run("Empty required tools", func(t *testing.T) {
		prompt := ai.Prompt{
			RequiredTools: nil,
		}
		
		err := loader.AddTools(&prompt)
		if err != nil {
			t.Fatalf("Should not fail for empty required tools: %v", err)
		}
		
		if len(prompt.Functions) != 0 {
			t.Errorf("Expected 0 functions for empty required tools, got %d", len(prompt.Functions))
		}
	})
	
	t.Run("Error message includes available toolSets", func(t *testing.T) {
		prompt := ai.Prompt{
			RequiredTools: []string{"@nonexistent"},
		}
		
		err := loader.AddTools(&prompt)
		if err == nil {
			t.Error("Expected error for missing toolSet")
		}
		
		if !containsString(err.Error(), "available toolSets") {
			t.Errorf("Error should mention available toolSets: %v", err)
		}
		
		if !containsString(err.Error(), "@server1") {
			t.Errorf("Error should mention @server1 in available toolSets: %v", err)
		}
	})
}

// Helper function to check if a string contains a substring
func containsString(str, substr string) bool {
	return len(str) >= len(substr) && 
		   (str[:len(substr)] == substr || (len(str) > len(substr) && containsString(str[1:], substr)))
}