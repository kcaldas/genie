# Changelog

All notable changes to the Genie project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Response Handler System** - Structured LLM response processing for automated file operations
  - `FileGenerationHandler` - Processes FILE:/CONTENT:/END_FILE format responses with diff preview
  - `HandlerRegistry` - Manages and routes responses to appropriate handlers
  - User confirmation system with diff previews before file creation/modification
  - Integration with chain execution through `ResponseHandler` field in ChainStep

- **TestFixture for Genie package** - Centralized testing harness that eliminates boilerplate in test files
  - `NewTestFixture()` - One-line test environment creation with real dependencies and mocked LLM
  - Helper methods: `CreateSession()`, `StartChat()`, `WaitForResponse()`, `WaitForResponseOrFail()`
  - `ExpectMessage()`, `ExpectSimpleMessage()`, `UseChain()`, `GetMockLLM()` for elegant testing
  - Automatic cleanup and project directory management
  - MockChainRunner for chain-agnostic testing

- **Issues Documentation** - `docs/issues.norg` for tracking critical bugs and their solutions

### Fixed
- **Auto-accept confirmations hanging with --accept-all flag**
  - Fixed topic mismatch: file handler now publishes to correct `user.confirmation.request` topic
  - CLI auto-accept now properly handles file generation confirmations
  - Multi-file changes work smoothly with automated confirmation

- **Function call recursion hanging with "final response still contains function calls" error**
  - When hitting function call limit (8), system now makes clean final call without tools
  - LLM forced to provide text conclusion using full accumulated context
  - Eliminates hanging and provides meaningful responses instead of errors

- **Restrictive token limits preventing file generation**
  - Removed artificial 1000-token limits from execution, planning, and verification prompts
  - System now uses default configuration limits allowing proper file generation
  - Resolves `FinishReasonMaxTokens` errors for complex implementation tasks

- **LLM response format issues in execution phase**
  - Updated `execute_changes.yaml` prompt to clarify LLM should output FILE: format, not use tools
  - LLM now properly generates structured responses for file creation
  - Added debug capability to capture raw LLM responses for troubleshooting

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

### Known Issues
- **LLM making destructive wholesale changes instead of minimal incremental modifications**
  - System performs massive rewrites instead of targeted changes (e.g., deleting 95% of file content)
  - Documented in `docs/issues.norg` with reproduction steps and required fixes
  - Planning and execution prompts need strengthening to enforce truly minimal changes

### Technical
- Chain execution now supports response handlers for structured LLM output processing
- EventBus architecture supports user confirmation workflows
- All tool implementations now include emoji-free output formatting
- TUI architecture properly uses Genie service layer instead of direct LLM calls
- Wire dependency injection setup includes OutputFormatter and HandlerRegistry
- Test coverage maintained across all refactored components
- Chain.Run() now accepts EventBus as parameter for cleaner architecture

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