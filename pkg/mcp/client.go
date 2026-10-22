// Package mcp adapts the official MCP SDK to Genie's tool registry.
package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	genieconfig "github.com/kcaldas/genie/pkg/config"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
	"github.com/kcaldas/genie/pkg/tools"
	"github.com/modelcontextprotocol/go-sdk/auth"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultToolCallTimeout = 2 * time.Minute
	serverConnectTimeout   = 10 * time.Second
)

// Options connects host-owned interaction and authorization to MCP. No sampling,
// roots, or elicitation capability is advertised without a host implementation.
// Configure these before connecting; callbacks must not reconnect or close the client.
type Options struct {
	ClientOptions func(server string) *sdk.ClientOptions
	// ConfigureClient registers SDK extension methods or middleware before connecting.
	ConfigureClient func(server string, client *sdk.Client) error
	HTTPClient      *http.Client
	OAuthHandler    func(server string) auth.OAuthHandler
	// ToolsChanged runs after the catalog is refreshed. Hosts decide when to
	// rebuild their registry and which newly discovered tools to authorize.
	ToolsChanged func(server string)
}

type Client struct {
	config       *Config
	servers      map[string]*ServerConnection
	tools        map[string]*MCPTool
	mu           sync.RWMutex
	lifecycle    sync.Mutex
	initialized  bool
	closed       bool
	serverErrors map[string]string
	maxBlobBytes int
	options      Options
}

type ServerConnection struct {
	name    string
	config  ServerConfig
	session *sdk.ClientSession
	tools   []Tool
	refresh sync.Mutex
}

type MCPTool struct {
	mcpTool    Tool
	serverName string
	client     *Client
}

func NewClient(config *Config, manager genieconfig.Manager) *Client {
	return NewClientWithOptions(config, manager, Options{})
}

func NewClientWithOptions(config *Config, manager genieconfig.Manager, options Options) *Client {
	if config == nil {
		config = &Config{McpServers: map[string]ServerConfig{}}
	}
	return &Client{config: config, servers: map[string]*ServerConnection{}, tools: map[string]*MCPTool{}, serverErrors: map[string]string{}, maxBlobBytes: llmshared.MaxToolBlobBytesFromEnv(manager), options: options}
}

func (c *Client) Init(workingDir string) error {
	c.lifecycle.Lock()
	defer c.lifecycle.Unlock()
	c.mu.RLock()
	initialized, closed := c.initialized, c.closed
	c.mu.RUnlock()
	if closed {
		return fmt.Errorf("MCP client is closed")
	}
	if initialized {
		return nil
	}
	path, err := FindConfigFile(workingDir)
	if err == nil {
		cfg, err := LoadConfig(path)
		if err != nil {
			return fmt.Errorf("load MCP configuration: %w", err)
		}
		c.mu.Lock()
		c.config = cfg
		c.mu.Unlock()
	}
	if err := c.connectServers(context.Background()); err != nil {
		return err
	}
	c.mu.Lock()
	c.initialized = true
	c.mu.Unlock()
	return nil
}

func (c *Client) ConnectToServers(ctx context.Context) error {
	c.lifecycle.Lock()
	defer c.lifecycle.Unlock()
	return c.connectServers(ctx)
}

func (c *Client) connectServers(ctx context.Context) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return fmt.Errorf("MCP client is closed")
	}
	configs := maps.Clone(c.config.McpServers)
	c.mu.RUnlock()
	names := make([]string, 0, len(configs))
	for name := range configs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.mu.RLock()
		existing := c.servers[name]
		c.mu.RUnlock()
		if existing != nil {
			continue
		}
		connectCtx, cancel := context.WithTimeout(ctx, serverConnectTimeout)
		err := c.connectToServer(connectCtx, name, configs[name])
		cancel()
		c.mu.Lock()
		if err != nil {
			c.serverErrors[name] = err.Error()
		} else {
			delete(c.serverErrors, name)
		}
		c.mu.Unlock()
		if err != nil {
			slog.Warn("Failed to connect to MCP server", "server", name, "error", err)
		}
	}
	return ctx.Err()
}

