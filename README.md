# 🧞 Genie - Powerful AI for Your Command Line

[![Go Version](https://img.shields.io/badge/go-1.23+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker](https://img.shields.io/badge/docker-ready-brightgreen.svg)](https://github.com/kcaldas/genie/pkgs/container/genie)
[![Beta](https://img.shields.io/badge/status-beta-orange.svg)](https://github.com/kcaldas/genie/releases)

Born from a developer's need for more control and understanding of AI assistance, Genie brings the power of AI directly to where you work - the command line. Inspired by tools like Aider and Claude Code, but designed for the Unix philosophy: do one thing well, compose freely, and integrate seamlessly into any workflow.

Whether you're coding, managing projects, taking notes, or automating pipelines, Genie adapts to your needs while giving you full transparency and control over what the AI is doing.

## 🎯 Philosophy

Genie was built on the belief that AI tools should:
- **Give you control**: You understand what's happening, not just trust a black box
- **Integrate naturally**: Work with your existing tools and workflows, not replace them  
- **Respect the terminal**: Embrace the power and flexibility of the command line
- **Stay composable**: Following Unix principles - pipe, redirect, script, and automate
- **Adapt to you**: Handle coding, project management, note-taking, or whatever you need

## ✨ What Genie Does

- **🤖 Universal AI Assistant**: Powered by Google's Gemini for any task you can imagine
- **🖥️ Dual Interface**: CLI for quick queries, TUI for interactive sessions
- **💻 Development Work**: Code generation, debugging, refactoring, architecture design
- **📋 Project Management**: Planning, task breakdown, progress tracking
- **📝 Writing & Research**: Draft documents, analyze text, research topics online
- **📁 File Operations**: Read, write, and organize files intelligently
- **🔧 Workflow Automation**: Integrate into scripts, pipelines, and CI/CD systems
- **🧠 Sequential Thinking**: Advanced reasoning for complex problem-solving
- **📝 Vim Mode**: Optional vim keybindings for power users
- **🐳 Docker Support**: Run safely in isolated containers
- **🔌 Extensible**: Plugin architecture via MCP (Model Context Protocol)

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
    # Development
    ./build/genie ask "refactor this function to use async/await"
    
    # Research
    ./build/genie ask "summarize the latest trends in AI"
    
    # System tasks
    ./build/genie ask "analyze my log files for errors"
    
    # Writing
    ./build/genie ask "help me write a technical blog post"
    ```

## 💼 Real-World Usage

### Pipeline Integration
```bash
# In your CI/CD pipeline
genie ask "review this pull request for potential issues" < changes.diff

# Automated documentation
genie ask "generate API docs from this OpenAPI spec" < api.yaml
```

### Project Management
```bash
# Break down complex tasks
genie ask "break down 'implement user authentication' into subtasks"

# Status updates
genie ask "summarize git commits from last week into a status report"
```

### Daily Automation
```bash
# Script automation
#!/bin/bash
ANALYSIS=$(genie ask "analyze this log file for errors" < app.log)
echo "$ANALYSIS" | mail -s "Daily Error Report" team@company.com

# Note processing
genie ask "organize these meeting notes by action items" < meeting.md > actions.md
```

## 🎨 Interactive TUI Experience

While the CLI is perfect for automation and quick queries, the TUI provides a rich, interactive experience that makes working with AI feel natural and powerful.

### ✨ TUI Features

```
┌─ Genie: Your AI Assistant ─────────────────────────────────────────────┐
│                                                                         │
│ 🤖 AI Response Area                                                     │
│ ┌─────────────────────────────────────────────────────────────────────┐ │
│ │ **Thinking (1/3)**                                                  │ │
│ │ Let me break down this architecture design step by step...          │ │
│ │                                                                     │ │
│ │ First, I'll consider the data flow requirements...                  │ │
│ │ ────────────────────────────────────────────────────────────────── │ │
│ │ For a microservices architecture, I recommend:                     │ │
│ │ • API Gateway for routing and authentication                       │ │
│ │ • Service mesh for inter-service communication                     │ │
│ │ • Event sourcing for data consistency                              │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│ 💬 Input Area                                                          │
│ ┌─────────────────────────────────────────────────────────────────────┐ │
│ │ > Help me design a scalable microservices architecture             │ │
│ │                                                                     │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│ Commands: :help :clear :config :exit    |    Vim Mode: OFF             │
└─────────────────────────────────────────────────────────────────────────┘
```

### 🎯 Why the TUI Stands Out

- **📜 Conversation History**: Keep full context of your session
- **🧠 Sequential Thinking**: Watch AI reasoning unfold step-by-step  
- **⚡ Real-time Responses**: See responses stream in as they're generated
- **🎨 Syntax Highlighting**: Code blocks are beautifully formatted with customizable themes
- **⌨️ Vim Mode**: Optional vim keybindings for power users (`:config vim on`)
- **📱 Responsive Layout**: Adapts to your terminal size
- **🎛️ Live Configuration**: Change settings without restarting

### 🔧 TUI Commands

| Command | Description | Examples |
|---------|-------------|----------|
| `:help` | Show all available commands | `:help`, `:?` |
| `:clear` | Clear conversation history | `:clear`, `:cls` |
| `:config` | Manage TUI settings | `:config vim on`, `:config theme dark` |
| `:debug` | Toggle debug information | `:debug on` |
| `:exit` | Exit the session | `:exit`, `:quit` |

### ⌨️ Vim Mode for Multi-line Editing

Genie includes a powerful vim editor for complex, multi-line input:

```bash
genie                    # Launch interactive mode
:config vim on           # Enable vim keybindings globally

# Activate vim editor for current input:
F4                       # Enter vim editor mode
Ctrl+V                   # Alternative key to enter vim editor
```

**When in Vim Editor Mode:**
- **Normal Mode**: `h/j/k/l` (navigate), `w/b` (words), `0/$` (line), `gg/G` (file), `dd` (delete line), `A` (append)
- **Insert Mode**: `i/a/o/O` (insert), `ESC` (back to normal)  
- **Command Mode**: `:w` (send message), `:q` (cancel input)

Perfect for writing complex prompts, code blocks, or multi-paragraph requests!

### 🎨 Customization

```bash
# Themes and appearance
:config theme dark              # Switch to dark theme
:config markdown-theme dracula  # Syntax highlighting theme
:config cursor true             # Show cursor
:config border true             # Show message borders

# User experience
:config wrap true               # Word wrap long messages  
:config timestamps true         # Show message timestamps
:config userlabel ">"           # Customize user prompt
:config assistantlabel "🤖"     # Customize AI prompt
```

The TUI transforms your terminal into a powerful AI workspace - try it yourself with `genie`!

## 🏗️ Architecture

Genie follows a clean, layered architecture that separates concerns and promotes modularity:

1.  **Entry Point (`cmd/main.go`):** A thin entry point that determines whether to launch the CLI or the TUI based on the command-line arguments.
2.  **CLI Client (`cmd/cli`):** Handles direct, one-off commands. Built using the [Cobra](https://github.com/spf13/cobra) library.
3.  **TUI Client (`cmd/tui`):** Provides an interactive, terminal-based user interface. Built using the [gocui](https://github.com/awesome-gocui/gocui) library.
4.  **Genie Core (`pkg/genie`):** The core of the application, containing the business logic, service layer, event bus, and session management.
5.  **AI Engine (`pkg/ai`):** Manages prompt processing, decision-making, and interaction with the LLM.
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
*   **`pkg/ai`:** The AI engine, which manages prompt processing, decision-making, and interaction with the LLM.
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

## 💡 Inspiration & Philosophy

Genie was inspired by incredible tools in the AI-assisted development space:
- **[Aider](https://github.com/paul-gauthier/aider)** - For showing how AI can be a true coding partner
- **[Claude Code](https://claude.ai/code)** - For demonstrating powerful AI integration in development workflows

But I wanted something that gave me more control, deeper understanding, and the flexibility to extend beyond just coding into project management, note-taking, and automation. Genie embraces the Unix philosophy: do one thing well, compose freely, and integrate seamlessly into any workflow.

## 🙏 Acknowledgments

- Built with [Google Gemini AI](https://ai.google.dev/)
- TUI powered by [gocui](https://github.com/awesome-gocui/gocui)
- Distribution via [GoReleaser](https://goreleaser.com/)

---

Made with ❤️ for developers who love the command line