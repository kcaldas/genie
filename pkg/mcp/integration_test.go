package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/tools"
)

func TestMCPClientIntegration(t *testing.T) {
	// Create a temporary directory for our test
	tmpDir := t.TempDir()
	
	// Create a simple test server executable
	serverPath := filepath.Join(tmpDir, "test_server.go")
	if err := WriteTestServerToFile(serverPath); err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	
	// Create MCP configuration
	config := &Config{
		McpServers: map[string]ServerConfig{
			"test-server": {
				Command: "go",
				Args:    []string{"run", serverPath},
				Env:     map[string]string{},
			},
		},
	}
	
	// Create MCP client
	client := NewClient(config)
	defer client.Close()
	
	// Connect to servers
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := client.ConnectToServers(ctx); err != nil {
		t.Fatalf("Failed to connect to servers: %v", err)
	}
	
	// Give the server a moment to fully initialize
	time.Sleep(100 * time.Millisecond)
	
	// Test tool discovery
	tools := client.GetTools()
	if len(tools) == 0 {
		t.Fatal("No tools discovered from MCP server")
	}
	
	t.Logf("Discovered %d tools", len(tools))
	for _, tool := range tools {
		decl := tool.Declaration()
		t.Logf("Tool: %s - %s", decl.Name, decl.Description)
	}
	
	// Test tool execution
	echoTool := findToolByName(tools, "echo")
	if echoTool == nil {
		t.Fatal("Echo tool not found")
	}
	
	// Execute the echo tool
	handler := echoTool.Handler()
	result, err := handler(ctx, map[string]interface{}{
		"text": "Hello, MCP!",
	})
	
	if err != nil {
		t.Fatalf("Failed to execute echo tool: %v", err)
	}
	
	t.Logf("Tool execution result: %+v", result)
	
	// Verify the result
	content, ok := result["content"].([]interface{})
	if !ok {
		t.Fatalf("Expected content array in result, got: %T %+v", result["content"], result["content"])
	}
	
	if len(content) == 0 {
		t.Fatal("Expected at least one content item")
	}
	
	firstContent, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected content item to be a map, got: %T %+v", content[0], content[0])
	}
	
	text, ok := firstContent["text"].(string)
	if !ok {
		t.Fatalf("Expected text field in content, got: %+v", firstContent)
	}
	
	if text == "" {
		t.Fatal("Expected non-empty text response")
	}
	
	t.Logf("Echo tool returned: %s", text)
}

func TestMCPConfigurationLoading(t *testing.T) {
	// Create a temporary directory for our test
	tmpDir := t.TempDir()
	
	// Create a test .mcp.json file
	configPath := filepath.Join(tmpDir, ".mcp.json")
	configContent := `{
  "mcpServers": {
    "test-server": {
      "command": "echo",
      "args": ["hello"],
      "env": {
        "TEST_VAR": "test_value"
      }
    }
  }
}`
	
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}
	
	// Test configuration loading
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	if len(config.McpServers) != 1 {
		t.Fatalf("Expected 1 server, got %d", len(config.McpServers))
	}
	
	testServer, exists := config.McpServers["test-server"]
	if !exists {
		t.Fatal("test-server not found in config")
	}
	
	if testServer.Command != "echo" {
		t.Errorf("Expected command 'echo', got '%s'", testServer.Command)
	}
	
	if len(testServer.Args) != 1 || testServer.Args[0] != "hello" {
		t.Errorf("Expected args ['hello'], got %v", testServer.Args)
	}
	
	if testServer.Env["TEST_VAR"] != "test_value" {
		t.Errorf("Expected TEST_VAR='test_value', got '%s'", testServer.Env["TEST_VAR"])
	}
}

func TestMCPClientWithoutConfig(t *testing.T) {
	// Test client behavior when no MCP configuration is found
	client, err := NewMCPClientFromConfig()
	if err != nil {
		t.Fatalf("Expected client to be created even without config, got error: %v", err)
	}
	defer client.Close()
	
	// Should return empty tools list
	tools := client.GetTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools with no config, got %d", len(tools))
	}
}

func TestTransportFactory(t *testing.T) {
	factory := NewTransportFactory()
	
	// Test stdio transport creation
	stdioConfig := ServerConfig{
		Command: "/bin/echo",
		Args:    []string{"hello"},
	}
	
	transport, err := factory.CreateTransport(stdioConfig)
	if err != nil {
		t.Fatalf("Failed to create stdio transport: %v", err)
	}
	defer transport.Close()
	
	if !transport.IsConnected() {
		// Expected for stdio transport before connection
	}
	
	// Test SSE transport creation
	sseConfig := ServerConfig{
		Type: "sse",
		URL:  "https://example.com/mcp",
		Headers: map[string]string{
			"Authorization": "Bearer token",
		},
	}
	
	sseTransport, err := factory.CreateTransport(sseConfig)
	if err != nil {
		t.Fatalf("Failed to create SSE transport: %v", err)
	}
	defer sseTransport.Close()
	
	// Test invalid config
	invalidConfig := ServerConfig{}
	_, err = factory.CreateTransport(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

// Helper function to find a tool by name
func findToolByName(toolList []tools.Tool, name string) tools.Tool {
	for _, tool := range toolList {
		decl := tool.Declaration()
		if decl.Name == name {
			return tool
		}
	}
	return nil
}