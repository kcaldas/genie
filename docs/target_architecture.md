# Genie Target Architecture

## Overview

This document outlines the target architecture for Genie, designed to provide clean separation of concerns between the CLI interface and core functionality. This architecture is inspired by Claude Code's design patterns and aims to create a scalable, testable, and maintainable codebase.

## Current Problems

Our current architecture has several issues that need to be addressed:

1. **CLI-LLM Tight Coupling**: Commands like `ask` directly instantiate LLM clients (`vertex.NewClient()`)
2. **No Abstraction Layer**: Commands have direct knowledge of specific LLM implementations
3. **Hard to Test**: CLI commands require environment setup and can't be easily mocked
4. **Not Scalable**: Adding new commands means duplicating LLM setup logic
5. **No Session Management**: Each command creates new client instances, no persistent context
6. **No Tool Orchestration**: No way for LLM to intelligently use multiple tools together
7. **Configuration Scattered**: Model selection, API keys, and settings spread across packages

## Target Architecture

### High-Level Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI Layer                            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
│  │ cmd/ask     │ │ cmd/config  │ │ cmd/doctor  │ ...      │
│  │             │ │             │ │             │          │
│  │ - Arg parse │ │ - Settings  │ │ - Health    │          │
│  │ - Validation│ │ - Show/Set  │ │ - Validate  │          │
│  │ - API calls │ │ - Validate  │ │ - Report    │          │
│  └─────────────┘ └─────────────┘ └─────────────┘          │
└─────────────────────┬───────────────────────────────────────┘
                      │ calls
┌─────────────────────▼───────────────────────────────────────┐
│                    Core API                                 │
│                 (pkg/api)                                   │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Session Manager                         │  │
│  │                (pkg/session)                         │  │
│  │  - Session lifecycle & coordination                  │  │
│  │  - Broadcasting to History & Context managers       │  │
│  │  - Configuration & settings management              │  │
│  │  - Multi-session support                            │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              History Manager                         │  │
│  │                (pkg/history)                         │  │
│  │  - Complete conversation record                      │  │
│  │  - Permanent audit trail                            │  │
│  │  - Analytics & debugging data                       │  │
│  │  - Independent storage lifecycle                    │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Context Manager                         │  │
│  │                (pkg/context)                         │  │
│  │  - Optimized LLM context                            │  │
│  │  - Token-aware filtering                            │  │
│  │  - Context compression & summarization              │  │
│  │  - Performance-optimized storage                    │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Tool Orchestrator                       │  │
│  │                (pkg/orchestrator)                    │  │
│  │  - Tool discovery & registration                    │  │
│  │  - Permission handling & user approval              │  │
│  │  - Tool execution coordination                      │  │
│  │  - Multi-tool workflow management                   │  │
│  │  - Result aggregation                               │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              LLM Manager                            │  │
│  │                (pkg/llm)                            │  │
│  │  - Provider abstraction (Vertex, OpenAI, etc.)     │  │
│  │  - Model selection & configuration                  │  │
│  │  - Response processing & streaming                  │  │
│  │  - Function calling coordination                    │  │
│  │  - Error handling & retries                         │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              MCP Integration                        │  │
│  │                (pkg/mcp)                            │  │
│  │  - MCP Server: Expose Genie tools to external      │  │
│  │  - MCP Client: Use external MCP servers            │  │
│  │  - .mcp.json configuration parsing                 │  │
│  │  - Transport handling (stdio, HTTP, WebSocket)     │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────┬───────────────────────────────────────┘
                      │ uses
┌─────────────────────▼───────────────────────────────────────┐
│                  Tool System                                │
│                 (pkg/tools)                                 │
│                                                             │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
│  │ fileops     │ │ git         │ │ search      │ ...      │
│  │             │ │             │ │             │          │
│  │ - Read      │ │ - Status    │ │ - Grep      │          │
│  │ - Write     │ │ - Commit    │ │ - Find      │          │
│  │ - Edit      │ │ - Diff      │ │ - Context   │          │
│  │ - MultiEdit │ │ - Log       │ │ - AST       │          │
│  │ - Glob      │ │ - Branch    │ │ - Symbols   │          │
│  │ - LS        │ │ - Merge     │ │ - Deps      │          │
│  └─────────────┘ └─────────────┘ └─────────────┘          │
│                                                             │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
│  │ build       │ │ test        │ │ lint        │ ...      │
│  │             │ │             │ │             │          │
│  │ - Go build  │ │ - Go test   │ │ - Golint    │          │
│  │ - Make      │ │ - Pytest    │ │ - ESLint    │          │
│  │ - Docker    │ │ - Jest      │ │ - Prettier  │          │
│  │ - Custom    │ │ - Custom    │ │ - Custom    │          │
│  └─────────────┘ └─────────────┘ └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

## Detailed Component Design

