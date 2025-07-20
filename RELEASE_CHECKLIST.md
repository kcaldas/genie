# Release Checklist - v0.1.5

## Pre-Release

- [x] Security review complete
- [x] Documentation updated
- [x] GitHub templates and policies added
- [x] CI/CD workflows configured
- [ ] Version updated in code
- [ ] Changelog created
- [ ] GoReleaser snapshot test

## Release Process

### 1. Version Update
```bash
# Update version in relevant files if needed
grep -r "0.0.0" . --exclude-dir=.git
```

### 2. Create Changelog
```bash
# Create CHANGELOG.md with initial release notes
```

### 3. Test Release Process
```bash
# Test GoReleaser snapshot
goreleaser release --snapshot --clean

# Test Docker build
docker build -f Dockerfile.local -t genie:test .
```

### 4. Create Release
```bash
# Tag and push
git tag v0.1.5
git push origin v0.1.5

# This triggers GitHub Actions release workflow
```

## Post-Release

- [ ] Verify GitHub release created
- [ ] Test downloads work
- [ ] Test Docker images
- [ ] Test Homebrew cask
- [ ] Announce release
- [ ] Update documentation links

## Beta Release Notes

### 🎉 Genie v0.1.5

**Powerful AI for Your Command Line**

First beta release of Genie - a transparent, controllable AI assistant following Unix principles.

#### ✨ Features
- **Interactive TUI**: Rich terminal interface with vim-like navigation
- **Direct CLI**: Simple commands for quick tasks
- **Personas**: Customizable AI personalities
- **Tool System**: File operations, git integration, sequential thinking
- **Cross-Platform**: macOS, Linux, Windows support
- **Docker Ready**: Secure containerized environment

#### 📦 Installation
```bash
# Homebrew (macOS)
brew install --cask genie

# Docker
docker run ghcr.io/kcaldas/genie:v0.1.5

# Binary downloads available on GitHub releases
```

#### 🔧 Configuration
```bash
export GEMINI_API_KEY="your-key"
genie  # Start interactive mode
```

#### 🎯 Philosophy
Built for developers who value:
- **Control**: You decide what runs
- **Transparency**: See exactly what's happening
- **Unix Principles**: Do one thing well
- **Local First**: Your data stays with you

This is a beta release - expect some rough edges. We appreciate your feedback!

---

Ready to experience AI that respects your workflow? Give Genie a try! 🧞‍♂️