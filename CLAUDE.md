# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Genie is a Go-based AI coding assistant tool similar to Claude Code, using Gemini as the LLM backend. The project provides both direct CLI commands and an interactive TUI for software engineering tasks.

## Architecture Overview

Genie follows a clean, layered architecture with four main components:

1. **Ultra-thin Main** (`cmd/genie/main.go`): Pure mode detection - routes to CLI or TUI based on command arguments
2. **CLI Client** (`cmd/cli/`): Handles direct commands like `genie ask "hello"`
3. **TUI Client** (`cmd/tui/`): Provides interactive REPL experience when running `genie` with no arguments
4. **Genie Core** (`pkg/genie/`): Business logic, service layer, event bus, session management

The CLI and TUI are independent clients of the same Genie core. This separation allows each client to manage its own concerns while consuming unified services from the core.

## Development Workflow

- Prefer Test-Driven Development (TDD) style workflow when possible
- Recommended TDD approach: Run tests > Change tests > See failure > Implement code
- Start renaming or refactoring by first modifying the tests to reflect the desired changes
- Use `ctx` for context variables to avoid conflicting with the `context` package

## Current Build Commands

```bash
# Build the project
go build -o build/genie ./cmd/genie

# Run tests
go test ./...

# Install dependencies
go mod tidy

# Run the CLI tool (ask command example)
./build/genie ask "hello"

# Run interactive TUI
./build/genie
```

## Key Packages

- `cmd/genie/` - Main entry point with CLI and TUI clients
- `pkg/genie/` - Core Genie service layer with event-driven architecture
- `pkg/ai/` - AI prompt execution and LLM abstraction
- `pkg/tools/` - Development tools (file ops, git, search, etc.)
- `pkg/events/` - Event bus for async communication
- `internal/di/` - Wire dependency injection

## Current CLI Commands

- `ask` - Send a question to the AI (e.g., `genie ask "explain this code"`)

## Current TUI Commands

When in interactive REPL mode (`genie` with no args):
- `/help` - Show available commands
- `/config` - TUI configuration management (cursor settings, etc.)
- `/clear` - Clear conversation history
- `/debug` - Toggle debug mode
- `/exit` - Exit REPL

## Code Conventions

### Dependency Injection with Wire
- Use Wire for dependency injection with providers in `internal/di/wire.go`
- Factory functions return interfaces: `func NewSessionManager() Manager`
- For channel-based broadcasting: each provider creates its own channel instance
- Don't test Wire injection itself - test actual functionality

### TDD Workflow Preference
- Write failing test → Make it pass → Refactor → Repeat
- For API changes: Update tests first, then implementation
- For internal refactoring: Keep tests unchanged to validate behavior

### File Naming
- Use descriptive names that match the primary type: `session_manager.go` for `SessionManager`
- Use `_test.go` suffix for test files

## Event-Driven Architecture

Genie uses an event bus for async communication:
- Genie core publishes events (e.g., `chat.response`)
- Clients subscribe to events directly via the event bus
- This design supports both local and future remote deployments

## Configuration

- TUI settings: `~/.genie/settings.tui.json` (managed via `/config` in REPL)
- Chat history: `.genie/history`