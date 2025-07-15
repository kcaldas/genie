package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// TestMCPServer is a simple MCP server implementation for testing
type TestMCPServer struct {
	tools map[string]Tool
}

// NewTestMCPServer creates a new test MCP server
func NewTestMCPServer() *TestMCPServer {
	server := &TestMCPServer{
		tools: make(map[string]Tool),
	}
	
	// Add some test tools
	server.tools["echo"] = Tool{
		Name:        "echo",
		Description: "Echo back the input text",
		InputSchema: ToolSchema{
			Type: "object",
			Properties: map[string]ToolSchemaProperty{
				"text": {
					Type:        "string",
					Description: "Text to echo back",
				},
			},
			Required: []string{"text"},
		},
	}
	
	server.tools["add"] = Tool{
		Name:        "add",
		Description: "Add two numbers",
		InputSchema: ToolSchema{
			Type: "object",
			Properties: map[string]ToolSchemaProperty{
				"a": {
					Type:        "number",
					Description: "First number",
				},
				"b": {
					Type:        "number",
					Description: "Second number",
				},
			},
			Required: []string{"a", "b"},
		},
	}
	
	return server
}

// Run starts the test MCP server using stdio
func (s *TestMCPServer) Run() {
	scanner := bufio.NewScanner(os.Stdin)
	
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		
		// Parse the incoming message
		message, err := ParseMessage([]byte(line))
		if err != nil {
			s.sendErrorResponse(nil, -32700, "Parse error")
			continue
		}
		
		// Handle the message based on type
		switch msg := message.(type) {
		case *Request:
			s.handleRequest(msg)
		case *Notification:
			s.handleNotification(msg)
		default:
			s.sendErrorResponse(nil, -32600, "Invalid Request")
		}
	}
}

// handleRequest handles JSON-RPC requests
func (s *TestMCPServer) handleRequest(req *Request) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	default:
		s.sendErrorResponse(req.ID, -32601, "Method not found")
	}
}

// handleNotification handles JSON-RPC notifications
func (s *TestMCPServer) handleNotification(notif *Notification) {
	switch notif.Method {
	case "notifications/initialized":
		// Server is now initialized
	default:
		// Unknown notification, ignore
	}
}

// handleInitialize handles the initialize request
func (s *TestMCPServer) handleInitialize(req *Request) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{},
		},
		ServerInfo: ServerInfo{
			Name:    "test-mcp-server",
			Version: "1.0.0",
		},
	}
	
	s.sendResponse(req.ID, result)
}

// handleToolsList handles the tools/list request
func (s *TestMCPServer) handleToolsList(req *Request) {
	var tools []Tool
	for _, tool := range s.tools {
		tools = append(tools, tool)
	}
	
	result := ListToolsResult{
		Tools: tools,
	}
	
	s.sendResponse(req.ID, result)
}

// handleToolsCall handles the tools/call request
func (s *TestMCPServer) handleToolsCall(req *Request) {
	// Parse the call request
	var callReq CallToolRequest
	if req.Params != nil {
		paramData, err := json.Marshal(req.Params)
		if err != nil {
			s.sendErrorResponse(req.ID, -32602, "Invalid params")
			return
		}
		
		if err := json.Unmarshal(paramData, &callReq); err != nil {
			s.sendErrorResponse(req.ID, -32602, "Invalid params")
			return
		}
	}
	
	// Check if tool exists
	tool, exists := s.tools[callReq.Name]
	if !exists {
		s.sendErrorResponse(req.ID, -32601, fmt.Sprintf("Tool '%s' not found", callReq.Name))
		return
	}
	
	// Execute the tool
	result := s.executeTool(tool, callReq.Arguments)
	s.sendResponse(req.ID, result)
}

