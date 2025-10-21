# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.2-beta] - 2025-10-22

### Added
- **Ollama Support**: Integration with Ollama for local LLM inference
  - Support for running models locally without API keys
  - Seamless provider switching between cloud and local models
  - Compatible with existing persona and tool systems

## [0.2.1-beta] - 2025-10-21

### Added
- **Multi-Provider Support**: Support for multiple LLM providers with automatic routing
  - Official Anthropic (Claude) backend integration with SDK
  - Official OpenAI backend integration
  - Provider multiplexer with intelligent routing based on persona configuration
  - Shared Genie client header for all LLM providers
- **Image Chat Support**: Send images in conversations for visual analysis
  - Enhanced GenAI client to handle diverse content parts (text + images)
  - Support for image attachments in chat interface
- **Seed Chat History**: Start Genie with pre-loaded conversation context
  - Useful for resuming work or providing initial context

### Enhanced
- **Provider Status Display**: UI now shows last-used provider and model information
- **Provider Switching**: Automatic fallback to default persona when requested persona is missing
- **Sampling Parameters**: Provider-specific parameter handling to avoid conflicts
  - Proper sampling parameter mapping for Anthropic API
- **Tool Deduplication**: Prevent duplicate tool definitions when using Anthropic backend

### Fixed
- **Tool Naming**: De-duplicate tools by name for Anthropic compatibility
- **Test Suite**: Fixed tests to work with new provider architecture

## [0.2.0-beta] - 2025-10-XX

## [0.1.7] - 2025-07-27

### Added
- **Homebrew Support**: Proper Homebrew tap with formula for easy installation and updates
  - `brew tap kcaldas/genie && brew install genie`
  - Automatic updates via `brew upgrade`
- **Release Command**: New slash command `/release` with complete release workflow guidance

### Fixed
- **TUI Default Output**: Default output mode is now "true" (24-bit color) for better display
  - Users upgrading from older versions may need to run `:config output true` in TUI
  - This fixes display issues with themes and colors
- **Update Permissions**: Improved error messaging for macOS installer update permissions

### Enhanced
- **Release Process**: Streamlined release checklist with clear step-by-step workflow
- **GoReleaser Config**: Enabled Homebrew formula upload for automatic tap updates

## [0.1.6] - 2025-07-26

### Added
- **Self-Update Capability**: Built-in update mechanism with security validation
  - `genie update` CLI command with `--check`, `--force`, and `--version` flags
  - `/update` TUI command with check/now/version/force subcommands
  - Secure updates using checksums.txt validation from GitHub releases
  - Version information display with `genie --version`
- **Slash Command System**: Complete implementation of TUI slash commands
  - Full slash command handling and execution
  - Auto-completion and suggestion system for slash commands
  - Dedicated help system with `/help` command
  - Enhanced input shell with basic completion
- **LLM Retry Mechanism**: Exponential backoff for improved reliability
- **Mac Installer Packages**: Generate .pkg installers for macOS distribution
- **Version Management**: Proper build-time version injection via ldflags

### Enhanced
- **TUI Input Handling**: Major improvements to write component and input processing
- **Panel System**: Re-render components after zoom to ensure proper width updates
- **Logging System**: 
  - Centralized file-based logging for TUI
  - Enhanced markdown rendering and help display
- **Tool System**:
  - Enhanced todo context and updated tool usage policies
  - Improved bash tool with explicit confirmation
  - Better documentation for safer command execution
- **Configuration System**: Refactored boolean configs to string-based system for better JSON handling

### Fixed
- **TUI Layout**: Corrected panel dimension calculations for full terminal width usage
- **String Boolean Configuration**: Improved handling of boolean config fields
- **Development Version Handling**: Self-update gracefully handles "dev" versions

## [0.1.5] - 2024-11-29

