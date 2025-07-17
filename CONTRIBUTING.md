# Contributing to Genie

Thanks for your interest in contributing! Genie is built with the community in mind.

## Quick Start

### 1. Setup Development Environment
```bash
# Clone and setup
git clone https://github.com/kcaldas/genie
cd genie
go mod download

# Build and test
go build -o genie ./cmd
go test ./...
```

### 2. Make Your Changes
```bash
# Create feature branch
git checkout -b feature/your-feature

# Make changes
# ... edit files ...

# Test your changes
go test ./...
./genie ask "test my changes"
```

### 3. Submit Pull Request
```bash
# Commit with clear message
git commit -m "feat: add new feature"

# Push and create PR
git push origin feature/your-feature
```

## Development

### Project Structure
```
├── cmd/           # Entry points (CLI, TUI)
├── pkg/           # Core packages
│   ├── genie/     # Main business logic
│   ├── ai/        # AI engine
│   ├── tools/     # Tool implementations
│   └── events/    # Event system
├── docs/          # Documentation
└── internal/      # Internal packages
```

### Build Commands
```bash
# Build binary
go build -o genie ./cmd

# Run tests
go test ./...

# Run with race detection
go test -race ./...

# Generate wire dependencies
go generate ./...

# Format code
go fmt ./...
```

### Code Style
- Follow standard Go conventions
- Use `gofmt` for formatting
- Write tests for new features
- Document public APIs

## Contributing Areas

### 🔧 Tools
Add new capabilities for AI to use:

```go
type MyTool struct{}

func (t *MyTool) Declaration() ai.Tool {
    return ai.Tool{
        Name: "my-tool",
        Description: "What this tool does",
        Parameters: /* JSON schema */,
    }
}

func (t *MyTool) Handler() ai.HandlerFunc {
    return func(ctx context.Context, args map[string]any) (map[string]any, error) {
        // Your implementation
    }
}
```

**Ideas:**
- Database tools (SQL, NoSQL)
- API testing tools
- Code analysis tools
- Documentation generators

### 🎨 TUI Components
Enhance the interactive interface:

```go
type NewComponent struct {
    BaseComponent
}

func (c *NewComponent) Render(g *gocui.Gui) error {
    // UI rendering
}
```

**Ideas:**
- File browser component
- Syntax highlighting improvements
- New themes
- Accessibility features

### 🤖 AI Enhancements
Improve AI capabilities:

**Ideas:**
- New LLM backends (OpenAI, Anthropic)
- Advanced prompt engineering
- Context management
- Response streaming improvements

### 📚 Documentation
Help others understand and use Genie:

**Areas:**
- Tutorial content
- Use case examples
- API documentation
- Video guides

## Guidelines

### Code Quality
- Write clear, readable code
- Add tests for new functionality
- Follow existing patterns
- Document complex logic

### Git Workflow
```bash
# Use conventional commits
git commit -m "feat: add new tool"
git commit -m "fix: resolve TUI bug"
git commit -m "docs: update installation guide"
```

**Commit Types:**
- `feat`: New features
- `fix`: Bug fixes
- `docs`: Documentation
- `test`: Tests
- `refactor`: Code refactoring
- `style`: Formatting changes

### Pull Requests
- Clear description of changes
- Reference related issues
- Include tests
- Update documentation

### Testing
```bash
# Test specific package
go test ./pkg/tools

# Test with coverage
go test -cover ./...

# Integration tests
go test -tags=integration ./...
```

## Community

### Getting Help
- 💬 [GitHub Discussions](https://github.com/kcaldas/genie/discussions)
- 🐛 [Issue Tracker](https://github.com/kcaldas/genie/issues)
- 📧 Email: [maintainer email]

### Reporting Issues
Use our issue templates:
- 🐛 Bug reports
- 💡 Feature requests
- 📚 Documentation improvements
- ❓ Questions

### Code of Conduct
Be respectful, inclusive, and helpful. We want Genie to be welcoming for everyone.

## Recognition

Contributors are recognized in:
- README acknowledgments
- Release notes
- Hall of fame (coming soon)

## Development Tips

### TDD Workflow
```bash
# 1. Write failing test
go test ./pkg/tools -run TestMyFeature

# 2. Implement feature
# ... code changes ...

# 3. Make test pass
go test ./pkg/tools -run TestMyFeature

# 4. Refactor if needed
```

### Debugging
```bash
# Debug TUI
genie
:debug on

# Debug CLI
GENIE_DEBUG=1 genie ask "test"

# Use Go debugger
dlv debug ./cmd
```

### Performance
```bash
# Profile CPU usage
go test -cpuprofile=cpu.prof ./pkg/genie

# Profile memory
go test -memprofile=mem.prof ./pkg/genie

# Benchmark
go test -bench=. ./pkg/tools
```

### Docker Development
```bash
# Build local image
docker build -f Dockerfile.local -t genie:dev .

# Test in container
docker run --rm -it genie:dev
```

## Release Process

### Versioning
We use [Semantic Versioning](https://semver.org/):
- `MAJOR.MINOR.PATCH`
- `v1.0.0`, `v1.1.0`, `v1.0.1`

### Release Steps
1. Update version in code
2. Update CHANGELOG.md
3. Create release tag
4. GoReleaser handles the rest

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

**Thank you for contributing to Genie!** 🧞‍♂️