package tools

import (
	"testing"

	"github.com/kcaldas/genie/pkg/events"
)

// MockTool is already defined in registry_test.go, so we'll reuse it

func TestRegistryToolSetOperations(t *testing.T) {
	registry := NewRegistry()
	
	// Create some mock tools
	tool1 := &MockTool{name: "tool1"}
	tool2 := &MockTool{name: "tool2"}
	tool3 := &MockTool{name: "tool3"}
	
	// Register individual tools
	err := registry.Register(tool1)
	if err != nil {
		t.Fatalf("Failed to register tool1: %v", err)
	}
	
	err = registry.Register(tool2)
	if err != nil {
		t.Fatalf("Failed to register tool2: %v", err)
	}
	
	err = registry.Register(tool3)
	if err != nil {
		t.Fatalf("Failed to register tool3: %v", err)
	}
	
	// Test RegisterToolSet
	t.Run("RegisterToolSet", func(t *testing.T) {
		toolSet1 := []Tool{tool1, tool2}
		err := registry.RegisterToolSet("set1", toolSet1)
		if err != nil {
			t.Fatalf("Failed to register toolSet: %v", err)
		}
		
		// Test duplicate registration
		err = registry.RegisterToolSet("set1", toolSet1)
		if err == nil {
			t.Error("Expected error for duplicate toolSet registration")
		}
		
		// Test empty toolSet
		err = registry.RegisterToolSet("empty", []Tool{})
		if err == nil {
			t.Error("Expected error for empty toolSet")
		}
		
		// Test empty name
		err = registry.RegisterToolSet("", toolSet1)
		if err == nil {
			t.Error("Expected error for empty toolSet name")
		}
	})
	
	// Test GetToolSet
	t.Run("GetToolSet", func(t *testing.T) {
		tools, exists := registry.GetToolSet("set1")
		if !exists {
			t.Fatal("Expected toolSet 'set1' to exist")
		}
		
		if len(tools) != 2 {
			t.Errorf("Expected 2 tools in set1, got %d", len(tools))
		}
		
		// Verify tools are correct
		toolNames := make(map[string]bool)
		for _, tool := range tools {
			toolNames[tool.Declaration().Name] = true
		}
		
		if !toolNames["tool1"] || !toolNames["tool2"] {
			t.Error("Expected tool1 and tool2 in set1")
		}
		
		// Test non-existent toolSet
		_, exists = registry.GetToolSet("nonexistent")
		if exists {
			t.Error("Expected toolSet 'nonexistent' to not exist")
		}
	})
	
	// Test GetToolSetNames
	t.Run("GetToolSetNames", func(t *testing.T) {
		// Register another toolSet
		toolSet2 := []Tool{tool3}
		err := registry.RegisterToolSet("set2", toolSet2)
		if err != nil {
			t.Fatalf("Failed to register toolSet2: %v", err)
		}
		
		names := registry.GetToolSetNames()
		if len(names) != 2 {
			t.Errorf("Expected 2 toolSet names, got %d", len(names))
		}
		
		nameSet := make(map[string]bool)
		for _, name := range names {
			nameSet[name] = true
		}
		
		if !nameSet["set1"] || !nameSet["set2"] {
			t.Error("Expected set1 and set2 in toolSet names")
		}
	})
	
	// Test that GetToolSet returns copies
	t.Run("GetToolSetReturnsCopies", func(t *testing.T) {
		tools, exists := registry.GetToolSet("set1")
		if !exists {
			t.Fatal("Expected toolSet 'set1' to exist")
		}
		
		originalLen := len(tools)
		
		// Modify the returned slice
		tools = append(tools, tool3)
		
		// Get the toolSet again
		tools2, exists := registry.GetToolSet("set1")
		if !exists {
			t.Fatal("Expected toolSet 'set1' to exist")
		}
		
		// Should still have original length
		if len(tools2) != originalLen {
			t.Errorf("Expected toolSet to remain unchanged, got %d tools, expected %d", len(tools2), originalLen)
		}
	})
}

func TestMCPIntegrationWithToolSets(t *testing.T) {
	// Create a mock MCP client
	mockClient := &MockMCPClient{
		tools: map[string][]Tool{
			"server1": {
				&MockTool{name: "server1_tool1"},
				&MockTool{name: "server1_tool2"},
			},
			"server2": {
				&MockTool{name: "server2_tool1"},
			},
		},
	}
	
	// Create event bus and todo manager
	eventBus := &events.NoOpEventBus{}
	todoManager := NewTodoManager()
	
	// Create registry with MCP
	registry := NewRegistryWithMCP(eventBus, todoManager, mockClient)
	
	// Test that individual tools are registered
	tool, exists := registry.Get("server1_tool1")
	if !exists {
		t.Error("Expected server1_tool1 to be registered")
	}
	if tool.Declaration().Name != "server1_tool1" {
		t.Errorf("Expected tool name 'server1_tool1', got '%s'", tool.Declaration().Name)
	}
	
	// Test that toolSets are registered
	toolSet1, exists := registry.GetToolSet("server1")
	if !exists {
		t.Error("Expected server1 toolSet to be registered")
	}
	if len(toolSet1) != 2 {
		t.Errorf("Expected 2 tools in server1 toolSet, got %d", len(toolSet1))
	}
	
	toolSet2, exists := registry.GetToolSet("server2")
	if !exists {
		t.Error("Expected server2 toolSet to be registered")
	}
	if len(toolSet2) != 1 {
		t.Errorf("Expected 1 tool in server2 toolSet, got %d", len(toolSet2))
	}
	
	// Test toolSet names
	toolSetNames := registry.GetToolSetNames()
	nameSet := make(map[string]bool)
	for _, name := range toolSetNames {
		nameSet[name] = true
	}
	
	if !nameSet["server1"] || !nameSet["server2"] {
		t.Error("Expected server1 and server2 in toolSet names")
	}
}

// MockMCPClient for testing
type MockMCPClient struct {
	tools map[string][]Tool
}

func (m *MockMCPClient) GetTools() []Tool {
	var allTools []Tool
	for _, serverTools := range m.tools {
		allTools = append(allTools, serverTools...)
	}
	return allTools
}

func (m *MockMCPClient) GetToolsByServer() map[string][]Tool {
	return m.tools
}