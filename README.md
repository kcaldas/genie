# Genie: Your AI Coding Assistant

Genie is a powerful, Go-based AI coding assistant designed to streamline your software development workflow directly from your terminal. Powered by Google's Gemini LLM, Genie offers both a command-line interface (CLI) for quick, direct interactions and an interactive text-based user interface (TUI) for a more immersive, conversational experience.

## ✨ Features

*   **Dual Interface:** Choose between a fast, scriptable CLI and a feature-rich, interactive TUI.
*   **Powerful AI:** Leverages the Gemini LLM for a wide range of coding tasks, from generating code to answering questions.
*   **Extensible Tooling:** A robust tool system allows the AI to interact with your file system, run shell commands, and more.
*   **Event-Driven Architecture:** A decoupled, asynchronous architecture ensures a responsive user experience.
*   **Configurable and Extensible:** Customize the TUI, create custom personas, and extend the toolset to fit your needs.

## 🏗️ Architecture

Genie follows a clean, layered architecture that separates concerns and promotes modularity:

1.  **Entry Point (`cmd/main.go`):** A thin entry point that determines whether to launch the CLI or the TUI based on the command-line arguments.
2.  **CLI Client (`cmd/cli`):** A client that handles direct, one-off commands. It's built using the [Cobra](https://github.com/spf13/cobra) library.
3.  **TUI Client (`cmd/tui`):** A client that provides an interactive, terminal-based user interface. It's built using the [gocui](https://github.com/awesome-gocui/gocui) library.
4.  **Genie Core (`pkg/genie`):** The core of the application, containing the business logic, service layer, event bus, and session management.
5.  **AI Engine (`pkg/ai`):** The AI engine that manages the chain of thought, decision-making, and interaction with the LLM.
6.  **Tools (`pkg/tools`):** A collection of tools that the AI can use to interact with the system, such as `ls`, `cat`, `grep`, and `bash`.
7.  **LLM Abstraction (`pkg/llm`):** An abstraction layer that provides a consistent interface for interacting with different LLM backends.

## 🚀 Getting Started

### Prerequisites

*   Go (version 1.23.6 or higher)
*   A configured Gemini API key or Google Cloud project.

### Installation

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

## 💡 Usage

### CLI Mode

For quick, one-off commands, use the `ask` subcommand:

```bash
./build/genie ask "What is the current working directory?"
```

You can also use the `--accept-all` flag to automatically accept all tool confirmations:

```bash
./build/genie ask --accept-all "Create a new file named 'hello.txt' with the content 'Hello, World!'"
```

### TUI Mode

For a more interactive experience, run `genie` without any subcommands:

```bash
./build/genie
```

The TUI provides a rich set of features, including:

*   A command history (navigate with the up and down arrow keys).
*   A debug panel (toggle with `Ctrl+D`).
*   A help system (run the `:help` command).
*   Markdown rendering for AI responses.

## ⚙️ Development

### Building and Running

*   **Build:** `make build`
*   **Run:** `make run`
*   **Test:** `make test`
*   **Lint:** `make lint`
*   **Generate Code:** `make generate` (runs Google Wire)

### Code Conventions

*   **Dependency Injection:** The project uses [Google Wire](https://github.com/google/wire) for compile-time dependency injection.
*   **Testing:** The project uses the `testify` library for testing.
*   **File Naming:** Files are named using `snake_case`. Test files are named with a `_test.go` suffix.

## 📦 Key Packages

*   **`cmd`:** The entry point for the application, containing the CLI and TUI clients.
*   **`pkg/genie`:** The core of the application, containing the business logic, service layer, and session management.
*   **`pkg/ai`:** The AI engine, which manages the chain of thought, decision-making, and interaction with the LLM.
*   **`pkg/tools`:** A collection of tools that the AI can use to interact with the system.
*   **`pkg/events`:** An event bus for asynchronous communication between different parts of the application.
*   **`pkg/llm`:** An abstraction layer that provides a consistent interface for interacting with different LLM backends.
*   **`internal/di`:** The dependency injection setup, which uses Google Wire to wire the application together.

## ⚙️ Configuration

Genie can be configured using environment variables. The following environment variables are supported:

*   **`GEMINI_API_KEY`:** Your Gemini API key.
*   **`GOOGLE_CLOUD_PROJECT`:** Your Google Cloud project ID.
*   **`GENAI_BACKEND`:** The GenAI backend to use (`gemini` or `vertex`).

## 🎭 Personas

Genie supports different personas, which are pre-configured prompts that can be used to customize the AI's behavior. You can specify a persona using the `--persona` flag.

## 🚌 Event-Driven Architecture

Genie uses an event-driven architecture to ensure a responsive user experience. The application uses an in-memory event bus to publish and subscribe to events.

## 💉 Dependency Injection

Genie uses [Google Wire](https://github.com/google/wire) for compile-time dependency injection. This allows for a clean separation of concerns and makes the code more modular and easier to test.
