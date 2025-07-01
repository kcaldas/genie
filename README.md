# Genie: An AI Coding Assistant

Genie is a powerful, Go-based AI coding assistant designed to streamline software development directly from your terminal. Leveraging Gemini as its Large Language Model (LLM) backend, Genie offers both direct Command Line Interface (CLI) commands and an interactive Text User Interface (TUI) for a seamless development experience.

## ✨ Features

*   **AI-Powered Assistance:** Harness the power of Gemini LLM for a wide range of coding tasks and queries.
*   **Dual Interface:** Choose between quick CLI commands (`genie ask "..."`) or an immersive, conversational TUI (REPL) mode.
*   **Contextual Understanding:** Provides relevant AI responses by utilizing project context, file contents, and chat history.
*   **Extensible Tooling:** Integrates seamlessly with development tools for file operations, Git, and intelligent search.
*   **Event-Driven Architecture:** Features a decoupled, scalable design for enhanced maintainability and future expansion.
*   **Configurable TUI:** Personalize your interactive REPL experience with customizable settings.

## 🏗️ Architecture Overview

Genie follows a clear, layered architecture, ensuring modularity and maintainability:

1.  **Ultra-thin Main (`cmd/main.go`):** The application's entry point, routing execution to either the CLI or TUI based on arguments.
2.  **CLI Client (`cmd/cli/`):** Handles direct, single-command interactions.
3.  **TUI Client (`cmd/tui/`):** Provides an interactive Read-Eval-Print Loop (REPL) for persistent, conversational sessions.
4.  **Genie Core (`pkg/genie/`):** Contains the core business logic, service layer, event bus, and session management, consumed independently by both CLI and TUI clients.

This separation ensures each client manages its specific concerns while relying on a consistent and robust core.

## 🚀 Getting Started

### Prerequisites

*   Go (version 1.23.6 or higher recommended)

### Installation & Build

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/kcaldas/genie.git # Replace with actual repo URL
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

## 💡 Usage

### CLI Mode

For direct, one-off commands:

```bash
./build/genie ask "explain this Go function"
```

### TUI (Interactive REPL) Mode

For a persistent, conversational interface:

```bash
./build/genie
```

#### TUI Commands

Once in TUI mode, use these commands:

*   `/help`: Displays available commands.
*   `/config`: Manages TUI configuration settings.
*   `/clear`: Clears the current conversation history.
*   `/debug`: Toggles debug mode for logging.
*   `/exit`: Exits the REPL session.

## 📦 Key Packages

*   `cmd/`: CLI and TUI clients, and the main entry point.
*   `pkg/genie/`: Core Genie service layer with event-driven architecture.
*   `pkg/ai/`: Handles AI chain execution and LLM interactions.
*   `pkg/tools/`: Development tools (file ops, Git, search).
*   `pkg/events/`: Event bus for asynchronous communication.
*   `internal/di/`: Dependency injection using Wire.

## ⚙️ Development & Contributing

Genie development strongly favors a Test-Driven Development (TDD) workflow. We welcome contributions! Please follow these steps:

1.  **Fork the repository.**
2.  **Create a new branch** for your feature or bug fix.
3.  **Implement changes** following TDD principles (write failing test -> implement -> refactor).
4.  **Write and run tests** to ensure correctness.
5.  **Submit a pull request** with a clear description.

### Code Conventions

*   **Dependency Injection:** Wire is used; providers defined in `internal/di/wire.go`. Factory functions should return interfaces.
*   **File Naming:** Descriptive names, e.g., `session_manager.go` for `SessionManager` type. Test files use `_test.go` suffix.
*   **Context Variables:** Use `ctx` to avoid conflicts with the standard `context` package.

### Event-Driven Architecture

Genie uses an event bus (`pkg/events/`) for asynchronous communication. The Genie core publishes events (e.g., `chat.response`), and clients subscribe directly. This design supports scalability for local and future distributed deployments.

### Configuration

*   **TUI Settings:** Stored in `~/.genie/settings.tui.json`, managed via `/config` command.
*   **Chat History:** Persisted in `.genie/history`.