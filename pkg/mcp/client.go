package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/tools"
)

const (
	// defaultToolCallTimeout bounds a tools/call round-trip when the
	// caller supplied no deadline of its own. Generous, because MCP
	// tools may legitimately run builds or long queries.
	defaultToolCallTimeout = 2 * time.Minute
	// serverConnectTimeout is the per-server budget for spawn +
	// initialize + tool discovery. Each server gets its own budget so
	// one slow `npx`-based server cannot starve the others.
	serverConnectTimeout = 10 * time.Second
)

// Client represents an MCP client that can connect to MCP servers
type Client struct {
	config       *Config
	servers      map[string]*ServerConnection
	tools        map[string]*MCPTool
	transport    *TransportFactory
	mu           sync.RWMutex
	initialized  bool
	serverErrors map[string]string
	requestID    atomic.Int64
}

// nextRequestID returns a monotonically increasing JSON-RPC request id.
// IDs must stay well within JavaScript's safe-integer range (2^53): Node-based
// MCP servers parse JSON numbers as doubles and silently drop requests whose
// id fails integer validation (e.g. a UnixNano timestamp).
func (c *Client) nextRequestID() int64 {
	// Start after the fixed ids used by initializeServer (1) and discoverTools (2).
	return c.requestID.Add(1) + 2
}

// ServerConnection represents a connection to an MCP server
type ServerConnection struct {
	name      string
	config    ServerConfig
	transport Transport
	tools     []Tool
	connected bool
	mu        sync.RWMutex
	// requestMu serializes request/response pairs on the transport:
	// stdio JSON-RPC has no multiplexing, so two concurrent callers
	// interleaving Send/Receive would steal each other's responses.
	requestMu sync.Mutex
}

// MCPTool wraps an MCP tool to implement Genie's Tool interface
type MCPTool struct {
	mcpTool    Tool
	serverName string
	client     *Client
}

// NewClient creates a new MCP client (uninitialized - call Init to connect)
func NewClient(config *Config) *Client {
	return &Client{
		config:       config,
		servers:      make(map[string]*ServerConnection),
		tools:        make(map[string]*MCPTool),
		serverErrors: make(map[string]string),
		transport:    NewTransportFactory(),
		initialized:  false,
	}
}

// Init initializes the MCP client by discovering config from the working directory
// and connecting to all configured servers. This should be called after the working
// directory is known (e.g., from Genie.Start).
func (c *Client) Init(workingDir string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil // Already initialized
	}

	// Try to find and load MCP configuration from the working directory
	configPath, err := FindConfigFile(workingDir)
	if err != nil {
		// No MCP config found - this is fine, just mark as initialized with no servers
		c.initialized = true
		return nil
	}

	// Load the configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		// Invalid config - mark as initialized with no servers
		c.initialized = true
		return nil
	}

	// Update config with discovered settings
	c.config = config

	// Connect to servers, each with its own timeout budget.
	for serverName, serverConfig := range c.config.McpServers {
		ctx, cancel := context.WithTimeout(context.Background(), serverConnectTimeout)
		err := c.connectToServer(ctx, serverName, serverConfig)
		cancel()
		if err != nil {
			// Record error but continue with other servers. Never print
			// to stdout: the TUI owns the terminal.
			c.serverErrors[serverName] = err.Error()
			slog.Warn("Failed to connect to MCP server", "server", serverName, "error", err)
			continue
		}
		delete(c.serverErrors, serverName)
	}

	c.initialized = true
	return nil
}

// ConnectToServers connects to all configured MCP servers
func (c *Client) ConnectToServers(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for serverName, serverConfig := range c.config.McpServers {
		if err := c.connectToServer(ctx, serverName, serverConfig); err != nil {
			// Record error but continue with other servers. Never print
			// to stdout: the TUI owns the terminal.
			c.serverErrors[serverName] = err.Error()
			slog.Warn("Failed to connect to MCP server", "server", serverName, "error", err)
			continue
		}
		delete(c.serverErrors, serverName)
	}

	return nil
}

