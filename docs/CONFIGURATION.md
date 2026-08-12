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

# Optional: Switch to OpenAI
export GENIE_LLM_PROVIDER="openai"
export OPENAI_API_KEY="sk-your-api-key"

# Optional: Advanced OpenAI configuration
export OPENAI_BASE_URL="https://api.openai.com/v1"
export OPENAI_ORG_ID="org_123"

# Optional: Switch to Anthropic (Claude)
export GENIE_LLM_PROVIDER="anthropic"
export ANTHROPIC_API_KEY="sk-ant-api-key"
# Optional: surface Claude's thinking blocks in the UI
export ANTHROPIC_SHOW_THINKING="true"
```

> Personas can override both `GENIE_MODEL_NAME` and `GENIE_LLM_PROVIDER` by specifying `model_name` and `llm_provider` in their `prompt.yaml`; the environment variables remain the global fallback.

### Model Parameters
```bash
# Model selection
export GENIE_MODEL_NAME="gemini-3.6-flash"  # Default
# Options: gemini-3.6-flash, gemini-2.5-flash, gemini-1.5-pro, gemini-1.5-flash

# Response length
export GENIE_MAX_TOKENS="65535"  # Default

# Creativity level (0.0 = focused, 1.0 = creative)
export GENIE_MODEL_TEMPERATURE="0.7"  # Default

# Response diversity
export GENIE_TOP_P="0.9"  # Default

# Default persona (built-in or custom)
export GENIE_PERSONA="genie"  # Default

# Operational allocation guard for one tool result body
export GENIE_MAX_TOOL_RESULT_BYTES="20971520"  # Default (20 MiB)

# Optional fixed guard for combined text from one tool-call step
export GENIE_MAX_TOOL_BATCH_BYTES="0"  # Default (disabled)

# Largest decoded native attachment accepted from one tool result
export GENIE_MAX_ATTACHMENT_BYTES="20971520" # Default (20 MiB)

# Provider metadata lookup deadline and optional cache isolation namespace
export GENIE_CAPABILITY_DISCOVERY_TIMEOUT="5s"
export GENIE_CAPABILITY_CACHE_NAMESPACE="agent-id"
```

Genie has two intentionally separate context controls. `GENIE_CONTEXT_BUDGET`
controls material retained and reconstructed across user turns: chat history,
remembered files, and other context providers. Tool calls, reasoning, and tool
results created inside one `ai.Gen` call form a transient turn workspace. They
accumulate only until the model produces the final answer and are then
discarded rather than charged to the next turn's synthetic context budget.

Before each follow-up model request, Genie uses the selected model's physical
input limit and the previous internal step's actual token usage to calculate
room for the pending tool results. Only those newly produced results are
reduced when they cannot fit. This lets a 100K retained context budget on a 1M
model use most of the remaining model window temporarily for a broad search,
without carrying that search transcript through the rest of the session.

Every result is normalized before any limit applies: a failed call
becomes text, structured JSON is serialized centrally, and binary
content remains a typed blob. `GENIE_MAX_TOOL_RESULT_BYTES` is therefore a
generous per-result operational allocation guard, not the normal semantic
context policy. Oversized text is truncated with a notice telling the model
that the result is incomplete and to narrow the call. Handler errors follow
the same path. Native image and document content does not spend this text
guard. `GENIE_MAX_ATTACHMENT_BYTES` is the equivalent operational ceiling per
decoded blob, including untrusted MCP media.

`GENIE_MAX_TOOL_BATCH_BYTES` is an optional additional fixed ceiling for a
whole tool-call step. It is disabled by default because physical admission
already adapts to the actual model and current transient workspace. When set,
the smaller of this ceiling and the physical allowance is spent in execution
order, so earlier results keep full fidelity and later ones tighten.

Whether a blob can be rendered is decided per provider when the request
is built. What a provider cannot display is reported in the text with
its type and size, so the model learns the content exists instead of
silently receiving nothing. Native content still consumes the model's real
input window. Genie discovers the selected model's capabilities through
Gemini, Anthropic, Ollama, and LM Studio metadata APIs and caches them. OpenAI
currently uses the static model registry because its model metadata endpoint
does not publish context limits. All providers use reported usage for generic
transient text admission. Gemini and Anthropic additionally count the complete
provider-native request after assembly. Media that does not fit is replaced by
a correlated omission notice; if minimal correlated tool results still cannot
fit, the turn fails before generation with a clear input-limit error.

Setting `GENIE_CONTEXT_BUDGET` to the full model window does not disable blobs
or tool calls; it simply leaves less transient room when the retained context
actually fills that budget.

Set any operational limit variable to `0` to disable that specific fixed cap.
Physical model admission still applies. Positive text values below 4096 are
raised to 4096, below which an omission or truncation notice may not fit.

### Shared Capability Cache

Capability discovery is optional and does not change `ai.Gen`. By default,
each Genie instance uses an in-memory cache. Hosts that cycle instances should
share one resolver at the agent lifetime:

```go
resolver := ai.NewCapabilityResolver(ai.NewMemoryCapabilityStore())

