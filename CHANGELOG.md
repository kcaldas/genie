# Changelog

All notable changes to the Genie project will be documented in this file, organized by date.

**Project Duration: 8 days** (Started: 2025-06-16)

## How to Update This Changelog

When adding new entries, simply say "update changelog" and I will:
1. Add a new date section for today (if it doesn't exist)
2. Include all changes made since the last entry
3. Organize changes by type: Added, Fixed, Changed, etc.
4. Update the day counter at the top
5. Focus only on key features and technical changes (no ideas, minor tweaks, or organizational changes)

Format: Use `## YYYY-MM-DD` for date headers, with most recent dates at the top.

## 2025-06-23

### Added
- **Diff Preview System** - File creation/modification with unified diff preview and confirmation
- **Response Handler System** - Structured LLM response processing for automated file operations
- **Chain Decision Nodes** - Workflow branching system for conditional execution paths
- **Centralized Model Configuration** - Unified configuration management for AI models
- **Vi-style Navigation** - j/k keys for confirmation dialogs

### Fixed
- **Auto-accept Confirmations** - Fixed hanging with --accept-all flag
- **Function Call Recursion** - Eliminated hanging with clean final calls without tools
- **Token Limits** - Removed restrictive 1000-token limits preventing file generation

### Changed
- **Function Call Limits** - Increased to 8 for better complex task handling

## 2025-06-22

### Added
- **Interactive Tool Confirmation System** - User approval workflow for tool execution
- **Tool-specific Output Formatting** - Custom display formatting for each tool type
- **Genie TestFixture** - Centralized testing harness eliminating boilerplate
- **Tool Registry** - Centralized management for extensible tool system

### Fixed
- **JSON Leakage** - Eliminated raw JSON in Gemini responses

### Changed
- **Architecture Reorganization** - Ultra-thin main with separate CLI/TUI clients

## 2025-06-21

### Added
- **Genie Service Layer** - Event-driven architecture abstraction

## 2025-06-20

### Added
- **ESC Key Cancellation** - Clean cancellation support in REPL
- **Context.Context Support** - Full cancellation chain from Gen interface to Vertex client
- **Local Caching** - PromptLoader performance improvements
- **Tool Registry** - Centralized extensible tool management
- **Required Tools Declaration** - Prompts can specify needed tools
- **Recursive ListFiles** - Depth capability with .gitignore support
- **Service Account Support** - Vertex AI authentication improvements

### Changed
- **Architecture Simplification** - Removed PromptExecutor pattern

## 2025-06-19

### Added
- **Environment Variables** - .env file support

## 2025-06-18

### Added
- **Multi-tool Function Calling** - Comprehensive AI function calling system
- **Event-driven Tool Execution** - Messaging system for tool operations

## 2025-06-17

### Added
- **Session-History-Context Architecture** - TDD implementation
- **Event Bus Architecture** - Replaced channel broadcasting
- **Bubble Tea REPL** - Foundation with dual-mode support
- **YAML Prompts** - Centralized with shared executor
- **Markdown Rendering** - Rich AI responses with Glamour
- **Persistent Command History** - Project-specific tracking

## 2025-06-16

### Added
- **Cobra CLI Framework** - TDD implementation
- **Structured Logging** - slog integration
- **Google Wire** - Dependency injection system
- **Vertex AI Integration** - LLM abstraction in pkg/llm/vertex

---

## Project Information

**Genie** is a Go-based AI coding assistant tool using Gemini as the LLM backend, providing both CLI commands and an interactive TUI for software engineering tasks.

### Architecture
- **Ultra-thin Main** (`cmd/main.go`) - Mode detection and routing
- **CLI Client** (`cmd/cli/`) - Direct command execution
- **TUI Client** (`cmd/tui/`) - Interactive REPL experience  
- **Genie Core** (`pkg/genie/`) - Business logic and service layer

### Contributing
- Follow Test-Driven Development (TDD) practices
- Use the TestFixture for writing new tests
- Run `go test ./...` to verify all tests pass
- Build with `go build -o build/genie ./cmd`