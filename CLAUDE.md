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

## Build Commands

```bash
# Build the project
make build
# Or directly:
go build -o build/genie ./cmd/genie

# Run tests
make test
# Or directly:
go test ./...

# Run tests with race detection
make test-race
# Or directly:
go test -race ./...

# Run tests with coverage
make test-coverage

# Run a specific test
go test ./pkg/tools -run TestMyFeature

# Install dependencies
make deps
# Or directly:
go mod tidy

# Generate Wire dependency injection code
make generate
# Or directly:
go generate ./...

# Run the CLI tool (ask command example)
./build/genie ask "hello"

# Run interactive TUI
./build/genie

# Run with a specific persona
./build/genie --persona engineer ask "review this code"
```

## Key Packages

- `cmd/genie/` - Main entry point with CLI and TUI clients
- `pkg/genie/` - Core Genie service layer with event-driven architecture and Wire dependency injection
- `pkg/ai/` - AI prompt execution and LLM abstraction
- `cmd/slashcommands/` - Slash command discovery and argument expansion
- `pkg/tools/` - Development tools (file ops, git, search, etc.)
- `pkg/skills/` - Skills system for modular, task-specific capabilities
- `pkg/events/` - Event bus for async communication
- `pkg/persona/` - Persona management and prompt factory
- `pkg/ctx/` - Context management (project, chat, file, todo, skill context)
- `pkg/mcp/` - Model Context Protocol client implementation
- `cmd/tui/controllers/commands/` - TUI command implementations

## Current CLI Commands

- `ask` - Send a question to the AI (e.g., `genie ask "explain this code"`)
- `--persona` - Use a specific persona (e.g., `genie --persona product_owner ask "plan this feature"`)

## Current TUI Commands

When in interactive REPL mode (`genie` with no args):
- `/help` - Show available commands
- `/config` - TUI configuration management (cursor settings, etc.)
- `/clear` - Clear conversation history
- `/debug` - Toggle debug mode
- `/exit` - Exit REPL
- `/write` - Multi-line input mode
- `/yank` - Copy last AI response to clipboard
- `/persona` - Manage personas (list, swap, cycle)
- `/theme` - Change color theme
- `/update` - Check for updates

## Code Conventions

### Dependency Injection with Wire
- Use Wire for dependency injection with providers in `pkg/genie/wire.go`
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

### Testing Patterns
- Unit tests: Component isolation with mocks
- Integration tests: End-to-end workflows
- Run specific test: `go test ./pkg/tools -run TestMyFeature`

## Event-Driven Architecture

Genie uses an event bus for async communication:
- Genie core publishes events (e.g., `chat.response`)
- Clients subscribe to events directly via the event bus
- This design supports both local and future remote deployments

## Configuration

- TUI settings: `~/.genie/settings.tui.json` (managed via `/config` in REPL)
- Chat history: `.genie/history`
- Personas: `.genie/personas/` (project-level) or `~/.genie/personas/` (user-level)
- Environment variables:
  - `GEMINI_API_KEY` - Required for Gemini API access
  - `GENIE_PERSONA` - Default persona to use
  - `GENIE_CAPTURE_LLM` - Enable LLM interaction capture for testing

## Persona System

Genie supports specialized AI personas with different expertise and tools:
- Built-in personas: `genie`, `product_owner`, `persona_creator`, `minimal`
- Custom personas in `.genie/personas/{name}/prompt.yaml`
- TUI commands: `:persona list`, `:persona swap <name>`, `:persona cycle add <name>`
- Keyboard shortcuts: Ctrl+P or Shift+Tab to cycle through personas

## Skills System

Genie implements Claude Skills-compatible functionality for modular, task-specific capabilities:

### Overview
Skills are specialized capability packages that extend Genie's functionality with domain-specific expertise. They use progressive disclosure to load only relevant context when needed, similar to Claude Code's skills system.

### Skill Architecture
- **AI-Driven Invocation**: Skills are autonomously activated by the AI based on task relevance
- **Progressive Loading**: Skill metadata loaded at startup, full content loaded on-demand
- **Ephemeral Context**: Skill content auto-removed after task completion to conserve tokens
- **SKILL.md Format**: Compatible with Claude Code's YAML frontmatter + markdown format

### Skill Locations (Priority Order)
1. **Project skills**: `.genie/skills/{skill-name}/SKILL.md`
2. **Claude compatibility**: `.claude/skills/{skill-name}/SKILL.md`
3. **User skills**: `~/.genie/skills/{skill-name}/SKILL.md`
4. **Built-in skills**: `pkg/skills/internal/skills/{skill-name}/SKILL.md`

### SKILL.md Format
```markdown
---
name: skill-name
description: What the skill does and when to use it
---

# Skill Content

Detailed instructions, procedures, and best practices...
```

### Built-in Skills
- **codebase-search**: Navigate and understand codebases, find implementations, answer "where is..." questions
- **test-helper**: Write comprehensive tests following best practices and project conventions

### How Skills Work
1. **Discovery**: Skill metadata (name, description) loaded at startup
2. **Invocation**: AI uses `Skill` tool to load a skill when relevant
3. **Context Loading**: Full SKILL.md content added to conversation context
4. **Task Execution**: AI follows skill guidance to complete the task
5. **Cleanup**: Skill content removed from context when task completes

### Creating Custom Skills
Place custom skills in `.genie/skills/{skill-name}/SKILL.md`:
```yaml
---
name: my-custom-skill
description: Brief description of when to use this skill
---

# Skill documentation goes here
```

### Implementation Details
- **Package**: `pkg/skills/`
- **SkillManager**: Discovery, loading, and lifecycle management
- **SkillTool**: AI invocation interface via function calling
- **SkillContextPartProvider**: Integrates with context management system
- **Wire Integration**: Full dependency injection support

## Tool System

Available tools are defined in `pkg/tools/`:
- File operations: `readFile`, `writeFile`, `listFiles`, `findFiles`
- Search: `searchInFiles`, `bash`
- Git operations: `git` command wrapper
- Todo management: `todo`, `todoWrite`
- Thinking: Advanced reasoning tool
- **Skill**: Load and invoke specialized skills for domain-specific tasks
- MCP tools: Dynamically loaded from Model Context Protocol servers

Note: The `bash` tool now includes an optional `_display_message` parameter for a clear, concise description of the command's purpose.

## Error Handling

- Structured errors with code, message, and cause
- Categories: User errors, System errors, Tool errors
- Always provide helpful error messages with recovery suggestions

## Performance Considerations

- Conversation history limits to manage memory
- Tool result caching for efficiency
- Non-blocking UI updates in TUI
- Concurrent AI request processing