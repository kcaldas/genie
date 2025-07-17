# Docker Usage

Run Genie safely in containers for isolation, testing, and distribution.

## Quick Start

### Public Image
```bash
# Interactive mode
docker run --rm -it \
  -e GEMINI_API_KEY="your-key" \
  ghcr.io/kcaldas/genie:latest

# CLI mode
docker run --rm \
  -e GEMINI_API_KEY="your-key" \
  ghcr.io/kcaldas/genie:latest ask "hello world"
```

### With Your Code
```bash
# Mount current directory (read-only)
docker run --rm -it \
  -v "$(pwd):/workspace:ro" \
  -w /workspace \
  -e GEMINI_API_KEY="your-key" \
  ghcr.io/kcaldas/genie:latest ask "analyze this codebase"
```

## Docker Script

Use the included convenience script:

```bash
# Build and run local image
./docker-run.sh --build-local

# Use with environment variables
GEMINI_API_KEY="your-key" ./docker-run.sh ask "hello"

# Interactive mode
./docker-run.sh

# Help
./docker-run.sh --help
```

## Building Locally

### For Development
```bash
# Build with full source (includes Go build)
docker build -f Dockerfile.local -t genie:local .

# Run your build
docker run --rm -it \
  -e GEMINI_API_KEY="your-key" \
  genie:local
```

### For Distribution
```bash
# Build binary first (requires GoReleaser)
goreleaser release --snapshot --clean

# Docker image will be created automatically
docker run --rm -it genie:local
```

## Configuration

### Environment Variables
```bash
docker run --rm -it \
  -e GEMINI_API_KEY="your-key" \
  -e GENIE_MODEL_NAME="gemini-2.5-flash" \
  -e GENIE_MODEL_TEMPERATURE="0.7" \
  ghcr.io/kcaldas/genie:latest
```

### Persistent Configuration
```bash
# Mount genie config directory
docker run --rm -it \
  -v "$HOME/.genie:/home/genie/.genie" \
  -e GEMINI_API_KEY="your-key" \
  ghcr.io/kcaldas/genie:latest
```

### Working with Files
```bash
# Read-only workspace
docker run --rm -it \
  -v "$(pwd):/workspace:ro" \
  -w /workspace \
  -e GEMINI_API_KEY="your-key" \
  ghcr.io/kcaldas/genie:latest ask "review this code" < main.go

# Read-write for file modifications
docker run --rm -it \
  -v "$(pwd):/workspace" \
  -w /workspace \
  -e GEMINI_API_KEY="your-key" \
  ghcr.io/kcaldas/genie:latest
```

## Security

### Why Use Docker?
- **Isolation:** Genie runs in contained environment
- **Safety:** No access to host system files (unless mounted)
- **Consistency:** Same environment everywhere
- **Easy testing:** Try without installation

### Security Features
- **Non-root user:** Runs as `genie` user (UID 1001)
- **Minimal base:** Alpine Linux (~15MB total)
- **Read-only workspace:** Default mounting is read-only
- **No persistence:** Container destroyed after use

### Best Practices
```bash
# Read-only mounts for safety
-v "$(pwd):/workspace:ro"

# Specific file access only
-v "$(pwd)/config.yaml:/workspace/config.yaml:ro"

# Never mount sensitive directories
# DON'T: -v "$HOME:/home" 
# DON'T: -v "/:/host"
```

## Use Cases

### Testing New Versions
```bash
# Test beta version safely
docker run --rm -it \
  -e GEMINI_API_KEY="your-key" \
  ghcr.io/kcaldas/genie:beta
```

### CI/CD Integration
```bash
# In GitHub Actions
- name: Code Review with Genie
  run: |
    docker run --rm \
      -v "${{ github.workspace }}:/workspace:ro" \
      -w /workspace \
      -e GEMINI_API_KEY="${{ secrets.GEMINI_API_KEY }}" \
      ghcr.io/kcaldas/genie:latest \
      ask "review this pull request" < changes.diff
```

### Team Sharing
```bash
# Dockerfile for team use
FROM ghcr.io/kcaldas/genie:latest
ENV GENIE_MODEL_TEMPERATURE=0.5
ENV GENIE_MAX_TOKENS=32000
# Team-specific defaults
```

### Development Environment
```bash
# Development with live reload
docker run --rm -it \
  -v "$(pwd):/workspace" \
  -v "$HOME/.genie:/home/genie/.genie" \
  -w /workspace \
  -e GEMINI_API_KEY="your-key" \
  genie:local
```

## Troubleshooting

### Common Issues

**Permission denied**
```bash
# Fix ownership after file operations
sudo chown -R $USER:$USER .
```

**Container exits immediately**
```bash
# Check if you provided required environment variables
docker run --rm \
  -e GEMINI_API_KEY="your-key" \
  ghcr.io/kcaldas/genie:latest --help
```

**Files not visible in container**
```bash
# Ensure correct mount path
docker run --rm -it \
  -v "$(pwd):/workspace:ro" \
  -w /workspace \
  ghcr.io/kcaldas/genie:latest
  
# Debug: list files in container
docker run --rm \
  -v "$(pwd):/workspace:ro" \
  ghcr.io/kcaldas/genie:latest \
  ls -la /workspace
```

**TUI not working**
```bash
# Ensure TTY allocation
docker run --rm -it \  # Note the -it flags
  ghcr.io/kcaldas/genie:latest
```

### Debug Mode
```bash
# Run with debug output
docker run --rm -it \
  -e GEMINI_API_KEY="your-key" \
  ghcr.io/kcaldas/genie:latest
# Then in TUI: :debug on
```

## Advanced Usage

### Custom Entrypoint
```bash
# Run custom commands
docker run --rm -it \
  --entrypoint /bin/sh \
  ghcr.io/kcaldas/genie:latest
```

### Multi-stage Builds
```dockerfile
# Custom image with your tools
FROM ghcr.io/kcaldas/genie:latest as genie
FROM alpine:3.20

# Install your tools
RUN apk add --no-cache git curl jq

# Copy genie
COPY --from=genie /usr/local/bin/genie /usr/local/bin/genie

ENTRYPOINT ["genie"]
```

### Docker Compose
```yaml
# docker-compose.yml
version: '3.8'
services:
  genie:
    image: ghcr.io/kcaldas/genie:latest
    environment:
      - GEMINI_API_KEY=${GEMINI_API_KEY}
    volumes:
      - .:/workspace:ro
      - ~/.genie:/home/genie/.genie
    working_dir: /workspace
    stdin_open: true
    tty: true
```

Run with: `docker-compose run --rm genie`