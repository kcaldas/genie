# Configuration

Genie can be configured through environment variables, config files, and TUI settings.

## Environment Variables

### API Configuration
```bash
# Required: Gemini API key
export GEMINI_API_KEY="your-api-key-here"

# Optional: Google Cloud (for Vertex AI)
export GOOGLE_CLOUD_PROJECT="your-project-id"
export GENAI_BACKEND="vertex"  # Default: "gemini"
```

### Model Parameters
```bash
# Model selection
export GENIE_MODEL_NAME="gemini-2.5-flash"  # Default
# Options: gemini-2.5-flash, gemini-1.5-pro, gemini-1.5-flash

# Response length
export GENIE_MAX_TOKENS="65535"  # Default

# Creativity level (0.0 = focused, 1.0 = creative)
export GENIE_MODEL_TEMPERATURE="0.7"  # Default

# Response diversity
export GENIE_TOP_P="0.9"  # Default
```

## Configuration Files

### .env File
Create a `.env` file in your working directory:
```bash
GEMINI_API_KEY=your-api-key-here
GENIE_MODEL_NAME=gemini-2.5-flash
GENIE_MODEL_TEMPERATURE=0.7
```

### TUI Settings
TUI settings are saved to `~/.genie/settings.tui.json`:
```json
{
  "theme": "dark",
  "showCursor": true,
  "markdownRendering": true,
  "glamourTheme": "dracula",
  "vimMode": false,
  "wrapMessages": true,
  "showTimestamps": false,
  "showMessagesBorder": true,
  "userLabel": ">",
  "assistantLabel": "🤖",
  "systemLabel": "■",
  "errorLabel": "✗"
}
```

## TUI Configuration

### Themes
```bash
:config theme dark          # Dark theme
:config theme light         # Light theme  
:config theme auto          # Auto detect
```

Available themes: `dark`, `light`, `auto`

### Syntax Highlighting
```bash
:config markdown-theme dracula    # Code highlighting theme
:config markdown-theme github     # GitHub style
:config markdown-theme auto       # Auto detect
```

Popular themes: `dracula`, `github`, `monokai`, `solarized-dark`, `solarized-light`

### Appearance
```bash
:config cursor true              # Show text cursor
:config border true              # Message borders
:config wrap true                # Word wrap long lines
:config timestamps true          # Show message timestamps
:config markdown false           # Disable markdown rendering
```

### Vim Mode
```bash
:config vim on              # Enable vim keybindings
:config vim off             # Disable vim mode
```

### Personalization
```bash
:config userlabel ">"           # User message prefix
:config assistantlabel "AI:"   # AI message prefix  
:config systemlabel "SYS:"     # System message prefix
:config errorlabel "ERR:"      # Error message prefix
```

### Reset Configuration
```bash
:config reset              # Reset all settings to defaults
```

## Model Behavior

### Temperature Settings
| Value | Behavior | Use Case |
|-------|----------|----------|
| 0.0-0.3 | Very focused, deterministic | Code generation, factual questions |
| 0.4-0.7 | Balanced creativity | General usage, problem solving |
| 0.8-1.0 | Highly creative | Creative writing, brainstorming |

### Token Limits
| Model | Max Tokens | Recommended |
|-------|------------|-------------|
| gemini-2.5-flash | 1M | 65535 |
| gemini-1.5-pro | 2M | 65535 |
| gemini-1.5-flash | 1M | 65535 |

## Advanced Configuration

### Multiple API Keys
```bash
# Switch between different keys
export GEMINI_API_KEY_WORK="work-key"
export GEMINI_API_KEY_PERSONAL="personal-key"

# Use specific key
GEMINI_API_KEY="$GEMINI_API_KEY_WORK" genie ask "work question"
```

### Project-Specific Settings
Create `.env` files in project directories:
```bash
# Project A
cd /project-a
echo "GENIE_MODEL_TEMPERATURE=0.3" > .env  # More focused

# Project B  
cd /project-b
echo "GENIE_MODEL_TEMPERATURE=0.8" > .env  # More creative
```

### Docker Configuration
```bash
# Pass environment variables to Docker
docker run --rm -it \
  -e GEMINI_API_KEY="$GEMINI_API_KEY" \
  -e GENIE_MODEL_TEMPERATURE="0.5" \
  ghcr.io/kcaldas/genie:latest
```

## Troubleshooting

### Configuration Priority
1. Command line flags (if any)
2. Environment variables
3. `.env` file in current directory
4. Default values

### Common Issues
**Settings not persisting**
- Check `~/.genie/` directory permissions
- TUI settings save automatically
- Environment variables need to be in shell profile

**API key not found**
```bash
# Check current value
echo $GEMINI_API_KEY

# Reload shell configuration
source ~/.bashrc  # or ~/.zshrc
```

**Model not responding**
- Check API key validity
- Verify model name spelling
- Check network connectivity