### Core API (pkg/api)

The central API provides a unified interface for all Genie functionality:

```go
// GenieAPI is the main interface for all Genie operations
type GenieAPI interface {
    // Core chat functionality
    Ask(ctx context.Context, prompt string, opts ...Option) (*Response, error)
    Chat(ctx context.Context, messages []Message, opts ...Option) (*Response, error)
    
    // Session management
    NewSession(ctx context.Context, config SessionConfig) (Session, error)
    GetSession(ctx context.Context, id string) (Session, error)
    ListSessions(ctx context.Context) ([]SessionInfo, error)
    
    // Tool operations
    ExecuteTool(ctx context.Context, tool string, args map[string]any) (*ToolResult, error)
    ListTools(ctx context.Context) ([]ToolInfo, error)
    
    // Configuration
    GetConfig(ctx context.Context) (*Config, error)
    UpdateConfig(ctx context.Context, updates ConfigUpdates) error
    
    // Health and diagnostics
    Health(ctx context.Context) (*HealthStatus, error)
    Doctor(ctx context.Context) (*DiagnosticReport, error)
}
```

### Session Manager (pkg/session)

Orchestrates session lifecycle and coordinates between History and Context managers. Acts as a broadcaster that sends conversation interactions to both independent storage systems while managing session-level concerns like configuration and settings.

### History Manager (pkg/history)

Maintains the complete, unfiltered conversation record for each session. Serves as the permanent audit trail containing every user message, assistant response, tool call, and metadata. Independent storage lifecycle allows for different retention policies and analytics requirements.

### Context Manager (pkg/context)

Manages the optimized conversation context used for LLM interactions. Stores the filtered, token-aware subset of conversation history that gets sent to the language model. Future evolution will include context compression, summarization, and performance optimizations.

## Session-History-Context Relationship

The separation of concerns between Session, History, and Context managers enables distinct responsibilities:

- **Session Manager**: Coordinates and broadcasts interactions to both storage systems, manages session lifecycle and configuration
- **History Manager**: Complete record storage with independent lifecycle, supports analytics and audit requirements
- **Context Manager**: Performance-optimized storage for LLM consumption, future context filtering and optimization
- **Independent Storage**: Each manager maintains its own storage, allowing for different retention policies and optimization strategies
- **Broadcasting Pattern**: Session sends each interaction to both managers simultaneously, ensuring consistency without tight coupling
- **Future Evolution**: Context Manager can implement intelligent filtering, summarization, and token management without affecting the complete history

### Tool Orchestrator (pkg/orchestrator)

Coordinates tool execution and manages permissions:

```go
type Orchestrator interface {
    // Tool registration
    RegisterTool(tool Tool) error
    UnregisterTool(name string) error
    GetTool(name string) (Tool, error)
    ListTools() []ToolInfo
    
    // Tool execution
    Execute(ctx context.Context, name string, args map[string]any) (*ToolResult, error)
    ExecuteWorkflow(ctx context.Context, workflow Workflow) (*WorkflowResult, error)
    
    // Permission management
    CheckPermission(ctx context.Context, tool string, args map[string]any) error
    RequestPermission(ctx context.Context, request PermissionRequest) error
    
    // LLM function calling integration
    GetFunctionDeclarations() []FunctionDeclaration
    HandleFunctionCall(ctx context.Context, call FunctionCall) (*FunctionResult, error)
}

type Tool interface {
    Name() string
    Description() string
    Parameters() ParameterSchema
    RequiresPermission() bool
    Execute(ctx context.Context, args map[string]any) (*ToolResult, error)
}
```

### LLM Manager (pkg/llm)

Abstracts LLM providers and handles model management:

```go
type LLMManager interface {
    // Provider management
    RegisterProvider(provider Provider) error
    GetProvider(name string) (Provider, error)
    ListProviders() []ProviderInfo
    
    // Model operations
    Chat(ctx context.Context, messages []Message, opts ...Option) (*Response, error)
    Stream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamEvent, error)
    
    // Function calling
    ChatWithFunctions(ctx context.Context, messages []Message, functions []FunctionDeclaration, opts ...Option) (*Response, error)
    
    // Configuration
    SetDefaultModel(model string) error
    GetAvailableModels() []ModelInfo
}

type Provider interface {
    Name() string
    SupportedModels() []ModelInfo
    Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error)
    Stream(ctx context.Context, request ChatRequest) (<-chan StreamEvent, error)
    Validate(config ProviderConfig) error
}
```

### Tool System (pkg/tools)

Each tool category gets its own package with a common interface:

