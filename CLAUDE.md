# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Genie is a Go-based AI coding assistant tool similar to Claude Code, initially using Gemini as the LLM backend. The project aims to provide an interactive CLI tool for software engineering tasks.

## Development Workflow

- Prefer Test-Driven Development (TDD) style workflow when possible
- Recommended TDD approach: Run tests > Change tests > See failure > Implement code
- Start renaming or refactoring by first modifying the tests to reflect the desired changes
- Last call our context ctx to avoid conflicting with the golang context

## Development Commands

Since this is a Go project, common commands will likely include:

```bash
# Initialize Go module (if not done)
go mod init genie

# Build the project
go build -o genie ./cmd/genie

# Run tests
go test ./...

# Run with race detection
go test -race ./...

# Install dependencies
go mod tidy

# Run the CLI tool
./genie

# Configuration management
genie config list
genie config get <key>
genie config set <key> <value>

# Health and diagnostics
genie doctor
genie status

# Cost tracking
genie cost --session
genie cost --history
```

## Architecture Guidelines

As a CLI tool inspired by Claude Code, the architecture should consider:

- **CLI Interface**: Use a library like cobra or urfave/cli for command structure
- **LLM Integration**: Abstract LLM providers (starting with Gemini) behind interfaces for future extensibility
- **Tool System**: Implement a plugin-like system for various development tools (file operations, git, search, etc.)
- **Session Management**: Handle conversation context and memory
- **Configuration**: Support for API keys, model selection, user preferences, and .mcp.json files
- **MCP Integration**: Dual server/client support for Model Context Protocol
- **Cost Awareness**: Built-in token tracking and cost optimization
- **Health Monitoring**: Comprehensive diagnostics and issue reporting
- **Configuration Management**: Multi-level settings with validation
- **Context Management**: Automatic context optimization and monitoring

## Unix Tool Conventions

Genie must follow traditional Unix/POSIX tool conventions and integrate seamlessly with shell environments:

### Standard I/O Behavior
- **stdin**: Accept input from pipes and redirections (`echo "code" | genie fix`)
- **stdout**: Write primary output that can be piped to other commands
- **stderr**: Use for error messages, warnings, and progress indicators
- Support for input/output redirection (`genie analyze < file.go > report.txt`)

### Command-line Interface Standards
- Follow standard option formats: `-h` (short), `--help` (long)
- Support `--version` for version information
- Use standard exit codes (0 for success, non-zero for errors)
- Implement proper signal handling (SIGINT, SIGTERM)
- Support `--quiet`/`-q` for minimal output and `--verbose`/`-v` for detailed output

### Shell Integration
- **Piping**: Work as both source and sink in pipelines
  - `find . -name "*.go" | genie review`
  - `genie generate tests | tee tests.go`
- **Command Substitution**: Support backticks and `$()` syntax
  - `vim $(genie suggest-files)`
- **Environment Variables**: Respect standard env vars like `$EDITOR`, `$PAGER`
- **Job Control**: Handle background execution and process groups properly

### File Operations
- Support `-` as filename to represent stdin/stdout
- Handle file globs and wildcards when appropriate
- Respect file permissions and ownership
- Support atomic file operations where possible

### Text Processing Compatibility
- Process text line-by-line when appropriate
- Support common text filters and transformations
- Handle different line endings (Unix/Windows/Mac)
- Work with standard encoding (UTF-8) and handle encoding issues gracefully

## Key Components Expected

- `cmd/` - CLI entry points and command definitions
- `pkg/` - Core business logic and reusable packages
- `internal/` - Private application code
- `tools/` - Individual tool implementations (file ops, git, search, etc.)
- `llm/` - LLM provider abstractions and implementations
- `mcp/` - Model Context Protocol server and client implementation
- `config/` - Multi-level configuration management
- `diagnostic/` - Health monitoring and troubleshooting tools
- `cost/` - Usage tracking and cost optimization

## Built-in Commands

Genie includes built-in slash commands for common operations:
- `/help` - Show available commands
- `/status` - System status and health
- `/config` - Configuration management
- `/cost` - Usage and cost tracking
- `/doctor` - Health diagnostics
- `/clear` - Clear conversation history
- `/compact` - Reduce context size
- `/bug` - Report issues
- `/permissions` - Manage tool permissions

## Configuration Files

Genie uses a hierarchical configuration system:
- `~/.genie/settings.json` - User global settings
- `.genie/settings.json` - Project shared settings  
- `.genie/settings.local.json` - Project personal settings
- Enterprise policy files for organizational controls

## MCP Requirements

Genie must serve as both MCP server and client:

### MCP Server Mode
- Expose Genie tools via MCP protocol
- Provide file system and git resources
- Support stdio, HTTP, and WebSocket transports
- Implement proper authentication and authorization

### MCP Client Mode
- Connect to external MCP servers
- Parse .mcp.json configuration files
- Integrate remote tools seamlessly with local tools
- Handle multiple concurrent MCP connections

### Configuration Format
- Support .mcp.json files for MCP server/client configuration
- Environment variable substitution in configuration
- Hot reload of configuration changes
- Validation of MCP configuration syntax

## Development Notes

- Follow standard Go project layout conventions
- Use interfaces to abstract external dependencies (LLM providers, file system, MCP, etc.)
- Implement proper error handling with wrapped errors
- Consider using context.Context for cancellation and timeouts
- Plan for concurrent operations where appropriate
- Design MCP integration to be optional but seamless when enabled

## Code Conventions

### File Naming
- Use descriptive names that match the primary type/interface: `session_manager.go` for SessionManager
- Use `_test.go` suffix for test files: `session_manager_test.go`
- Avoid generic names like `manager.go` - be specific about what is being managed

### Dependency Injection with Wire
- **Factory Functions**: Use `NewXxx()` factory functions in packages to create instances
- **Wire Providers**: Wire provider functions should be in `internal/di/wire.go`, not in individual packages
- **Interface Returns**: Factory functions should return interfaces, not concrete types
- **No Framework Testing**: Don't write tests for Wire injection itself - test actual functionality instead
- **Example**:
  ```go
  // In pkg/session/session_manager.go
  func NewSessionManager() Manager {
      return &InMemoryManager{...}
  }
  
  // In internal/di/wire.go
  func ProvideSessionManager() session.Manager {
      return session.NewSessionManager()
  }
  ```

### TDD Workflow Preference
- **Always prefer Test-Driven Development (TDD)** when implementing new features or refactoring
- **TDD Cycle**: Write failing test → Make it pass → Refactor → Repeat

#### For New Features:
1. Write failing test for desired behavior
2. Implement minimal code to make test pass
3. Refactor while keeping tests green

#### For API Changes (public interface changes):
1. Run existing tests to ensure they pass
2. Update tests to use new API/naming (tests should fail)
3. Update implementation to make tests pass
4. Refactor if needed while keeping tests green

#### For Internal Refactoring (implementation changes only):
1. Run existing tests to ensure they pass
2. Refactor internal implementation
3. **Keep tests unchanged** - they should continue to pass
4. Tests validate that behavior hasn't changed

- **Benefits**: Ensures we don't break existing functionality and validates that our changes work correctly
```