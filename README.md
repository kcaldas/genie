# 🧞 Genie - Powerful AI for Your Command Line

[![Go Version](https://img.shields.io/badge/go-1.23+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker](https://img.shields.io/badge/docker-ready-brightgreen.svg)](https://github.com/kcaldas/genie/pkgs/container/genie)
[![Beta](https://img.shields.io/badge/status-beta-orange.svg)](https://github.com/kcaldas/genie/releases)

Transform your terminal into an AI-powered workspace. Born from a developer's need for control and transparency in AI assistance.

Quick demo:

[![asciicast](https://asciinema.org/a/asMYIL7iVrpEck2CeLAI3sPqN.svg)](https://asciinema.org/a/asMYIL7iVrpEck2CeLAI3sPqN)

Theming demo:
[![asciicast](https://asciinema.org/a/RlX6vOghWR2ZIaG0gevAG5Cvp.svg)](https://asciinema.org/a/RlX6vOghWR2ZIaG0gevAG5Cvp)

## 🚀 Quick Start

### Installation

#### macOS (Homebrew)
```bash
brew tap kcaldas/genie
brew install genie
```

#### Direct Download
```bash
# Download latest release
curl -L https://github.com/kcaldas/genie/releases/latest/download/genie_$(uname -s)_$(uname -m).tar.gz | tar xz
sudo mv genie /usr/local/bin/
```

#### Docker
```bash
docker run --rm -it ghcr.io/kcaldas/genie:latest
```

#### Build from Source
```bash
go install github.com/kcaldas/genie/cmd/genie@latest
```

### Configuration
Genie ships with Gemini enabled by default. To get started:
1. Generate a key from [Google AI Studio](https://aistudio.google.com/app/apikey)
2. Set it as an environment variable:
```bash
export GEMINI_API_KEY="YOUR_API_KEY"
genie ask "hello world"              # CLI mode
git diff | genie ask "commit msg?"   # Unix pipes
genie                                # Interactive TUI mode
```

> **💡 Tip:** The Gemini API provides 100 free requests per day with Gemini 2.5 Pro. Upgrade to a paid plan for higher rate limits.

Prefer OpenAI? Switch providers with two environment variables:
```bash
export GENIE_LLM_PROVIDER="openai"
export OPENAI_API_KEY="sk-your-api-key"
genie ask "summarize README.md"
```
Optionally set `OPENAI_BASE_URL` or `OPENAI_ORG_ID` if you use a custom endpoint.

**📖 Full setup guide:** [docs/INSTALLATION.md](docs/INSTALLATION.md)

## 🎭 Personas

Genie supports different personas for specialized tasks:

```bash
# Use specific personas
genie --persona engineer ask "review this code"
genie --persona product-owner ask "plan this feature"
```

Each persona can pin its own `model_name` and `llm_provider` inside `prompt.yaml`, while `GENIE_MODEL_NAME` and `GENIE_LLM_PROVIDER` remain global fallbacks.

**📖 Learn more:** [docs/personas.md](docs/personas.md)

## 💡 Philosophy

Inspired by [Lazygit](https://github.com/jesseduffield/lazygit), [Claude Code](https://claude.ai/code), and [Aider](https://github.com/paul-gauthier/aider), and built for the Unix philosophy: composable, transparent, and adaptable.

**Core beliefs:**
- **Give you control** - Understand what's happening, not just trust a black box, and is infinitely hackable
- **Integrate naturally** - Work with your existing tools, don't replace them
- **Respect the terminal** - Embrace the power and flexibility of the command line
- **Stay composable** - Pipe, redirect, script, and automate freely

Whether you're coding, managing projects, taking notes, or automating workflows, Genie adapts to your needs.

## 📚 Documentation

- **[Installation & Setup](docs/INSTALLATION.md)** - Complete installation guide
- **[TUI Guide](docs/TUI.md)** - Interactive interface features  
- **[CLI Usage](docs/CLI.md)** - Command line examples
- **[Configuration](docs/CONFIGURATION.md)** - Customization options
- **[Personas](docs/personas.md)** - AI personality system
- **[Docker Usage](docs/DOCKER.md)** - Container setup
- **[Architecture](docs/ARCHITECTURE.md)** - How Genie works
- **[Contributing](CONTRIBUTING.md)** - Join the project

## 🔗 Ecosystem

- [kcaldas/genie.nvim](https://github.com/kcaldas/genie.nvim) - Neovim companion plugin

## 🙏 Acknowledgments

Built with [Google Gemini AI](https://ai.google.dev/) • [gocui](https://github.com/awesome-gocui/gocui) • [GoReleaser](https://goreleaser.com/)

---

Made with ❤️ for developers who love the command line