// connectToServer connects to a single MCP server
func (c *Client) connectToServer(ctx context.Context, serverName string, config ServerConfig) error {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid server configuration: %w", err)
	}

	// Create transport
	transport, err := c.transport.CreateTransport(config)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	// Connect transport (stdio connects automatically, others need explicit connection)
	if connectable, ok := transport.(interface{ Connect(context.Context) error }); ok {
		if err := connectable.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect transport: %w", err)
		}
	}

	// Create server connection
	conn := &ServerConnection{
		name:      serverName,
		config:    config,
		transport: transport,
		connected: true,
	}

	// Initialize the server
	if err := c.initializeServer(ctx, conn); err != nil {
		transport.Close()
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Discover tools
	if err := c.discoverTools(ctx, conn); err != nil {
		transport.Close()
		return fmt.Errorf("failed to discover tools: %w", err)
	}

	c.servers[serverName] = conn
	return nil
}

// initializeServer sends the initialize request to the MCP server
func (c *Client) initializeServer(ctx context.Context, conn *ServerConnection) error {
	initReq := InitializeRequest{
		ProtocolVersion: "2024-11-05",
		Capabilities: ClientCapabilities{
			Experimental: make(map[string]interface{}),
		},
		ClientInfo: ClientInfo{
			Name:    "genie",
			Version: "1.0.0",
		},
	}

	// Send initialize request
	req := NewRequest(1, "initialize", initReq)
	if err := conn.transport.Send(ctx, req); err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	// Receive the correct response for our request
	resp, err := c.receiveResponseForRequest(ctx, conn.transport, req.ID)
	if err != nil {
		return fmt.Errorf("failed to receive initialize response: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize request failed: %s", resp.Error.Message)
	}

	// Send initialized notification
	notif := NewNotification("notifications/initialized", nil)
	if err := conn.transport.Send(ctx, notif); err != nil {
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}

	return nil
}

// discoverTools discovers available tools from the MCP server
func (c *Client) discoverTools(ctx context.Context, conn *ServerConnection) error {
	// Send list_tools request
	req := NewRequest(2, "tools/list", ListToolsRequest{})
	if err := conn.transport.Send(ctx, req); err != nil {
		return fmt.Errorf("failed to send tools/list request: %w", err)
	}

	// Receive the correct response for our request
	resp, err := c.receiveResponseForRequest(ctx, conn.transport, req.ID)
	if err != nil {
		return fmt.Errorf("failed to receive tools/list response: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("tools/list request failed: %s", resp.Error.Message)
	}

	// Parse tools from result
	resultData, err := json.Marshal(resp.Result)
	if err != nil {
		return fmt.Errorf("failed to marshal tools result: %w", err)
	}

	var toolsResult ListToolsResult
	if err := json.Unmarshal(resultData, &toolsResult); err != nil {
		return fmt.Errorf("failed to parse tools result: %w", err)
	}

	// Store tools in connection
	conn.mu.Lock()
	conn.tools = toolsResult.Tools
	conn.mu.Unlock()

	// Create MCP tool wrappers and register them
	for _, tool := range toolsResult.Tools {
		mcpTool := &MCPTool{
			mcpTool:    tool,
			serverName: conn.name,
			client:     c,
		}
		c.tools[tool.Name] = mcpTool
	}

	return nil
}

// receiveResponseForRequest reads messages until it finds the response
// matching the request ID, skipping any number of unrelated messages
// (typically server notifications). The wait is bounded by ctx.
func (c *Client) receiveResponseForRequest(ctx context.Context, transport Transport, requestID interface{}) (*Response, error) {
	requestIDStr := fmt.Sprintf("%v", requestID)

	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("gave up waiting for response to request %v: %w", requestID, err)
		}

		respData, err := transport.Receive(ctx)
		if err != nil {
			return nil, err
		}

		var resp Response
		if err := json.Unmarshal(respData, &resp); err != nil {
			continue // Skip invalid JSON
		}

		// Check if this response matches our request ID
		respIDStr := fmt.Sprintf("%v", resp.ID)
		if respIDStr == requestIDStr {
			return &resp, nil
		}

		// A different id or no id at all (notification): keep reading.
	}
}