func (c *Client) connectToServer(ctx context.Context, name string, config ServerConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid server configuration: %w", err)
	}
	options := sdk.ClientOptions{Capabilities: &sdk.ClientCapabilities{}}
	if c.options.ClientOptions != nil {
		if configured := c.options.ClientOptions(name); configured != nil {
			options = *configured
		}
	}
	if options.Capabilities == nil {
		options.Capabilities = &sdk.ClientCapabilities{}
	}
	changed := options.ToolListChangedHandler
	options.ToolListChangedHandler = func(ctx context.Context, req *sdk.ToolListChangedRequest) {
		// SDK notification handlers must return without making blocking RPCs.
		go func() {
			refreshCtx, cancel := context.WithTimeout(context.Background(), serverConnectTimeout)
			defer cancel()
			if err := c.RefreshTools(refreshCtx, name); err != nil {
				c.mu.Lock()
				if !c.closed {
					c.serverErrors[name] = fmt.Sprintf("refresh tools: %v", err)
				}
				c.mu.Unlock()
			} else if c.options.ToolsChanged != nil {
				c.options.ToolsChanged(name)
			}
		}()
		if changed != nil {
			changed(ctx, req)
		}
	}
	transport, err := newTransport(name, config, c.options, c.maxBlobBytes)
	if err != nil {
		return err
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "genie", Version: "1.0.0"}, &options)
	if c.options.ConfigureClient != nil {
		if err := c.options.ConfigureClient(name, client); err != nil {
			return fmt.Errorf("configure MCP client: %w", err)
		}
	}
	if config.GetTransportType() == TransportHTTP {
		// The SDK retains legacy cancellation notifications. The new HTTP protocol
		// uses closing the request stream as the signal and defines no such POST.
		client.AddSendingMiddleware(func(next sdk.MethodHandler) sdk.MethodHandler {
			return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
				if method == "notifications/cancelled" {
					if session, ok := req.GetSession().(*sdk.ClientSession); ok && session.InitializeResult() != nil && session.InitializeResult().ProtocolVersion >= "2026-07-28" {
						return nil, nil
					}
				}
				return next(ctx, method, req)
			}
		})
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	conn := &ServerConnection{name: name, config: config, session: session}
	catalog, err := loadTools(ctx, session)
	if err != nil {
		_ = session.Close()
		return fmt.Errorf("discover tools: %w", err)
	}
	conn.tools = catalog
	c.mu.Lock()
	c.servers[name] = conn
	c.rebuildToolsLocked()
	c.mu.Unlock()
	return nil
}

func loadTools(ctx context.Context, session *sdk.ClientSession) ([]Tool, error) {
	if session.InitializeResult().Capabilities.Tools == nil {
		return nil, nil
	}
	result := []Tool{}
	seen := map[string]bool{}
	params := &sdk.ListToolsParams{}
	cursors := map[string]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if cursors[params.Cursor] {
			return nil, fmt.Errorf("repeated tools/list pagination cursor")
		}
		cursors[params.Cursor] = true
		page, err := session.ListTools(ctx, params)
		if err != nil {
			return nil, err
		}
		for _, tool := range page.Tools {
			if seen[tool.Name] {
				return nil, fmt.Errorf("duplicate tool %q in server catalog", tool.Name)
			}
			seen[tool.Name] = true
			raw, err := json.Marshal(tool)
			if err != nil {
				return nil, err
			}
			var definition Tool
			if err := json.Unmarshal(raw, &definition); err != nil {
				return nil, fmt.Errorf("tool %q: %w", tool.Name, err)
			}
			result = append(result, definition)
		}
		if page.NextCursor == "" {
			break
		}
		params.Cursor = page.NextCursor
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// RefreshTools atomically replaces a server's catalog, preserving the last good
// snapshot on failure. Existing tool handles cannot call a removed tool.
func (c *Client) RefreshTools(ctx context.Context, server string) error {
	c.mu.RLock()
	conn := c.servers[server]
	closed := c.closed
	c.mu.RUnlock()
	if closed {
		return fmt.Errorf("MCP client is closed")
	}
	if conn == nil {
		return fmt.Errorf("MCP server %q is not connected", server)
	}
	conn.refresh.Lock()
	defer conn.refresh.Unlock()
	catalog, err := loadTools(ctx, conn.session)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.servers[server] != conn {
		return fmt.Errorf("MCP server %q disconnected", server)
	}
	conn.tools = catalog
	c.rebuildToolsLocked()
	delete(c.serverErrors, server)
	return nil
}

// Never resolve ambiguous unqualified names to a random server. Namespacing is
// opt-in through toolPrefix, so a catalog change cannot silently reroute a name.
func (c *Client) rebuildToolsLocked() {
	next := map[string]*MCPTool{}
	ambiguous := map[string]bool{}
	names := make([]string, 0, len(c.servers))
	for name := range c.servers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, server := range names {
		conn := c.servers[server]
		for _, definition := range conn.tools {
			exposed := conn.config.ToolPrefix + definition.Name
			if ambiguous[exposed] {
				continue
			}
			if _, exists := next[exposed]; exists {
				delete(next, exposed)
				ambiguous[exposed] = true
				continue
			}
			next[exposed] = &MCPTool{mcpTool: definition, serverName: server, client: c}
		}
	}
	c.tools = next
}

func (c *Client) GetTools() []tools.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, 0, len(c.tools))
	for name := range c.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]tools.Tool, 0, len(names))
	for _, name := range names {
		result = append(result, c.tools[name])
	}
	return result
}
func (c *Client) ServerErrors() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return maps.Clone(c.serverErrors)
}
func (c *Client) GetToolsByServer() map[string][]tools.Tool {
	result := map[string][]tools.Tool{}
	for _, tool := range c.GetTools() {
		m := tool.(*MCPTool)
		result[m.serverName] = append(result[m.serverName], tool)
	}
	return result
}

