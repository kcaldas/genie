package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestMCPSpecCompliance tests that our implementation follows the MCP 2024-11-05 specification
func TestMCPSpecCompliance(t *testing.T) {
	// Test MCP specification compliance for 2024-11-05 version
	
	t.Run("JSON-RPC 2.0 Compliance", func(t *testing.T) {
		// Test that all requests/responses follow JSON-RPC 2.0 format
		tests := []struct {
			name     string
			request  Request
			expected []string // fields that must be present in response
		}{
			{
				name: "initialize request",
				request: Request{
					Jsonrpc: "2.0",
					ID:      1,
					Method:  "initialize",
					Params: InitializeRequest{
						ProtocolVersion: "2024-11-05",
						Capabilities:    ClientCapabilities{},
						ClientInfo: ClientInfo{
							Name:    "test-client",
							Version: "1.0.0",
						},
					},
				},
				expected: []string{"jsonrpc", "id", "result"},
			},
			{
				name: "tools/list request",
				request: Request{
					Jsonrpc: "2.0",
					ID:      2,
					Method:  "tools/list",
				},
				expected: []string{"jsonrpc", "id", "result"},
			},
			{
				name: "tools/call request",
				request: Request{
					Jsonrpc: "2.0",
					ID:      3,
					Method:  "tools/call",
					Params: CallToolRequest{
						Name:      "echo",
						Arguments: map[string]interface{}{"text": "test"},
					},
				},
				expected: []string{"jsonrpc", "id", "result"},
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Verify request format
				reqData, err := json.Marshal(tt.request)
				if err != nil {
					t.Fatalf("Request serialization failed: %v", err)
				}
				
				var reqCheck map[string]interface{}
				if err := json.Unmarshal(reqData, &reqCheck); err != nil {
					t.Fatalf("Request deserialization failed: %v", err)
				}
				
				// Check JSON-RPC 2.0 fields
				if reqCheck["jsonrpc"] != "2.0" {
					t.Errorf("Expected jsonrpc='2.0', got %v", reqCheck["jsonrpc"])
				}
				
				if reqCheck["id"] == nil {
					t.Error("Request ID must not be null")
				}
				
				if reqCheck["method"] == "" {
					t.Error("Method must be present")
				}
			})
		}
	})
	
	t.Run("Initialize Protocol Compliance", func(t *testing.T) {
		// Test initialize request/response format
		initReq := InitializeRequest{
			ProtocolVersion: "2024-11-05",
			Capabilities:    ClientCapabilities{},
			ClientInfo: ClientInfo{
				Name:    "genie",
				Version: "1.0.0",
			},
		}
		
		// Verify request structure
		reqData, err := json.Marshal(initReq)
		if err != nil {
			t.Fatalf("Initialize request serialization failed: %v", err)
		}
		
		var reqMap map[string]interface{}
		if err := json.Unmarshal(reqData, &reqMap); err != nil {
			t.Fatalf("Initialize request deserialization failed: %v", err)
		}
		
		// Check required fields
		if reqMap["protocolVersion"] != "2024-11-05" {
			t.Errorf("Expected protocolVersion='2024-11-05', got %v", reqMap["protocolVersion"])
		}
		
		if reqMap["capabilities"] == nil {
			t.Error("Capabilities must be present")
		}
		
		if reqMap["clientInfo"] == nil {
			t.Error("ClientInfo must be present")
		}
		
		// Test initialize response format
		initResult := InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: ServerCapabilities{
				Tools: &ToolsCapability{},
			},
			ServerInfo: ServerInfo{
				Name:    "test-server",
				Version: "1.0.0",
			},
		}
		
		resultData, err := json.Marshal(initResult)
		if err != nil {
			t.Fatalf("Initialize result serialization failed: %v", err)
		}
		
		var resultMap map[string]interface{}
		if err := json.Unmarshal(resultData, &resultMap); err != nil {
			t.Fatalf("Initialize result deserialization failed: %v", err)
		}
		
		// Check required response fields
		if resultMap["protocolVersion"] != "2024-11-05" {
			t.Errorf("Expected protocolVersion='2024-11-05', got %v", resultMap["protocolVersion"])
		}
		
		if resultMap["capabilities"] == nil {
			t.Error("Capabilities must be present in response")
		}
		
		if resultMap["serverInfo"] == nil {
			t.Error("ServerInfo must be present in response")
		}
	})
	
	t.Run("Tools List Compliance", func(t *testing.T) {
		// Test tools/list response format
		toolsResult := ListToolsResult{
			Tools: []Tool{
				{
					Name:        "echo",
					Description: "Echo back the input",
					InputSchema: ToolSchema{
						Type: "object",
						Properties: map[string]ToolSchemaProperty{
							"text": {
								Type:        "string",
								Description: "Text to echo",
							},
						},
						Required: []string{"text"},
					},
				},
			},
		}
		
		resultData, err := json.Marshal(toolsResult)
		if err != nil {
			t.Fatalf("Tools list result serialization failed: %v", err)
		}
		
		var resultMap map[string]interface{}
		if err := json.Unmarshal(resultData, &resultMap); err != nil {
			t.Fatalf("Tools list result deserialization failed: %v", err)
		}
		
		// Check required fields
		tools, ok := resultMap["tools"].([]interface{})
		if !ok {
			t.Fatal("Tools must be an array")
		}
		
		if len(tools) == 0 {
			t.Fatal("Expected at least one tool")
		}
		
		tool, ok := tools[0].(map[string]interface{})
		if !ok {
			t.Fatal("Tool must be an object")
		}
		
		// Check tool structure
		if tool["name"] == nil {
			t.Error("Tool name must be present")
		}
		
		if tool["description"] == nil {
			t.Error("Tool description must be present")
		}
		
		if tool["inputSchema"] == nil {
			t.Error("Tool inputSchema must be present")
		}
		
		inputSchema, ok := tool["inputSchema"].(map[string]interface{})
		if !ok {
			t.Fatal("InputSchema must be an object")
		}
		
		if inputSchema["type"] != "object" {
			t.Error("InputSchema type should be 'object'")
		}
		
		if inputSchema["properties"] == nil {
			t.Error("InputSchema properties must be present")
		}
	})
	
	t.Run("Tools Call Compliance", func(t *testing.T) {
		// Test tools/call request format
		callReq := CallToolRequest{
			Name:      "echo",
			Arguments: map[string]interface{}{"text": "Hello World"},
		}
		
		reqData, err := json.Marshal(callReq)
		if err != nil {
			t.Fatalf("Tools call request serialization failed: %v", err)
		}
		
		var reqMap map[string]interface{}
		if err := json.Unmarshal(reqData, &reqMap); err != nil {
			t.Fatalf("Tools call request deserialization failed: %v", err)
		}
		
		// Check required fields
		if reqMap["name"] == nil {
			t.Error("Tool name must be present")
		}
		
		if reqMap["arguments"] == nil {
			t.Error("Tool arguments must be present")
		}
		
		// Test tools/call response format
		callResult := CallToolResult{
			Content: []Content{
				{
					Type: "text",
					Text: "Echo: Hello World",
				},
			},
			IsError: false,
		}
		
		resultData, err := json.Marshal(callResult)
		if err != nil {
			t.Fatalf("Tools call result serialization failed: %v", err)
		}
		
		var resultMap map[string]interface{}
		if err := json.Unmarshal(resultData, &resultMap); err != nil {
			t.Fatalf("Tools call result deserialization failed: %v", err)
		}
		
		// Check required response fields
		content, ok := resultMap["content"].([]interface{})
		if !ok {
			t.Fatal("Content must be an array")
		}
		
		if len(content) == 0 {
			t.Fatal("Expected at least one content item")
		}
		
		contentItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatal("Content item must be an object")
		}
		
		if contentItem["type"] == nil {
			t.Error("Content type must be present")
		}
		
		if contentItem["type"] == "text" && contentItem["text"] == nil {
			t.Error("Text content must have text field")
		}
		
		if resultMap["isError"] == nil {
			t.Error("IsError field must be present")
		}
		
		if _, ok := resultMap["isError"].(bool); !ok {
			t.Error("IsError must be a boolean")
		}
	})
	
	t.Run("Client Integration Compliance", func(t *testing.T) {
		// Test that our client follows the spec
		config := &Config{
			McpServers: map[string]ServerConfig{
				"test-server": {
					Command: "go",
					Args:    []string{"run", "/tmp/test_server.go"},
				},
			},
		}
		
		client := NewClient(config)
		defer client.Close()
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		// Test connection and initialization
		err := client.ConnectToServers(ctx)
		if err != nil {
			t.Fatalf("Failed to connect to servers: %v", err)
		}
		
		// Test tool discovery
		tools := client.GetTools()
		if len(tools) == 0 {
			t.Fatal("No tools discovered")
		}
		
		// Test tool execution
		echoTool := tools[0]
		handler := echoTool.Handler()
		
		result, err := handler(ctx, map[string]interface{}{
			"text": "Spec compliance test",
		})
		if err != nil {
			t.Fatalf("Tool execution failed: %v", err)
		}
		
		// Verify result format matches spec
		content, ok := result["content"].([]interface{})
		if !ok {
			t.Fatal("Content must be an array")
		}
		
		if len(content) == 0 {
			t.Fatal("Expected at least one content item")
		}
		
		contentItem, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatal("Content item must be an object")
		}
		
		if contentItem["type"] != "text" {
			t.Error("Expected content type 'text'")
		}
		
		if contentItem["text"] == nil {
			t.Error("Text content must have text field")
		}
		
		isError, ok := result["isError"].(bool)
		if !ok {
			t.Fatal("IsError must be a boolean")
		}
		
		if isError {
			t.Error("Tool execution should not be an error")
		}
	})
}