// GetTools returns all discovered MCP tools as Genie tools
func (c *Client) GetTools() []tools.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []tools.Tool
	for _, mcpTool := range c.tools {
		result = append(result, mcpTool)
	}
	return result
}

// ServerErrors returns the last connection error per configured server that
// failed to connect. Servers that connected successfully are absent.
func (c *Client) ServerErrors() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]string, len(c.serverErrors))
	for name, msg := range c.serverErrors {
		result[name] = msg
	}
	return result
}

// GetToolsByServer returns tools grouped by server name
func (c *Client) GetToolsByServer() map[string][]tools.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string][]tools.Tool)
	for _, mcpTool := range c.tools {
		serverName := mcpTool.serverName
		if result[serverName] == nil {
			result[serverName] = make([]tools.Tool, 0)
		}
		result[serverName] = append(result[serverName], mcpTool)
	}
	return result
}

// Ensure Client implements the MCPClient interface
var _ tools.MCPClient = (*Client)(nil)

// CallTool executes an MCP tool
func (c *Client) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (*CallToolResult, error) {
	c.mu.RLock()
	mcpTool, exists := c.tools[toolName]
	if !exists {
		c.mu.RUnlock()
		return nil, fmt.Errorf("tool %s not found", toolName)
	}

	serverName := mcpTool.serverName
	server, exists := c.servers[serverName]
	if !exists {
		c.mu.RUnlock()
		return nil, fmt.Errorf("server %s not found", serverName)
	}
	c.mu.RUnlock()

	server.mu.RLock()
	if !server.connected {
		server.mu.RUnlock()
		return nil, fmt.Errorf("server %s is not connected", serverName)
	}
	transport := server.transport
	server.mu.RUnlock()

	// Ensure arguments is not nil for MCP servers that require it
	if arguments == nil {
		arguments = make(map[string]interface{})
	}

	// Bound the call so a wedged server cannot hang the whole turn.
	// Callers that set their own deadline keep it.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultToolCallTimeout)
		defer cancel()
	}

	// Send tool call request
	callReq := CallToolRequest{
		Name:      toolName,
		Arguments: arguments,
	}

	// Serialize the send/receive pair: stdio JSON-RPC is not
	// multiplexed, so concurrent callers (parent + Task subagents share
	// this client) must take turns on the transport.
	server.requestMu.Lock()
	defer server.requestMu.Unlock()

	req := NewRequest(c.nextRequestID(), "tools/call", callReq)
	if err := transport.Send(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to send tools/call request: %w", err)
	}

	// Receive the response matching our request id, skipping unrelated
	// messages such as server notifications.
	resp, err := c.receiveResponseForRequest(ctx, transport, req.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to receive tools/call response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/call request failed: %s", resp.Error.Message)
	}

	// Parse result
	resultData, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool result: %w", err)
	}

	var result CallToolResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	return &result, nil
}

// Close closes all server connections
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, server := range c.servers {
		server.mu.Lock()
		if server.connected && server.transport != nil {
			server.transport.Close()
			server.connected = false
		}
		server.mu.Unlock()
	}

	return nil
}

// MCPTool implementation of Genie's Tool interface

// Declaration converts MCP tool schema to Genie's function declaration
func (t *MCPTool) Declaration() *ai.FunctionDeclaration {
	// Convert MCP schema to Genie schema
	params := convertMCPSchemaToGenieSchema(t.mcpTool.InputSchema)

	return &ai.FunctionDeclaration{
		Name:        t.mcpTool.Name,
		Description: t.mcpTool.Description,
		Parameters:  params,
		Response: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"content": {
					Type: ai.TypeArray,
					Items: &ai.Schema{
						Type: ai.TypeObject,
						Properties: map[string]*ai.Schema{
							"type": {Type: ai.TypeString},
							"text": {Type: ai.TypeString},
						},
					},
				},
			},
		},
	}
}