// ToolsMeta retains Genie's display and localized progress extensions. Config
// overrides are keyed by the server's wire name even when toolPrefix is used.
func (c *Client) ToolsMeta() map[string]tools.MCPToolMeta {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]tools.MCPToolMeta, len(c.tools))
	for name, t := range c.tools {
		cfg := c.config.McpServers[t.serverName]
		meta := tools.MCPToolMeta{Server: t.serverName, ServerDisplayName: cfg.DisplayName, DisplayName: t.mcpTool.Title, SignalText: cfg.SignalText}
		if override, ok := cfg.Tools[t.mcpTool.Name]; ok {
			if override.DisplayName != "" {
				meta.DisplayName = override.DisplayName
			}
			if !override.SignalText.IsZero() {
				meta.SignalText = override.SignalText
			}
		}
		result[name] = meta
	}
	return result
}

// Session exposes typed resources, prompts, completions, subscriptions and
// extension APIs to embedding hosts. A session is owned and closed by Client.
// These are protocol APIs; hosts remain responsible for access policy.
func (c *Client) Session(server string) (*sdk.ClientSession, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return nil, fmt.Errorf("MCP client is closed")
	}
	conn := c.servers[server]
	if conn == nil {
		return nil, fmt.Errorf("MCP server %q is not connected", server)
	}
	return conn.session, nil
}

func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
	c.mu.RLock()
	tool := c.tools[name]
	c.mu.RUnlock()
	if tool == nil {
		return nil, fmt.Errorf("MCP tool %q is unavailable or ambiguous; configure toolPrefix for colliding servers", name)
	}
	return c.callTool(ctx, tool, args)
}

func (c *Client) callTool(ctx context.Context, tool *MCPTool, args map[string]any) (*CallToolResult, error) {
	c.mu.RLock()
	conn := c.servers[tool.serverName]
	name := tool.mcpTool.Name
	if conn != nil {
		name = conn.config.ToolPrefix + name
	}
	current := c.tools[name]
	allowed := !c.closed && conn != nil && current != nil && current.serverName == tool.serverName
	c.mu.RUnlock()
	if !allowed {
		return nil, fmt.Errorf("MCP tool %q is no longer available", name)
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultToolCallTimeout)
		defer cancel()
	}
	result, err := conn.session.CallTool(ctx, &sdk.CallToolParams{Name: tool.mcpTool.Name, Arguments: args})
	if err != nil {
		return nil, fmt.Errorf("call MCP tool %q: %w", name, err)
	}
	if result.NeedsInput() {
		return nil, fmt.Errorf("MCP tool %q requires host input; configure an interaction handler", name)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var converted CallToolResult
	if err := json.Unmarshal(raw, &converted); err != nil {
		return nil, fmt.Errorf("decode MCP result: %w", err)
	}
	return &converted, nil
}

