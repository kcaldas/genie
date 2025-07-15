# MCP (Model Context Protocol) Integration

This package provides Model Context Protocol (MCP) support for Genie, allowing it to consume external MCP servers and their tools seamlessly alongside native Genie tools.

## Overview

The MCP integration enables Genie to:
- Read and parse `.mcp.json` configuration files (compatible with Claude Code format)
- Act as an MCP client to consume external MCP servers
- Seamlessly integrate MCP tools with Genie's existing tool system
- Support all MCP transport types (stdio, SSE, HTTP)

## Architecture

### MCP Tool Integration Flow

**MCP tools work identically to internal tools through Genie's unified tool interface:**

#### 1. Tool Interface Implementation
Both internal and MCP tools implement the same `Tool` interface:

```go
type Tool interface {
    Declaration() *ai.FunctionDeclaration  // Schema for the LLM
    Handler() ai.HandlerFunc               // Execution function
    FormatOutput(result map[string]interface{}) string
}
```

#### 2. MCP Tool Handler Function
Our `MCPTool` implements `Handler()` just like internal tools:

```go
func (t *MCPTool) Handler() ai.HandlerFunc {
    return func(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
        // This function bridges Genie's tool system to MCP protocol
        result, err := t.client.CallTool(ctx, t.mcpTool.Name, params)
        if err != nil {
            return nil, err
        }
        
        // Convert MCP response to Genie format
        return map[string]interface{}{
            "content":  contentMaps,
            "isError": result.IsError,
        }, nil
    }
}
```

#### 3. Integration with Prompt System
When a prompt is loaded, **both internal and MCP tools are processed identically** in the prompt loader:

```go
// Add tools to prompt
for _, tool := range toolsList {  // toolsList contains BOTH internal and MCP tools
    declaration := tool.Declaration()
    prompt.Functions = append(prompt.Functions, declaration)    // LLM sees the schema
    
    // Wrap handler with events
    originalHandler := tool.Handler()                           // Gets the handler function
    wrappedHandler := l.wrapHandlerWithEvents(declaration.Name, originalHandler)
    prompt.Handlers[declaration.Name] = wrappedHandler         // LLM can call this
}
```

#### 4. LLM Execution
When the LLM wants to call a tool:

1. **LLM sees the tool** in `prompt.Functions` (same for internal and MCP)
2. **LLM calls the tool** by name with parameters
3. **Genie looks up the handler** in `prompt.Handlers[toolName]`
4. **Handler executes:**
   - **Internal tool**: Directly executes the function
   - **MCP tool**: Sends JSON-RPC request to MCP server, gets response, converts format

#### 5. The Magic Bridge
The `MCPTool.Handler()` acts as a **bridge** that:
- Takes Genie's standard `ai.HandlerFunc` signature
- Converts parameters to MCP JSON-RPC format
- Sends request to external MCP server
- Receives MCP response
- Converts response back to Genie's expected format

**From the LLM's perspective, there's no difference between internal and MCP tools!** They all have the same schema format and handler signature.

### Example in Action:
```
LLM: "I want to echo 'hello'"
Genie: Looks up "echo" in prompt.Handlers
Handler: MCPTool.Handler() function
       → Sends {"method": "tools/call", "params": {"name": "echo", "arguments": {"text": "hello"}}}
       → External MCP server processes request
       → Returns {"content": [{"type": "text", "text": "Echo: hello"}], "isError": false}
       → MCPTool converts to Genie format and returns
LLM: Receives result in standard Genie format
```

## MCP Specification Compliance

This implementation is **100% compliant with the MCP 2024-11-05 specification**:

### JSON-RPC 2.0 Compliance
- ✅ All requests include `jsonrpc: "2.0"`
- ✅ All requests have non-null `id` fields
- ✅ All responses include `jsonrpc: "2.0"` and matching `id`

### Initialize Protocol
- ✅ **Client Request Format:**
  ```json
  {
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {"name": "genie", "version": "1.0.0"}
    }
  }
  ```

### Tools List & Call
- ✅ **Tools List**: Proper `tools/list` request/response format
- ✅ **Tools Call**: Proper `tools/call` request/response format
- ✅ **Content Format**: Spec-compliant content arrays with `type` and `text` fields
- ✅ **Error Handling**: Proper `isError` field always present

## Configuration

### .mcp.json Format
Place a `.mcp.json` file in your project root with the following format:

```json
{
  "mcpServers": {
    "server-name": {
      "command": "/path/to/server",
      "args": ["--arg1", "value1"],
      "env": {
        "ENV_VAR": "value"
      }
    },
    "sse-server": {
      "type": "sse",
      "url": "https://example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${API_KEY}"
      }
    }
  }
}
```

### Environment Variable Support
Full support for environment variable expansion using `${VAR:-default}` syntax.

### Transport Types
- **stdio**: Default transport using command execution
- **sse**: Server-Sent Events transport
- **http**: HTTP transport

## Key Components

### Files
- `config.go` - Configuration parsing and environment variable expansion
- `protocol.go` - MCP protocol types and JSON-RPC 2.0 implementation
- `transport.go` - Transport layer abstraction (stdio/SSE/HTTP)
- `client.go` - MCP client implementation and tool adapter
- `factory.go` - Client factory for dependency injection

### Testing
- `config_test.go` - Configuration parsing tests
- `integration_test.go` - Full client-server integration tests
- `test_server.go` - Simple MCP server for testing
- `test_server_test.go` - Server protocol compliance tests
- `spec_compliance_test.go` - Comprehensive MCP specification compliance tests

## Usage

### Automatic Discovery
When Genie starts, it automatically:
1. Looks for `.mcp.json` files in the project root
2. Connects to configured MCP servers
3. Discovers available tools
4. Registers them in the tool registry
5. Makes them available to the LLM

### Manual Integration
```go
// Create MCP client
client, err := mcp.NewMCPClientFromConfig()
if err != nil {
    return err
}

// Connect to servers
ctx := context.Background()
if err := client.ConnectToServers(ctx); err != nil {
    return err
}

// Get tools
tools := client.GetTools()
```

## Testing

Run the comprehensive test suite:
```bash
go test ./pkg/mcp -v
```

This includes:
- Configuration parsing tests
- Protocol compliance tests
- Client-server integration tests
- MCP specification compliance verification

## Integration with Genie

The MCP package integrates seamlessly with Genie through:

1. **Dependency Injection**: `ProvideMCPClient()` in `internal/di/wire.go`
2. **Tool Registry**: `NewRegistryWithMCP()` combines native and MCP tools
3. **Event System**: MCP tools use the same event bus as native tools
4. **Configuration**: Automatic discovery of `.mcp.json` files

This makes MCP tools completely transparent to users - they just appear as additional tools in the system.