// Handler returns the execution handler for the MCP tool
func (t *MCPTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
		// Call the MCP tool through the client
		result, err := t.client.CallTool(ctx, t.mcpTool.Name, params)
		if err != nil {
			return nil, err
		}

		// Convert Content structs to plain maps to match MCP spec
		var contentMaps []interface{}
		for _, content := range result.Content {
			contentMap := map[string]interface{}{
				"type": content.Type,
			}
			if content.Text != "" {
				contentMap["text"] = content.Text
			}
			contentMaps = append(contentMaps, contentMap)
		}

		// Convert result to match MCP spec format
		return map[string]interface{}{
			"content": contentMaps,
			"isError": result.IsError,
		}, nil
	}
}

// FormatOutput formats the tool result for display
func (t *MCPTool) FormatOutput(result map[string]interface{}) string {
	content, ok := result["content"].([]interface{})
	if !ok {
		return fmt.Sprintf("Tool result: %v", result)
	}

	var output string
	for _, item := range content {
		if contentItem, ok := item.(map[string]interface{}); ok {
			if text, exists := contentItem["text"].(string); exists {
				output += text + "\n"
			}
		}
	}

	return output
}

// convertMCPSchemaToGenieSchema converts MCP tool schema to Genie's ai.Schema
func convertMCPSchemaToGenieSchema(mcpSchema ToolSchema) *ai.Schema {
	schema := &ai.Schema{
		Type:       convertStringToType(mcpSchema.Type),
		Required:   mcpSchema.Required,
		Properties: make(map[string]*ai.Schema),
	}

	for propName, propDef := range mcpSchema.Properties {
		genieProp := convertToolSchemaProperty(propDef)
		schema.Properties[propName] = genieProp
	}

	return schema
}

// convertToolSchemaProperty converts a single MCP ToolSchemaProperty to Genie's ai.Schema
func convertToolSchemaProperty(propDef ToolSchemaProperty) *ai.Schema {
	genieProp := &ai.Schema{
		Type:        convertStringToType(propDef.Type),
		Description: propDef.Description,
		Enum:        propDef.Enum,
	}

	// Handle numeric constraints
	if propDef.MinLength != nil {
		genieProp.MinLength = int64(*propDef.MinLength)
	}
	if propDef.MaxLength != nil {
		genieProp.MaxLength = int64(*propDef.MaxLength)
	}
	if propDef.Minimum != nil {
		genieProp.Minimum = *propDef.Minimum
	}
	if propDef.Maximum != nil {
		genieProp.Maximum = *propDef.Maximum
	}

	// Handle array items
	if propDef.Type == "array" {
		if propDef.Items != nil {
			// Use the provided items schema
			genieProp.Items = convertToolSchemaProperty(*propDef.Items)
		} else {
			// Provide a default items schema for arrays without specified items
			// Gemini requires this field for array types
			genieProp.Items = &ai.Schema{
				Type: ai.TypeString, // Default to string items
			}
		}
	}

	// Handle object properties
	if propDef.Type == "object" && len(propDef.Properties) > 0 {
		genieProp.Properties = make(map[string]*ai.Schema)
		for subPropName, subPropDef := range propDef.Properties {
			genieProp.Properties[subPropName] = convertToolSchemaProperty(subPropDef)
		}
	}

	return genieProp
}

// convertStringToType converts string type names to ai.Type constants
func convertStringToType(typeStr string) ai.Type {
	switch typeStr {
	case "string":
		return ai.TypeString
	case "number":
		return ai.TypeNumber
	case "integer":
		return ai.TypeInteger
	case "boolean":
		return ai.TypeBoolean
	case "array":
		return ai.TypeArray
	case "object":
		return ai.TypeObject
	default:
		return ai.TypeString // default to string for unknown types
	}
}