first, err := genie.NewGenie(genie.WithCapabilityResolver(resolver))
// Later instances reuse cached entries and concurrent refreshes.
next, err := genie.NewGenie(genie.WithCapabilityResolver(resolver))
```

Implement `ai.CapabilityStore` and pass it to `ai.NewCapabilityResolver` for a
durable or distributed cache. Cache keys include schema version, provider,
authority, namespace, and model. `WithCapabilityStore` is a convenience when
only persistence is shared; sharing the resolver also coalesces concurrent
refreshes across Genie instances. Expired entries are used stale when a
provider refresh fails.

### Debugging
```bash
# Show internal LLM thoughts in output
export GEMINI_SHOW_THOUGHTS="true" # Default: "false"

# Ask Gemini to include internal reasoning/thoughts in responses
export GEMINI_INCLUDE_THOUGHTS="true" # Default: "false"
```

## Configuration Files

### .env File
Create a `.env` file in your working directory:
```bash
GEMINI_API_KEY=your-api-key-here
GENIE_MODEL_NAME=gemini-3.6-flash
GENIE_MODEL_TEMPERATURE=0.7
```

### TUI Settings
TUI settings support both global and local configurations:

**Global Config**: `~/.genie/settings.tui.json` (system-wide defaults)
**Local Config**: `.genie/settings.tui.json` (project-specific overrides)

Configuration hierarchy: `defaults → global → local`

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

### Configuration Scopes
```bash
# Local config (project-specific, saves to .genie/settings.tui.json)
:config theme dark              # Set theme for current project only

# Global config (system-wide, saves to ~/.genie/settings.tui.json)  
:config --global theme dark     # Set theme globally for all projects
```

**Local configs override global configs**, allowing you to set global defaults and project-specific customizations.

### Themes
```bash
:config theme dark              # Dark theme (local)
:config theme light             # Light theme (local)
:config theme auto              # Auto detect (local)
:config --global theme dark     # Dark theme (global)
```

Available themes: `dark`, `light`, `auto`

### Syntax Highlighting
```bash
:config markdown-theme dracula         # Code highlighting theme (local)
:config markdown-theme github          # GitHub style (local)
:config markdown-theme auto            # Auto detect (local)
:config --global markdown-theme dracula # Global syntax theme
```

Popular themes: `dracula`, `github`, `monokai`, `solarized-dark`, `solarized-light`

### Appearance
```bash
:config cursor true                     # Show text cursor (local)
:config border true                     # Message borders (local)
:config wrap true                       # Word wrap long lines (local)
:config timestamps true                 # Show message timestamps (local)
:config markdown false                  # Disable markdown rendering (local)
:config --global cursor true            # Global cursor setting
```

### Vim Mode
```bash
:config vim on                          # Enable vim keybindings (local)
:config vim off                         # Disable vim mode (local)
:config --global vim on                 # Enable vim globally
```

### Personalization
```bash
:config userlabel ">"                   # User message prefix (local)
:config assistantlabel "AI:"           # AI message prefix (local)
:config systemlabel "SYS:"             # System message prefix (local)
:config errorlabel "ERR:"              # Error message prefix (local)
:config --global userlabel ">"         # Global user prefix
```

### Tool Configuration
```bash
:config tool TodoWrite hide true       # Hide tool output (local)
:config tool bash accept true          # Auto-accept tool (local)
:config --global tool TodoWrite hide true  # Global tool settings
```

### Reset Configuration
```bash
:config reset                          # Remove local config file (reverts to global/defaults)
:config --global reset                 # Reset global settings to defaults
```

**Important**: Local reset removes the local config file entirely, allowing global configuration to take effect. Global reset overwrites the global config file with defaults.

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
| gemini-3.6-flash | 1M | 65536 |
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
