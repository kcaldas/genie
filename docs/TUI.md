# TUI Guide - Interactive Interface

The TUI (Text User Interface) provides a rich, interactive way to work with Genie.

## Getting Started

```bash
genie  # Launch TUI mode
```

## Interface Overview

```
┌─ Genie: Your AI Assistant ─────────────────────────────┐
│ 🤖 AI Response Area                                    │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ **Thinking (1/3)**                                  │ │
│ │ Let me analyze this step by step...                 │ │
│ │                                                     │ │
│ │ For this problem, I recommend:                      │ │
│ │ • First approach the data structure                 │ │
│ │ • Then optimize the algorithm                       │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ 💬 Input Area                                          │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ > How do I optimize this algorithm?                 │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ :help :clear :config :exit    |    Vim Mode: OFF       │
└─────────────────────────────────────────────────────────┘
```

## Key Features

### 📜 Conversation History
- Full session context maintained
- Scroll through previous responses
- Reference earlier parts of conversation

### 🧠 Thinking
Watch AI reasoning unfold in real-time:
```
**Thinking (1/4)**
Let me break this down step by step...

**Thinking (2/4)** 
Now I need to consider the edge cases...
```

### ⚡ Streaming Responses
- See responses appear as they're generated
- No waiting for complete responses
- Natural conversation flow

## Commands

| Command | Shortcut | Description |
|---------|----------|-------------|
| `:help` | `?` | Show help |
| `:clear` | `:cls` | Clear history |
| `:config` | `:cfg` | Change settings |
| `:debug` | | Toggle debug info |
| `:exit` | `:quit` | Exit TUI |

## Vim Editor Mode

### Activation
```bash
:config vim on    # Enable globally
F4               # Enter vim editor (current input)
Ctrl+V           # Alternative vim editor key
```

### Vim Commands
**Normal Mode:**
- `h/j/k/l` - Navigate
- `w/b` - Word movement
- `0/$` - Line start/end
- `gg/G` - File start/end
- `dd` - Delete line
- `A` - Append at end

**Insert Mode:**
- `i/a/o/O` - Insert text
- `ESC` - Back to normal

**Command Mode:**
- `:w` - Send message
- `:q` - Cancel input

## Customization

### Configuration Scopes
Genie supports both local and global configuration:
- **Local**: Project-specific settings (`.genie/settings.tui.json`)
- **Global**: System-wide defaults (`~/.genie/settings.tui.json`)

Local configs override global configs.

### Themes
```bash
:config theme dark              # Dark theme (local)
:config theme light             # Light theme (local)
:config theme auto              # Auto detect (local)
:config --global theme dark     # Global theme
```

### Appearance
```bash
:config cursor true                     # Show cursor (local)
:config border true                     # Message borders (local)
:config wrap true                       # Word wrap (local)
:config timestamps true                 # Show timestamps (local)
:config markdown-theme dracula          # Syntax highlighting (local)
:config --global cursor true            # Global cursor setting
```

### Personalization
```bash
:config userlabel ">"                   # User prompt (local)
:config assistantlabel "🤖"             # AI prompt (local)
:config systemlabel "■"                 # System messages (local)
:config errorlabel "✗"                  # Error messages (local)
:config --global userlabel ">"          # Global user prompt
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `F4` | Enter vim editor |
| `Ctrl+V` | Enter vim editor |
| `Ctrl+C` | Exit TUI |
| `Tab` | Command completion |

## Tips

### Multi-line Input
- Use `F4` or `Ctrl+V` for complex prompts
- Perfect for code blocks or long questions
- Vim editing makes it powerful

### Configuration
- Settings persist between sessions
- Changes take effect immediately
- Reset local with `:config reset` (removes local config file)
- Reset global with `:config --global reset` (overwrites global with defaults)
- Local configs override global configs

### Performance
- Large conversations may slow scrolling
- Use `:clear` to reset if needed
- Debug mode shows performance info