### Added
- **Task Tool**: Implemented a new `Task` tool for isolated research sessions.
- **Persona Configuration**: Added `GENIE_PERSONA` environment variable support and new persona files.
- **Nvim Plugin Reference**: Added reference to nvim companion plugin.
- **Gemini Show Thoughts Config**: Added `GEMINI_SHOW_THOUGHTS` configuration option for LLM output.

### Fixed
- **Empty Gemini Responses**: Prevented empty responses from Gemini by returning the last thought if no regular text is available.

### Enhanced
- **Persona and Tool Updates**: Updated assistant persona to use the `Task` tool and refined persona prompts for concise output.
- **Documentation**: Updated `TodoRead` and `TodoWrite` tool descriptions.
- **Codebase Cleanup**: Removed `GetLLMContext` and related tests, and removed `TodoRead` tool completely.
- **Internal Notes**: Moved internal notes to `ops` directory.

## [0.1.4] - 2025-07-19

### Added
- **Unix Pipe Support**: Full pipeline integration for both CLI and TUI modes
  - `echo "question" | genie ask` - CLI with piped input
  - `echo "question" | genie` - TUI with initial piped message
  - `git diff | genie ask "commit message?"` - Shell workflow integration
- **Global and Local Configuration**: Hierarchical config system
  - `:config --global theme dark` - Global settings
  - `:config theme light` - Local project overrides
  - Smart merging: defaults → global → local
- **Per-Tool Configuration**: Fine-grained control over tool behavior
  - Auto-accept tools without confirmation
  - Hide tool execution from chat display
- **Environment Variable Support**: `GENIE_PERSONA` for default persona
- **Enhanced TUI Features**:
  - Right panel zoom functionality (Ctrl+Z)
  - Separate diff theme system with highlighted backgrounds
  - Subtitle support for UI components
  - Separate theme border/title colors
  - Custom editor with special key handling (voice input support)

### Fixed
- **Critical Gemini API Bug**: Fixed validation error when using minimal persona
  - Resolved "tools[0].tool_type: required one_of 'tool_type' must have one initialized field"
  - Empty tools array now handled correctly
- **Windows Compatibility**: Fixed persona loading and embedded persona locations
- **User Confirmation**: Fixed writeFile auto-accept logic
- **Context Cancellation**: Properly cancel context when user negates confirmations

### Enhanced
- **Shared Stdin Utilities**: Improved code organization for pipe handling
- **TUI Message Initialization**: Seamless initial message support
- **Config Deep Merging**: Generic reflection-based configuration merging
- **Theme System**: Removed unused mode configuration, improved theming
- **MCP Integration**: Removed git MCP dependency for cleaner architecture

## [0.1.0-beta] - 2025-07-17

### Added
- **Interactive TUI Mode**: Rich terminal interface with vim-style navigation (F4/Ctrl+V for multi-line)
- **Direct CLI Mode**: Quick AI interactions with `genie ask "your question"`
- **Personas System**: Customize AI personality and behavior for different workflows
- **Powerful Tools**:
  - File operations: read, write, search, and manage your codebase
  - Git integration: status, diffs, and repository operations
  - Bash execution: run commands safely with confirmation
  - Thinking: Watch AI reason through complex problems step-by-step
  - Todo management: Track and organize tasks
  - Smart search: grep and find with AI assistance
- **Cross-Platform**: Native binaries for macOS, Linux, and Windows
- **Docker Support**: Secure containerized environment for safe AI interactions
- **Smart Configuration**: 
  - Environment variables and .env file support
  - Per-mode settings (TUI/CLI)
  - Flexible API key management
- **User Experience**:
  - Real-time streaming responses
  - Tool confirmation dialogs for safety
  - Session history and context management
  - Debug mode for transparency
  - Comprehensive help system

### Philosophy
Built for developers who value:
- **Control**: You approve every action before execution
- **Transparency**: See exactly what the AI is thinking and doing
- **Unix Principles**: Composable, focused, and reliable
- **Local First**: Your conversations and data stay on your machine

[0.1.0-beta]: https://github.com/kcaldas/genie/releases/tag/v0.1.0-beta
