# Changelog

All notable changes to the Genie project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **TestFixture for Genie package** - Centralized testing harness that eliminates boilerplate in test files
  - `NewTestFixture()` - One-line test environment creation with real dependencies and mocked LLM
  - Helper methods: `CreateSession()`, `StartChat()`, `WaitForResponse()`, `WaitForResponseOrFail()`
  - Automatic cleanup and project directory management
  - Example test demonstrating usage patterns

### Changed
- **Refactored test architecture** across TUI and integration tests
  - Reduced TUI test imports from 15 to 7 packages (53% reduction)
  - Eliminated ~50-80 lines of boilerplate setup per test file
  - Standardized test setup patterns for consistency
  - Moved LLM-specific tests to appropriate packages

### Improved
- **Tool output formatting** - Enhanced display of tool execution results in TUI
  - Added `FormatOutput` method to Tool interface for custom formatting
  - Created `OutputFormatter` service to parse and format Gemini tool outputs
  - Removed emojis from tool formatters for better terminal compatibility
  - Integrated formatting through dependency injection system

### Technical
- All tool implementations now include emoji-free output formatting
- TUI architecture properly uses Genie service layer instead of direct LLM calls
- Wire dependency injection setup includes OutputFormatter
- Test coverage maintained across all refactored components

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