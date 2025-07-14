# Genie: Your AI Coding Assistant

Genie is a powerful, Go-based AI coding assistant designed to streamline your software development workflow directly from your terminal. Powered by Google's Gemini LLM, Genie offers both a command-line interface (CLI) for quick, direct interactions and an interactive text-based user interface (TUI) for a more immersive, conversational experience.

## ✨ Features

*   **Dual Interface:** Choose between a fast, scriptable CLI and a feature-rich, interactive TUI.
*   **Powerful AI:** Leverages the Gemini LLM for a wide range of coding tasks, from generating code to answering questions.
*   **Extensible Tooling:** A robust tool system allows the AI to interact with your file system, run shell commands, and more.
*   **Event-Driven Architecture:** A decoupled, asynchronous architecture ensures a responsive user experience.
*   **Configurable and Extensible:** Customize the TUI, create custom personas, and extend the toolset to fit your needs.

## 🚀 Getting Started

### Prerequisites

*   Go (version 1.23.6 or higher)
*   A configured Gemini API key or Google Cloud project.

### Installation & Running

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/kcaldas/genie.git
    cd genie
    ```
2.  **Install dependencies:**
    ```bash
    make deps
    ```
3.  **Build the application:**
    ```bash
    make build
    ```
4.  **Run the TUI:**
    ```bash
    ./build/genie
    ```
5.  **Run the CLI:**
    ```bash
    ./build/genie ask "What is the current working directory?"
    ```

## 🏗️ Architecture

Genie follows a clean, layered architecture that separates concerns and promotes modularity:

1.  **Entry Point (`cmd/main.go`):** A thin entry point that determines whether to launch the CLI or the TUI based on the command-line arguments.
2.  **CLI Client (`cmd/cli`):** Handles direct, one-off commands. Built using the [Cobra](https://github.com/spf13/cobra) library.
3.  **TUI Client (`cmd/tui`):** Provides an interactive, terminal-based user interface. Built using the [gocui](https://github.com/awesome-gocui/gocui) library.
4.  **Genie Core (`pkg/genie`):** The core of the application, containing the business logic, service layer, event bus, and session management.
5.  **AI Engine (`pkg/ai`):** Manages the chain of thought, decision-making, and interaction with the LLM.
6.  **Tools (`pkg/tools`):** A collection of tools that the AI can use to interact with the system, such as file operations, git, and shell commands.
7.  **LLM Abstraction (`pkg/llm`):** An abstraction layer that provides a consistent interface for interacting with different LLM backends.

## ⚙️ Development

### Makefile Commands

The `Makefile` provides several commands to streamline development:

*   `make build`: Build the binary.
*   `make run`: Run the application in TUI mode.
*   `make test`: Run all tests.
*   `make test-race`: Run tests with the race detector.
*   `make lint`: Run the linter.
*   `make generate`: Generate code using Google Wire.
*   `make clean`: Clean build artifacts.

### Code Conventions

*   **Dependency Injection:** The project uses [Google Wire](https://github.com/google/wire) for compile-time dependency injection. See `internal/di/wire.go`.
*   **Testing:** The project uses the `testify` library for testing. Test files are named with a `_test.go` suffix.
*   **File Naming:** Go source files are named using `snake_case.go`.

## 📦 Key Packages

*   **`cmd`:** Entry point for the application, containing the CLI (`cmd/cli`) and TUI (`cmd/tui`) clients.
*   **`pkg/genie`:** The core business logic, service layer, and session management.
*   **`pkg/ai`:** The AI engine, which manages the chain of thought, decision-making, and interaction with the LLM.
*   **`pkg/tools`:** The extensible tool system that the AI uses to interact with the environment.
*   **`pkg/events`:** An event bus for asynchronous communication between different parts of the application.
*   **`pkg/llm`:** An abstraction layer for interacting with different LLM backends (e.g., Gemini, Vertex).
*   **`internal/di`:** The dependency injection setup, which uses Google Wire to wire the application together.

## ⚙️ Configuration

Genie can be configured using environment variables:

*   `GEMINI_API_KEY`: Your Gemini API key.
*   `GOOGLE_CLOUD_PROJECT`: Your Google Cloud project ID.
*   `GENAI_BACKEND`: The GenAI backend to use (`gemini` or `vertex`).

## 🎭 Personas

Genie supports different personas, which are pre-configured prompts that can be used to customize the AI's behavior. You can specify a persona using the `--persona` flag.