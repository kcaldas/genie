# Architecture

Genie follows a clean, layered architecture designed for modularity and extensibility.

## Overview

```
┌─────────────────┐    ┌─────────────────┐
│   CLI Client    │    │   TUI Client    │
│   (cmd/cli)     │    │   (cmd/tui)     │
└─────────┬───────┘    └─────────┬───────┘
          │                      │
          └──────────┬───────────┘
                     │
          ┌──────────▼───────────┐
          │    Genie Core        │
          │    (pkg/genie)       │
          └──────────┬───────────┘
                     │
    ┌────────────────┼────────────────┐
    │                │                │
┌───▼───┐       ┌────▼────┐      ┌────▼────┐
│ Tools │       │   AI    │      │ Events  │
│       │       │ Engine  │      │   Bus   │
└───────┘       └─────────┘      └─────────┘
```

## Core Components

### 1. Ultra-thin Main (`cmd/main.go`)
**Purpose:** Route to CLI or TUI based on arguments

```go
func main() {
    if len(os.Args) > 1 {
        // Has arguments → CLI mode
        cli.Execute()
    } else {
        // No arguments → TUI mode
        tui.Start()
    }
}
```

### 2. CLI Client (`cmd/cli/`)
**Purpose:** Handle direct commands
**Framework:** [Cobra](https://github.com/spf13/cobra)

**Features:**
- One-shot commands
- Scriptable interface
- Standard Unix behavior
- Error handling and exit codes

### 3. TUI Client (`cmd/tui/`)
**Purpose:** Interactive terminal interface
**Framework:** [gocui](https://github.com/awesome-gocui/gocui)

**Components:**
- Layout management
- Event handling
- Component system
- State management

### 4. Genie Core (`pkg/genie/`)
**Purpose:** Business logic and orchestration

**Key modules:**
- Session management
- Request processing
- Response handling
- Tool coordination

## Detailed Architecture

### Event-Driven Design

```go
type EventBus interface {
    Publish(topic string, data interface{})
    Subscribe(topic string, handler func(interface{}))
}
```

**Events:**
- `chat.request` - User input received
- `chat.response` - AI response ready
- `tool.call` - Tool execution request
- `tool.result` - Tool execution complete

### Tool System

```go
type Tool interface {
    Declaration() ai.Tool
    Handler() ai.HandlerFunc
    FormatOutput(result map[string]any) string
}
```

**Built-in Tools:**
- `ReadFileTool` - File operations
- `BashTool` - Command execution
- `ThinkingTool` - Advanced reasoning

### Dependency Injection

Uses [Google Wire](https://github.com/google/wire) for compile-time DI:

```go
//go:build wireinject

func InitializeCLI() (*CLI, error) {
    wire.Build(
        NewConfigManager,
        NewAIEngine,
        NewToolRegistry,
        NewCLI,
    )
    return nil, nil
}
```

## Data Flow

### CLI Request Flow
```
User Input → CLI Parser → Genie Core → AI Engine → Tools → Response
```

### TUI Request Flow
```
User Input → TUI Handler → Event Bus → Genie Core → AI Engine → Tools → Event Bus → TUI Display
```

### Tool Execution
```
AI Decision → Tool Registry → Tool Handler → External System → Result → AI Context
```

## Key Design Principles

### 1. Separation of Concerns
- **UI Layer:** CLI/TUI handle user interaction
- **Business Layer:** Genie Core manages logic
- **Service Layer:** AI Engine and Tools

### 2. Interface-Based Design
```go
type AIEngine interface {
    Process(ctx context.Context, request Request) (Response, error)
}

type ToolRegistry interface {
    Register(tool Tool) error
    GetTool(name string) (Tool, bool)
}
```

### 3. Event-Driven Communication
- Loose coupling between components
- Async processing capabilities
- Extensible event system

### 4. Dependency Injection
- Testable components
- Clear dependencies
- Compile-time validation

## Configuration Management

### Layered Configuration
1. Command line flags
2. Environment variables
3. `.env` files
4. Default values

### Config Types
```go
type Config struct {
    // AI Configuration
    ModelName   string
    Temperature float32
    MaxTokens   int32
    
    // TUI Configuration
    Theme           string
    VimMode         bool
    ShowCursor      bool
}
```

## Error Handling

### Structured Errors
```go
type GenieError struct {
    Code    string
    Message string
    Cause   error
}
```

### Error Categories
- **User errors:** Invalid input, missing config
- **System errors:** Network issues, API failures
- **Tool errors:** Command failures, file access

## Testing Strategy

### Unit Tests
- Component isolation
- Mock dependencies
- Interface testing

### Integration Tests
- End-to-end workflows
- Tool interactions
- Configuration scenarios

### Test Structure
```
pkg/
├── genie/
│   ├── core.go
│   ├── core_test.go
│   └── integration_test.go
└── tools/
    ├── bash.go
    └── bash_test.go
```

## Extensibility

### Adding New Tools
```go
type MyTool struct {}

func (t *MyTool) Declaration() ai.Tool {
    return ai.Tool{
        Name: "my-tool",
        Description: "Does something useful",
        Parameters: /* JSON schema */,
    }
}

func (t *MyTool) Handler() ai.HandlerFunc {
    return func(ctx context.Context, args map[string]any) (map[string]any, error) {
        // Implementation
    }
}
```

### Adding New UI Components
```go
type NewComponent struct {
    BaseComponent
}

func (c *NewComponent) Render(g *gocui.Gui) error {
    // Rendering logic
}

func (c *NewComponent) HandleInput(g *gocui.Gui, v *gocui.View) error {
    // Input handling
}
```

## Performance Considerations

### Memory Management
- Conversation history limits
- Tool result caching
- Efficient UI updates

### Concurrency
- AI requests in goroutines
- Non-blocking UI updates
- Tool execution parallelization

### Caching
- Tool declarations
- Configuration values
- UI layouts

## Security

### Input Validation
- Command injection prevention
- Path traversal protection
- API key sanitization

### Tool Execution
- Sandboxed environments (Docker)
- Permission checks
- Resource limits

### Data Handling
- No persistent storage of sensitive data
- Memory clearing for API keys
- Secure communication channels
