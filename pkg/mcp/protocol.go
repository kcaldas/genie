package mcp

import (
	"encoding/json"
	"fmt"
)

// JSON-RPC 2.0 message types

// Request represents a JSON-RPC 2.0 request
type Request struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response
type Response struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Notification represents a JSON-RPC 2.0 notification (no response expected)
type Notification struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Error represents a JSON-RPC 2.0 error
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP-specific protocol types

// InitializeRequest is sent to establish a connection with an MCP server
type InitializeRequest struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    ClientCapabilities     `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

// ClientCapabilities describes what the client supports
type ClientCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Sampling     *SamplingCapability    `json:"sampling,omitempty"`
}

// SamplingCapability indicates if the client supports sampling
type SamplingCapability struct{}

// ClientInfo contains information about the client
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the response to an initialize request
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ServerCapabilities describes what the server supports
type ServerCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Logging      *LoggingCapability     `json:"logging,omitempty"`
	Prompts      *PromptsCapability     `json:"prompts,omitempty"`
	Resources    *ResourcesCapability   `json:"resources,omitempty"`
	Tools        *ToolsCapability       `json:"tools,omitempty"`
}

// LoggingCapability indicates if the server supports logging
type LoggingCapability struct{}

// PromptsCapability indicates if the server supports prompts
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability indicates if the server supports resources
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// ToolsCapability indicates if the server supports tools
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerInfo contains information about the server
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema ToolSchema  `json:"inputSchema"`
}

// ToolSchema represents the JSON schema for tool input
type ToolSchema struct {
	Type       string                            `json:"type"`
	Properties map[string]ToolSchemaProperty     `json:"properties,omitempty"`
	Required   []string                          `json:"required,omitempty"`
	AdditionalProperties interface{}            `json:"additionalProperties,omitempty"`
}

// ToolSchemaProperty represents a property in a tool schema
type ToolSchemaProperty struct {
	Type        string                         `json:"type,omitempty"`
	Description string                         `json:"description,omitempty"`
	Enum        []string                       `json:"enum,omitempty"`
	Default     interface{}                    `json:"default,omitempty"`
	MinLength   *int                           `json:"minLength,omitempty"`
	MaxLength   *int                           `json:"maxLength,omitempty"`
	Minimum     *float64                       `json:"minimum,omitempty"`
	Maximum     *float64                       `json:"maximum,omitempty"`
	Items       *ToolSchemaProperty            `json:"items,omitempty"`       // For array types
	Properties  map[string]ToolSchemaProperty  `json:"properties,omitempty"`  // For object types
}

// ListToolsRequest requests the list of available tools
type ListToolsRequest struct{}

// ListToolsResult contains the list of available tools
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolRequest represents a tool execution request
type CallToolRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// CallToolResult represents the result of a tool execution
type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError"`
}

// Content represents content returned by tools
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Helper functions

// NewRequest creates a new JSON-RPC 2.0 request
func NewRequest(id interface{}, method string, params interface{}) *Request {
	return &Request{
		Jsonrpc: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

// NewNotification creates a new JSON-RPC 2.0 notification
func NewNotification(method string, params interface{}) *Notification {
	return &Notification{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
	}
}

// NewResponse creates a new JSON-RPC 2.0 response
func NewResponse(id interface{}, result interface{}) *Response {
	return &Response{
		Jsonrpc: "2.0",
		ID:      id,
		Result:  result,
	}
}

// NewErrorResponse creates a new JSON-RPC 2.0 error response
func NewErrorResponse(id interface{}, code int, message string, data interface{}) *Response {
	return &Response{
		Jsonrpc: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// IsRequest checks if the message is a request
func IsRequest(data []byte) bool {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return false
	}
	_, hasID := msg["id"]
	_, hasMethod := msg["method"]
	return hasID && hasMethod
}

// IsResponse checks if the message is a response
func IsResponse(data []byte) bool {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return false
	}
	_, hasID := msg["id"]
	_, hasResult := msg["result"]
	_, hasError := msg["error"]
	return hasID && (hasResult || hasError)
}

// IsNotification checks if the message is a notification
func IsNotification(data []byte) bool {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return false
	}
	_, hasID := msg["id"]
	_, hasMethod := msg["method"]
	return !hasID && hasMethod
}

// ParseMessage parses a JSON message into the appropriate type
func ParseMessage(data []byte) (interface{}, error) {
	if IsRequest(data) {
		var req Request
		err := json.Unmarshal(data, &req)
		return &req, err
	}
	
	if IsResponse(data) {
		var resp Response
		err := json.Unmarshal(data, &resp)
		return &resp, err
	}
	
	if IsNotification(data) {
		var notif Notification
		err := json.Unmarshal(data, &notif)
		return &notif, err
	}
	
	return nil, fmt.Errorf("unknown message type")
}