```go
// Common tool interface
type Tool interface {
    Name() string
    Description() string
    Parameters() ParameterSchema
    RequiresPermission() bool
    Execute(ctx context.Context, args map[string]any) (*ToolResult, error)
}

// File operations tools (pkg/tools/fileops)
type FileOpsTools struct {
    ReadTool      Tool
    WriteTool     Tool
    EditTool      Tool
    MultiEditTool Tool
    GlobTool      Tool
    LSTool        Tool
}

// Git tools (pkg/tools/git)
type GitTools struct {
    StatusTool Tool
    DiffTool   Tool
    CommitTool Tool
    LogTool    Tool
    BranchTool Tool
}

// Search tools (pkg/tools/search)
type SearchTools struct {
    GrepTool    Tool
    FindTool    Tool
    ContextTool Tool
    ASTTool     Tool
    SymbolTool  Tool
}
```

## Benefits of This Architecture

### 1. Clean Separation of Concerns
- **CLI Layer**: Only handles argument parsing, validation, and API calls
- **Core API**: Business logic, orchestration, and coordination
- **Tool System**: Isolated, testable, and reusable components

### 2. Testability
- Mock the Core API for CLI testing
- Mock individual tools for orchestrator testing
- Mock LLM providers for response testing
- Integration tests can use real components

### 3. Reusability
- Core API can be used by CLI, web UI, IDE plugins, or other interfaces
- Tools can be reused across different commands and contexts
- Session management works for any interface

### 4. Scalability
- Easy to add new commands (just call the API)
- Easy to add new tools (register with orchestrator)
- Easy to add new LLM providers (implement Provider interface)
- Easy to add new transports (MCP, HTTP, etc.)

### 5. MCP Integration
- Core API naturally exposes tools via MCP protocol
- Can act as both MCP server (expose tools) and client (use external tools)
- .mcp.json configuration integrates seamlessly

### 6. Session Management
- Persistent context across interactions
- Cost tracking and memory management
- Multi-session support for parallel workflows

### 7. Configuration Management
- Centralized configuration with validation
- Multi-level settings (user, project, enterprise)
- Hot reloading and dynamic updates

## Migration Strategy

### Phase 1: Foundation ✅ (Completed)
1. **✅ Implement basic Session Manager** (`pkg/session`)
2. **✅ Create Context Manager** (`pkg/context`) 
3. **✅ Create History Manager** (`pkg/history`)
4. **✅ Wire dependency injection** for all managers
5. **✅ Session broadcasting** to both History and Context managers

### Phase 2: Tool Migration
1. **Move fileops** to `pkg/tools/fileops`
2. **Move git operations** to `pkg/tools/git`
3. **Create search tools** in `pkg/tools/search`
4. **Implement Tool Orchestrator** (`pkg/orchestrator`)

### Phase 3: LLM Abstraction
1. **Create LLM Manager** (`pkg/llm`)
2. **Refactor Vertex client** to implement Provider interface
3. **Add support for other providers** (OpenAI, Anthropic, etc.)
4. **Integrate function calling** with tool orchestrator

### Phase 4: CLI Refactor
1. **Refactor ask command** to use Core API
2. **Add session commands** (new, list, switch, etc.)
3. **Add tool commands** (list, execute, etc.)
4. **Add config commands** (get, set, validate, etc.)

### Phase 5: MCP Integration
1. **Implement MCP server** (`pkg/mcp/server`)
2. **Implement MCP client** (`pkg/mcp/client`)
3. **Add .mcp.json support**
4. **Integrate external MCP tools**

## Implementation Guidelines

### Error Handling
- Use wrapped errors with context
- Consistent error types across components
- Graceful degradation when tools are unavailable

### Configuration
- Use interfaces for configuration sources
- Support environment variables, files, and CLI flags
- Validate configuration at startup and runtime

### Logging
- Use structured logging throughout
- Component-specific loggers with context
- Configurable log levels per component

### Testing
- Unit tests for each component
- Integration tests for component interactions
- End-to-end tests for full workflows
- Benchmarks for performance-critical paths

### Documentation
- API documentation with examples
- Tool documentation with usage patterns
- Architecture documentation (this file)
- Migration guides for major changes

## Future Considerations

### Plugin System
- Dynamic tool loading
- Plugin marketplace
- Sandboxed execution
- Plugin API versioning

### Performance Optimization
- Tool result caching
- Parallel tool execution
- Streaming responses
- Context compression

### Security
- Tool permission system
- Audit logging
- Secure configuration storage
- Network security

### Enterprise Features
- Multi-tenant support
- Enterprise authentication
- Policy enforcement
- Usage analytics

## Related Documents

- [Implementation Phases](./pm/implementation-phases.norg) - Development roadmap
- [Tools Inventory](./pm/tools-inventory.norg) - Complete tool specifications
- [MCP Features](./pm/mcp-features.norg) - MCP integration requirements
- [CLAUDE.md](../CLAUDE.md) - Project guidance and conventions

---

*This document is a living specification and will be updated as we learn more about the requirements and implementation details.*