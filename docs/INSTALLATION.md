# Installation & Setup

## Quick Install

### Option 1: Download Binary
```bash
# macOS/Linux
curl -L https://github.com/kcaldas/genie/releases/latest/download/genie_$(uname -s)_$(uname -m).tar.gz | tar xz
sudo mv genie /usr/local/bin/

# Windows (PowerShell)
curl -L https://github.com/kcaldas/genie/releases/latest/download/genie_Windows_x86_64.zip -o genie.zip
Expand-Archive genie.zip
```

### Option 2: Package Managers
```bash
# Homebrew (coming soon)
brew tap kcaldas/genie
brew install genie

# Docker
docker run --rm -it ghcr.io/kcaldas/genie:latest
```

### Option 3: Build from Source
```bash
# Requires Go 1.23+
git clone https://github.com/kcaldas/genie
cd genie
go build -o genie ./cmd
sudo mv genie /usr/local/bin/
```

## Configuration

### 1. Get API Key
Get a free Gemini API key at [Google AI Studio](https://makersuite.google.com/app/apikey).

### 2. Set Environment Variable
```bash
# Add to your shell profile (~/.bashrc, ~/.zshrc, etc.)
export GEMINI_API_KEY="your-api-key-here"

# Or create .env file in current directory
echo "GEMINI_API_KEY=your-api-key-here" > .env
```

### 3. Test Installation
```bash
# Quick test
genie ask "hello"

# Interactive mode
genie
```

## Advanced Configuration

### Environment Variables
```bash
# Model selection
export GENIE_MODEL_NAME="gemini-2.5-flash"  # Default

# Model parameters
export GENIE_MAX_TOKENS="65535"
export GENIE_MODEL_TEMPERATURE="0.7"
export GENIE_TOP_P="0.9"

# Backend selection
export GENAI_BACKEND="gemini"  # or "vertex"
export GOOGLE_CLOUD_PROJECT="your-project-id"  # For Vertex AI
```

### TUI Settings
Settings are automatically saved to `~/.genie/settings.tui.json`:

```bash
genie
:config theme dark
:config vim on
:config cursor true
```

## Troubleshooting

### Common Issues

**"command not found: genie"**
- Ensure `/usr/local/bin` is in your PATH
- Or place binary in a directory that's in your PATH

**"API key not found"**
- Check `echo $GEMINI_API_KEY` returns your key
- Restart terminal after setting environment variable

**"permission denied"**
```bash
chmod +x genie
```

**Docker permission issues**
```bash
sudo docker run --rm -it ghcr.io/kcaldas/genie:latest
```

### Getting Help
- Check our [issues](https://github.com/kcaldas/genie/issues)
- Join discussions in [GitHub Discussions](https://github.com/kcaldas/genie/discussions)