func (c *Client) Close() error {
	c.lifecycle.Lock()
	defer c.lifecycle.Unlock()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	connections := c.servers
	c.servers = map[string]*ServerConnection{}
	c.tools = map[string]*MCPTool{}
	c.mu.Unlock()
	var errs []error
	for _, conn := range connections {
		if err := conn.session.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

var _ tools.MCPClient = (*Client)(nil)

func (t *MCPTool) exposedName() string {
	t.client.mu.RLock()
	defer t.client.mu.RUnlock()
	return t.client.config.McpServers[t.serverName].ToolPrefix + t.mcpTool.Name
}

func (t *MCPTool) Declaration() *ai.FunctionDeclaration {
	// Convert MCP schema to Genie schema
	params := convertMCPSchemaToGenieSchema(t.mcpTool.InputSchema)

	declaration := &ai.FunctionDeclaration{
		Name:        t.exposedName(),
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
	if t.mcpTool.OutputSchema != nil {
		declaration.Response = &ai.Schema{JSONSchema: t.mcpTool.OutputSchema}
	}
	return declaration
}

// Handler returns the execution handler for the MCP tool
func (t *MCPTool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, params map[string]interface{}) (ai.ToolOutput, error) {
		// Call the MCP tool through the client
		result, err := t.client.callTool(ctx, t, params)
		if err != nil {
			return ai.ToolOutput{}, err
		}

		details := map[string]any{
			"content": mcpContentDetails(result.Content),
			"isError": result.IsError,
		}
		modelContent := make([]ai.ToolContent, 0, len(result.Content)+1)
		conversionFailed := false
		for _, content := range result.Content {
			blocks, err := mcpContentBlocks(content, t.client.maxBlobBytes)
			if err != nil {
				conversionFailed = true
				modelContent = append(modelContent, ai.TextContent{Text: fmt.Sprintf("[invalid MCP %s content: %v]", content.Type, err)})
				continue
			}
			modelContent = append(modelContent, blocks...)
		}
		if result.StructuredContent != nil {
			details["structuredContent"] = result.StructuredContent
			modelContent = append(modelContent, ai.JSONContent{Value: result.StructuredContent})
		}
		if conversionFailed {
			details["contentConversionError"] = true
		}
		return ai.ToolOutput{Content: modelContent, Details: details, IsError: result.IsError || conversionFailed}, nil
	}
}

func mcpContentDetails(contents []Content) []Content {
	details := make([]Content, len(contents))
	for i, content := range contents {
		content.Data = ""
		if content.Resource != nil {
			resource := *content.Resource
			resource.Blob = ""
			content.Resource = &resource
		}
		details[i] = content
	}
	return details
}

func mcpContentBlocks(content Content, maxBlobBytes int) ([]ai.ToolContent, error) {
	switch content.Type {
	case "text":
		return []ai.ToolContent{ai.TextContent{Text: content.Text}}, nil
	case "image", "audio":
		data, err := decodeMCPBlob(content.Data, maxBlobBytes)
		if err != nil {
			return nil, fmt.Errorf("decode base64: %w", err)
		}
		return []ai.ToolContent{ai.BlobContent{MIMEType: content.MIMEType, Data: data, Name: content.Name}}, nil
	case "resource":
		if content.Resource == nil {
			return nil, fmt.Errorf("missing resource")
		}
		if content.Resource.Text != "" {
			return []ai.ToolContent{ai.TextContent{Text: content.Resource.Text}}, nil
		}
		data, err := decodeMCPBlob(content.Resource.Blob, maxBlobBytes)
		if err != nil {
			return nil, fmt.Errorf("decode resource blob: %w", err)
		}
		return []ai.ToolContent{ai.BlobContent{
			MIMEType: content.Resource.MIMEType,
			Data:     data,
			Name:     content.Resource.URI,
		}}, nil
	default:
		return []ai.ToolContent{ai.JSONContent{Value: content}}, nil
	}
}

func decodeMCPBlob(encoded string, maxBytes int) ([]byte, error) {
	decodedLen := base64.StdEncoding.DecodedLen(len(encoded))
	if strings.HasSuffix(encoded, "==") {
		decodedLen -= 2
	} else if strings.HasSuffix(encoded, "=") {
		decodedLen--
	}
	if maxBytes >= 0 && decodedLen > maxBytes {
		return nil, fmt.Errorf("decoded blob may exceed the %d-byte attachment safety limit", maxBytes)
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	if maxBytes >= 0 && len(data) > maxBytes {
		return nil, fmt.Errorf("decoded blob exceeds the %d-byte attachment safety limit", maxBytes)
	}
	return data, nil
}

// FormatOutput formats the tool result for display
func (t *MCPTool) FormatOutput(result map[string]interface{}) string {
	raw, ok := result["content"]
	if !ok {
		return fmt.Sprintf("Tool result: %v", result)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Sprintf("Tool result: %v", result)
	}
	var content []Content
	if err := json.Unmarshal(data, &content); err != nil {
		return fmt.Sprintf("Tool result: %v", result)
	}

	var output string
	for _, item := range content {
		if item.Text != "" {
			output += item.Text + "\n"
		}
	}

	return output
}

// convertMCPSchemaToGenieSchema converts MCP tool schema to Genie's ai.Schema
func convertMCPSchemaToGenieSchema(mcpSchema ToolSchema) *ai.Schema {
	if mcpSchema.Raw != nil {
		return &ai.Schema{Type: ai.TypeObject, JSONSchema: mcpSchema.Raw}
	}
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
