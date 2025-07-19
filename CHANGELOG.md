# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
  - Sequential thinking: Watch AI reason through complex problems step-by-step
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