// executeTool executes a tool and returns the result
func (s *TestMCPServer) executeTool(tool Tool, args map[string]interface{}) CallToolResult {
	switch tool.Name {
	case "echo":
		text, ok := args["text"].(string)
		if !ok {
			return CallToolResult{
				Content: []Content{{Type: "text", Text: "Error: text parameter must be a string"}},
				IsError: true,
			}
		}
		return CallToolResult{
			Content: []Content{{Type: "text", Text: text}},
		}
		
	case "add":
		a, aOk := args["a"].(float64)
		b, bOk := args["b"].(float64)
		if !aOk || !bOk {
			return CallToolResult{
				Content: []Content{{Type: "text", Text: "Error: a and b parameters must be numbers"}},
				IsError: true,
			}
		}
		result := a + b
		return CallToolResult{
			Content: []Content{{Type: "text", Text: fmt.Sprintf("%.2f", result)}},
		}
		
	default:
		return CallToolResult{
			Content: []Content{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", tool.Name)}},
			IsError: true,
		}
	}
}

// sendResponse sends a JSON-RPC response
func (s *TestMCPServer) sendResponse(id interface{}, result interface{}) {
	response := NewResponse(id, result)
	s.sendMessage(response)
}

// sendErrorResponse sends a JSON-RPC error response
func (s *TestMCPServer) sendErrorResponse(id interface{}, code int, message string) {
	response := NewErrorResponse(id, code, message, nil)
	s.sendMessage(response)
}

// sendMessage sends a message to stdout
func (s *TestMCPServer) sendMessage(message interface{}) {
	data, err := json.Marshal(message)
	if err != nil {
		return
	}
	
	fmt.Println(string(data))
}

// CreateTestServerExecutable creates a standalone executable for the test MCP server
func CreateTestServerExecutable(outputPath string) error {
	serverCode := `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

` + getServerStructsAndMethods() + `

func main() {
	server := NewTestMCPServer()
	server.Run()
}
`
	
	return os.WriteFile(outputPath, []byte(serverCode), 0644)
}

// getServerStructsAndMethods returns the server implementation as a string
func getServerStructsAndMethods() string {
	return `
// Include all the structs and methods from test_server.go here
// This is a simplified version for the standalone executable

type TestMCPServer struct {
	tools map[string]interface{}
}

func NewTestMCPServer() *TestMCPServer {
	return &TestMCPServer{
		tools: make(map[string]interface{}),
	}
}

func (s *TestMCPServer) Run() {
	scanner := bufio.NewScanner(os.Stdin)
	
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		
		// Simple echo implementation for testing
		fmt.Printf("{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"Echo: %s\"}]}}\n", line)
	}
}
`
}

// WriteTestServerToFile writes a simple test server to a file for subprocess execution
func WriteTestServerToFile(filename string) error {
	content := `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Request struct {
	Jsonrpc string      ` + "`json:\"jsonrpc\"`" + `
	ID      interface{} ` + "`json:\"id,omitempty\"`" + `
	Method  string      ` + "`json:\"method\"`" + `
	Params  interface{} ` + "`json:\"params,omitempty\"`" + `
}

type Response struct {
	Jsonrpc string      ` + "`json:\"jsonrpc\"`" + `
	ID      interface{} ` + "`json:\"id,omitempty\"`" + `
	Result  interface{} ` + "`json:\"result,omitempty\"`" + `
	Error   *Error      ` + "`json:\"error,omitempty\"`" + `
}

type Error struct {
	Code    int         ` + "`json:\"code\"`" + `
	Message string      ` + "`json:\"message\"`" + `
	Data    interface{} ` + "`json:\"data,omitempty\"`" + `
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}
		
		var resp Response
		resp.Jsonrpc = "2.0"
		resp.ID = req.ID
		
		switch req.Method {
		case "initialize":
			resp.Result = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name": "test-server",
					"version": "1.0.0",
				},
			}
		case "tools/list":
			resp.Result = map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name": "echo",
						"description": "Echo back the input",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"text": map[string]interface{}{
									"type": "string",
									"description": "Text to echo",
								},
							},
							"required": []string{"text"},
						},
					},
				},
			}
		case "tools/call":
			// Simple echo implementation
			resp.Result = map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Echo: Hello from test server!",
					},
				},
				"isError": false,
			}
		default:
			resp.Error = &Error{
				Code: -32601,
				Message: "Method not found",
			}
		}
		
		if respData, err := json.Marshal(resp); err == nil {
			fmt.Println(string(respData))
		}
	}
}
`
	
	return os.WriteFile(filename, []byte(content), 0755)
}