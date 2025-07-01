# Genie: An AI Coding Assistant

Genie is a Go-based AI coding assistant tool, similar to Claude Code, utilizing Gemini as its LLM backend. It offers both direct Command Line Interface (CLI) commands and an interactive Text User Interface (TUI) for various software engineering tasks.

## Project Overview

Genie is designed to streamline software development by providing an AI-powered assistant directly within your terminal. Its core functionality revolves around understanding and assisting with coding tasks, leveraging a clean, layered architecture for maintainability and extensibility.

## Features

*   **AI-Powered Assistance:** Leverage Gemini LLM for various coding tasks and queries.
*   **Dual Interface:** Interact via direct CLI commands or an interactive TUI (REPL) mode.
*   **Contextual Understanding:** Utilizes project context, file contents, and chat history to provide relevant AI responses.
*   **Extensible Tooling:** Integrates with various development tools for file operations, Git, and search.
*   **Event-Driven Architecture:** Decoupled components for enhanced scalability and maintainability.
*   **Configurable TUI:** Customize interactive REPL settings for a personalized experience.

## Architecture Overview

Genie follows a clear, layered architecture comprising four main components:

1.  **Ultra-thin Main** (`cmd/main.go`): This serves as the entry point, primarily handling mode detection to route execution to either the CLI or TUI based on command-line arguments.
2.  **CLI Client** (`cmd/cli/`): Manages direct, single-command interactions, such as `genie ask "hello"`.
3.  **TUI Client** (`cmd/tui/`): Provides an an interactive Read-Eval-Print Loop (REPL) experience when `genie` is run without arguments, offering a more persistent and conversational interface.
4.  **Genie Core** (`pkg/genie/`): Contains the core business logic, service layer, event bus, and session management. Both the CLI and TUI clients are independent consumers of these unified services.

This separation ensures that each client can manage its specific concerns while relying on a consistent and robust core.

## Getting Started

### Prerequisites

*   Go (version 1.23.6 or higher recommended)

### Installation & Build

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/kcaldas/genie.git # Replace with actual repo URL if different
    cd genie
    ```
2.  **Install Go module dependencies:**
    ```bash
    go mod tidy
    ```
3.  **Build the project executable:**
    ```bash
    go build -o build/genie ./cmd
    ```

### Running Genie

#### CLI Mode

To use Genie in CLI mode for direct commands:

```bash
./build/genie ask "explain this code"
```

#### TUI (Interactive REPL) Mode

To launch the interactive TUI:

```bash
./build/genie
```

## Key Packages

The project is organized into several key packages:

*   `cmd/`: Contains the CLI and TUI clients, along with the ultra-thin main entry point.
*   `pkg/genie/`: Houses the core Genie service layer, built with an event-driven architecture.
*   `pkg/ai/`: Handles AI chain execution and provides an abstraction layer for LLM interactions.
*   `pkg/tools/`: Contains various development tools, including file operations, Git integration, and search functionalities.
*   `pkg/events/`: Implements the event bus for asynchronous communication within the application.
*   `internal/di/`: Manages dependency injection using the Wire framework.

## Commands

### CLI Commands

Currently, the primary CLI command is:

*   `ask`: Send a question or prompt to the AI (e.g., `genie ask "summarize this article"`).

### TUI Commands (Interactive REPL)

When in the interactive REPL mode (`./build/genie`), the following commands are available:

*   `/help`: Displays available commands and usage information.
*   `/config`: Manages TUI configuration settings (e.g., cursor settings).
*   `/clear`: Clears the current conversation history in the REPL.
*   `/debug`: Toggles debug mode for enhanced logging.
*   `/exit`: Exits the interactive REPL session.

## Development Workflow

Genie development strongly prefers a Test-Driven Development (TDD) style workflow:

*   **TDD Approach:** Write a failing test → Implement code to make it pass → Refactor → Repeat.
*   **API Changes:** When modifying APIs, update the relevant tests first to reflect the desired changes, then implement the changes.
*   **Internal Refactoring:** For internal code refactoring, aim to keep existing tests unchanged to validate that the external behavior remains consistent.
*   **Context Variables:** Use `ctx` for context variables to avoid naming conflicts with the standard `context` package.

## Code Conventions

### Dependency Injection with Wire

*   **Framework:** Wire is used for dependency injection, with providers defined in `internal/di/wire.go`.
*   **Factory Functions:** Factory functions should return interfaces (e.g., `func NewSessionManager() Manager`).
*   **Channel-based Broadcasting:** Each provider creates its own channel instance for broadcasting.
*   **Testing:** Focus on testing the actual functionality rather than the Wire injection itself.

### File Naming

*   **Descriptive Names:** Use descriptive file names that align with the primary type they define (e.g., `session_manager.go` for the `SessionManager` type).
*   **Test Files:** Test files should use the `_test.go` suffix (e.g., `session_manager_test.go`).

## Event-Driven Architecture

Genie utilizes an event bus for asynchronous communication between components:

*   **Publishing Events:** The Genie core publishes events (e.g., `chat.response`).
*   **Subscribing to Events:** Clients subscribe to events directly via the event bus.
*   **Scalability:** This design supports both local deployments and facilitates future remote or distributed deployments.

## Configuration

*   **TUI Settings:** User-specific TUI settings are stored in `~/.genie/settings.tui.json` and can be managed via the `/config` command in the REPL.
*   **Chat History:** Conversation history is persisted in `.genie/history`.

## Contributing

We welcome contributions to Genie! Please follow these steps to contribute:

1.  **Fork the repository.**
2.  **Create a new branch** for your feature or bug fix.
3.  **Implement your changes** following the [Development Workflow](#development-workflow) and [Code Conventions](#code-conventions).
4.  **Write and run tests** to ensure your changes are working correctly and haven't introduced regressions.
5.  **Submit a pull request** with a clear description of your changes.