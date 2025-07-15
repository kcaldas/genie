package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestMCPTestServer(t *testing.T) {
	// Create a temporary test server file
	tmpDir := t.TempDir()
	serverPath := tmpDir + "/test_server.go"
	
	if err := WriteTestServerToFile(serverPath); err != nil {
		t.Fatalf("Failed to create test server file: %v", err)
	}
	
	// Start the test server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "go", "run", serverPath)
	
	// Set up pipes for communication
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("Failed to create stderr pipe: %v", err)
	}
	
	// Start the process
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	
	defer func() {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()
	
	// Create a scanner to read responses
	scanner := bufio.NewScanner(stdout)
	
	// Helper function to send a request and get response
	sendRequest := func(req Request) (Response, error) {
		// Send request
		reqData, err := json.Marshal(req)
		if err != nil {
			return Response{}, err
		}
		
		t.Logf("Sending: %s", string(reqData))
		if _, err := stdin.Write(append(reqData, '\n')); err != nil {
			return Response{}, err
		}
		
		// Read response - may need to read multiple lines to find the right response
		for i := 0; i < 5; i++ { // Try up to 5 responses to find the right one
			if !scanner.Scan() {
				return Response{}, scanner.Err()
			}
			
			respLine := scanner.Text()
			t.Logf("Received: %s", respLine)
			
			var resp Response
			if err := json.Unmarshal([]byte(respLine), &resp); err != nil {
				continue // Skip invalid JSON
			}
			
			// Check if this response matches our request ID
			// Convert both to strings for comparison since JSON unmarshaling can change types
			respIDStr := fmt.Sprintf("%v", resp.ID)
			reqIDStr := fmt.Sprintf("%v", req.ID)
			if respIDStr == reqIDStr {
				return resp, nil
			}
			
			// If it's a different response, continue reading
			t.Logf("Response ID %v (%T) doesn't match request ID %v (%T), continuing...", resp.ID, resp.ID, req.ID, req.ID)
		}
		
		return Response{}, fmt.Errorf("no matching response found for request ID %v", req.ID)
	}
	
	// Test 1: Initialize
	t.Run("Initialize", func(t *testing.T) {
		initReq := Request{
			Jsonrpc: "2.0",
			ID:      1,
			Method:  "initialize",
			Params: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}
		
		resp, err := sendRequest(initReq)
		if err != nil {
			t.Fatalf("Failed to get initialize response: %v", err)
		}
		
		if resp.Error != nil {
			t.Fatalf("Initialize request failed: %+v", resp.Error)
		}
		
		if resp.Result == nil {
			t.Fatal("Expected result in initialize response")
		}
		
		// Check result structure
		result, ok := resp.Result.(map[string]interface{})
		if !ok {
			t.Fatal("Expected result to be an object")
		}
		
		if protocolVersion, ok := result["protocolVersion"].(string); !ok || protocolVersion != "2024-11-05" {
			t.Errorf("Expected protocolVersion '2024-11-05', got %v", result["protocolVersion"])
		}
		
		if _, ok := result["capabilities"]; !ok {
			t.Error("Expected capabilities in result")
		}
		
		if _, ok := result["serverInfo"]; !ok {
			t.Error("Expected serverInfo in result")
		}
	})
	
	// Test 2: Send initialized notification
	t.Run("Initialized", func(t *testing.T) {
		initNotif := Request{
			Jsonrpc: "2.0",
			Method:  "notifications/initialized",
		}
		
		// Send notification
		notifData, err := json.Marshal(initNotif)
		if err != nil {
			t.Fatalf("Failed to marshal notification: %v", err)
		}
		
		if _, err := stdin.Write(append(notifData, '\n')); err != nil {
			t.Fatalf("Failed to send notification: %v", err)
		}
		
		// No response expected for notifications
	})
	
	// Test 3: List tools
	t.Run("ListTools", func(t *testing.T) {
		listReq := Request{
			Jsonrpc: "2.0",
			ID:      2,
			Method:  "tools/list",
		}
		
		resp, err := sendRequest(listReq)
		if err != nil {
			t.Fatalf("Failed to get tools/list response: %v", err)
		}
		
		if resp.Error != nil {
			t.Fatalf("Tools/list request failed: %+v", resp.Error)
		}
		
		if resp.Result == nil {
			t.Fatal("Expected result in tools/list response")
		}
		
		result, ok := resp.Result.(map[string]interface{})
		if !ok {
			t.Fatal("Expected result to be an object")
		}
		
		tools, ok := result["tools"].([]interface{})
		if !ok {
			t.Fatal("Expected tools to be an array")
		}
		
		if len(tools) == 0 {
			t.Fatal("Expected at least one tool")
		}
		
		// Check first tool
		firstTool, ok := tools[0].(map[string]interface{})
		if !ok {
			t.Fatal("Expected tool to be an object")
		}
		
		if name, ok := firstTool["name"].(string); !ok || name != "echo" {
			t.Errorf("Expected first tool name to be 'echo', got %v", firstTool["name"])
		}
		
		if _, ok := firstTool["description"]; !ok {
			t.Error("Expected tool to have description")
		}
		
		if _, ok := firstTool["inputSchema"]; !ok {
			t.Error("Expected tool to have inputSchema")
		}
	})
	
	// Test 4: Call tool
	t.Run("CallTool", func(t *testing.T) {
		callReq := Request{
			Jsonrpc: "2.0",
			ID:      3,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name": "echo",
				"arguments": map[string]interface{}{
					"text": "Hello, World!",
				},
			},
		}
		
		resp, err := sendRequest(callReq)
		if err != nil {
			t.Fatalf("Failed to get tools/call response: %v", err)
		}
		
		if resp.Error != nil {
			t.Fatalf("Tools/call request failed: %+v", resp.Error)
		}
		
		if resp.Result == nil {
			t.Fatal("Expected result in tools/call response")
		}
		
		result, ok := resp.Result.(map[string]interface{})
		if !ok {
			t.Fatal("Expected result to be an object")
		}
		
		content, ok := result["content"].([]interface{})
		if !ok {
			t.Fatal("Expected content to be an array")
		}
		
		if len(content) == 0 {
			t.Fatal("Expected at least one content item")
		}
		
		firstContent, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatal("Expected content item to be an object")
		}
		
		if contentType, ok := firstContent["type"].(string); !ok || contentType != "text" {
			t.Errorf("Expected content type 'text', got %v", firstContent["type"])
		}
		
		if text, ok := firstContent["text"].(string); !ok || text == "" {
			t.Errorf("Expected non-empty text content, got %v", firstContent["text"])
		} else {
			t.Logf("Tool returned: %s", text)
		}
	})
	
	// Test 5: Unknown method
	t.Run("UnknownMethod", func(t *testing.T) {
		unknownReq := Request{
			Jsonrpc: "2.0",
			ID:      4,
			Method:  "unknown/method",
		}
		
		resp, err := sendRequest(unknownReq)
		if err != nil {
			t.Fatalf("Failed to get unknown method response: %v", err)
		}
		
		if resp.Error == nil {
			t.Fatal("Expected error for unknown method")
		}
		
		if resp.Error.Code != -32601 {
			t.Errorf("Expected error code -32601, got %d", resp.Error.Code)
		}
		
		if !strings.Contains(resp.Error.Message, "Method not found") {
			t.Errorf("Expected 'Method not found' in error message, got: %s", resp.Error.Message)
		}
	})
}