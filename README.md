# 🧞 Genie - Powerful AI for Your Command Line

[![Go Version](https://img.shields.io/badge/go-1.23+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker](https://img.shields.io/badge/docker-ready-brightgreen.svg)](https://github.com/kcaldas/genie/pkgs/container/genie)
[![Beta](https://img.shields.io/badge/status-beta-orange.svg)](https://github.com/kcaldas/genie/releases)

Transform your terminal into an AI-powered workspace. Born from a developer's need for control and transparency in AI assistance.

## 🚀 Quick Start

### Installation
```bash
# Download latest release
curl -L https://github.com/kcaldas/genie/releases/latest/download/genie_$(uname -s)_$(uname -m).tar.gz | tar xz
sudo mv genie /usr/local/bin/

# Or use Docker
docker run --rm -it ghcr.io/kcaldas/genie:latest

# Or build from source
go install github.com/kcaldas/genie/cmd@latest
```

### Configuration
Set your API key and start using:
```bash
export GEMINI_API_KEY="your-api-key-here"
genie ask "hello world"  # CLI mode
genie                    # Interactive TUI mode
```

**📖 Full setup guide:** [docs/INSTALLATION.md](docs/INSTALLATION.md)

## 🎭 Personas

Genie supports different personas for specialized tasks:

```bash
# Use specific personas
genie --persona engineer ask "review this code"
genie --persona product-owner ask "plan this feature"
```

**📖 Learn more:** [docs/PERSONAS.md](docs/PERSONAS.md)

## 💡 Philosophy

Inspired by [Aider](https://github.com/paul-gauthier/aider) and [Claude Code](https://claude.ai/code), but built for the Unix philosophy: composable, transparent, and adaptable.

**Core beliefs:**
- **Give you control** - Understand what's happening, not just trust a black box
- **Integrate naturally** - Work with your existing tools, don't replace them
- **Respect the terminal** - Embrace the power and flexibility of the command line
- **Stay composable** - Pipe, redirect, script, and automate freely

Whether you're coding, managing projects, taking notes, or automating workflows, Genie adapts to your needs.

## 📚 Documentation

- **[Installation & Setup](docs/INSTALLATION.md)** - Complete installation guide
- **[TUI Guide](docs/TUI.md)** - Interactive interface features  
- **[CLI Usage](docs/CLI.md)** - Command line examples
- **[Configuration](docs/CONFIGURATION.md)** - Customization options
- **[Personas](docs/PERSONAS.md)** - AI personality system
- **[Docker Usage](docs/DOCKER.md)** - Container setup
- **[Architecture](docs/ARCHITECTURE.md)** - How Genie works
- **[Contributing](CONTRIBUTING.md)** - Join the project

## 🙏 Acknowledgments

Built with [Google Gemini AI](https://ai.google.dev/) • [gocui](https://github.com/awesome-gocui/gocui) • [GoReleaser](https://goreleaser.com/)

---

Made with ❤️ for developers who love the command line