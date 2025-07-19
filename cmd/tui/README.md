# Genie gocui TUI

This is the gocui-based TUI implementation for Genie, providing better overlay support and a more organized component structure.

## Architecture

The TUI is organized into focused, maintainable files:

### Core Files

- **`tui.go`** - Main TUI struct, initialization, and core setup
- **`start.go`** - Entry point and Genie logging configuration
- **`layout.go`** - UI layout management and view positioning
- **`handlers.go`** - Key binding handlers and command processing
- **`messages.go`** - Message rendering and display logic
- **`debug.go`** - Debug panel component
- **`history.go`** - Command history navigation
- **`events.go`** - Event bus subscriptions

## Features

### ✅ Implemented
- **Split-screen debug panel** (Ctrl+D or `:debug`)
- **Command history navigation** (Up/Down arrows)
- **File-based logging** (logs to `.genie/tui.log`)
- **Modal dialog overlays** (no viewport jumping)
- **Comprehensive help system** (`:help`)
- **Focus management** (Tab to cycle, visual indicators)
- **Advanced scrolling** (Ctrl+U/D, PgUp/PgDn, Home/End)
- **Tool confirmation dialogs**
- **Tool call messages** (real-time tool progress with white ● indicators)
- **Tool execution results** (green ● success, red ● failure)
- **Markdown rendering** (rich AI response formatting)
- **Swappable renderers** (Glamour, plain text, custom)
- **Request cancellation** (ESC)

### 🚧 In Progress
- Configuration system (`:config`)
- Expandable tool results
- Context viewer
- Scrollable confirmations

## Usage

```bash
# Use gocui TUI (recommended for better overlays)
./genie --tui=gocui

# Or use the default Bubble Tea TUI
./genie
```

## Logging

The gocui TUI automatically configures file-based logging:
- **Log location**: `{project}/.genie/tui.log`
- **Log level**: Info (good balance of detail)
- **Format**: Text with timestamps
- **Scope**: All Genie components log to file

This keeps the terminal UI clean while providing full debugging information.

## Component Pattern

Each component follows gocui's manager pattern:

```go
// 1. State in TUI struct
type TUI struct {
    showDebug bool
    debugMessages []string
}

// 2. Layout logic in layout.go
if t.showDebug {
    // Create debug view
}

// 3. Event handlers in handlers.go
func (t *TUI) toggleDebugPanel(g *gocui.Gui, v *gocui.View) error {
    t.showDebug = !t.showDebug
    return nil
}

// 4. Rendering in dedicated component file
func (t *TUI) renderDebugMessages(v *gocui.View) {
    // Render logic
}
```

## Key Bindings

### Global Controls
- **Ctrl+C** - Quit
- **Tab** - Cycle focus between panels
- **ESC** - Cancel current request

### Scrolling (works on focused panel)
- **PgUp/PgDn** - Page up/down
- **Ctrl+U/Ctrl+D** - Half-page up/down  
- **Ctrl+B/Ctrl+F** - Page up/down (vi-style)
- **Home/End** - Jump to top/bottom

### Input Panel (when focused)
- **Enter** - Send message/command
- **Up/Down** - Navigate command history
- **Ctrl+D** - Toggle debug panel

### Dialog Controls
- **y/n** - Confirm/cancel dialogs
- **ESC** - Cancel dialog

### Focus Indicators
- **Yellow border** - Currently focused panel
- **Default border** - Unfocused panels

## Commands

- `:help` - Show help
- `:debug` - Toggle debug panel
- `:config` - Configure TUI settings (cursor, theme, output mode, etc.)
- `:theme [name]` - Change color theme
- `:renderer [type]` - Show/switch markdown renderer
- `:clear` - Clear messages
- `:exit` - Quit

### Configuration Options

Genie supports both global and local configuration scopes:

```bash
# Local config (project-specific, saves to .genie/settings.tui.json)
:config <setting> <value>

# Global config (system-wide, saves to ~/.genie/settings.tui.json)
:config --global <setting> <value>
```

**Local configs override global configs**, allowing you to set global defaults and project-specific customizations.

Available settings:
- `cursor` - Show/hide cursor (true/false)
- `markdown` - Enable/disable markdown rendering (true/false)
- `theme` - Change color theme (default/dracula/monokai/solarized/nord)
- `wrap` - Enable/disable message wrapping (true/false)
- `timestamps` - Show/hide timestamps (true/false)
- `border` - Show/hide border around messages panel (true/false)
- `output` - Terminal output mode:
  - `true` - 24-bit color with enhanced Unicode support (recommended)
  - `256` - 256-color mode with standard Unicode
  - `normal` - 8-color mode with basic character support

Examples:
```bash
:config theme dark                    # Local theme
:config --global theme dark           # Global theme
:config tool TodoWrite hide true      # Local tool config
:config --global tool bash accept true # Global tool config
:config reset                         # Remove local config (reverts to global)
:config --global reset                # Reset global config to defaults
```

**Note**: Changes to output mode require restarting the application. Border settings take effect immediately.

## Message Types

The TUI displays different types of messages with distinct styling:

- **User messages** (`> `) - Cyan color with prompt prefix
- **AI responses** - Green color, plain text
- **System messages** (`• `) - Yellow color with bullet prefix  
- **Error messages** (`❌ `) - Red color with error icon
- **Tool call messages** (`⚡ `) - Bright blue, real-time tool progress
- **Tool executions** (`🔧 `) - Magenta color, tool completion results

## Markdown Rendering

The TUI supports multiple markdown renderers that can be switched on-the-fly:

### Available Renderers

- **Glamour** (default): Rich markdown with syntax highlighting, themes, and full CommonMark support
- **PlainText**: No formatting, fastest performance, always available as fallback
- **Custom**: Placeholder for future goldmark-based renderer (not yet implemented)

### Renderer Commands

```bash
/renderer                    # Show current renderer info
/renderer glamour           # Switch to Glamour renderer  
/renderer plaintext         # Switch to plain text renderer
/renderer custom            # Switch to custom renderer (fallback to plaintext)
```

### Automatic Fallback

The renderer system includes automatic fallback:
1. Try preferred renderer (e.g., Glamour)
2. If unavailable, fallback to Glamour
3. If Glamour fails, fallback to plain text
4. Plain text is always available

### Architecture

The renderer is completely isolated and swappable:

```go
// Interface for easy swapping
type MarkdownRenderer interface {
    Render(content string) (string, error)
    UpdateWidth(width int) error
    IsEnabled() bool
}

// Easy to add new renderers
func NewCustomRenderer(width int) MarkdownRenderer {
    // Implement your renderer here
}
```

## Development

The organized structure makes it easy to:
- **Add new components** - Create new files following the pattern
- **Extend functionality** - Components are isolated and focused
- **Debug issues** - Debug panel shows real-time system events
- **Maintain code** - Each file has a single responsibility

## Comparison with Bubble Tea TUI

| Feature | gocui TUI | Bubble Tea TUI |
|---------|-----------|----------------|
| Overlapping views | ✅ Native support | ❌ Viewport jumping |
| Component organization | ✅ Multi-file | ✅ Multi-package |
| File logging | ✅ Automatic | ⚠️ Manual setup |
| Debug panel | ✅ Split-screen | ❌ Not available |
| Dependencies | 🔷 Minimal | 🔶 Full ecosystem |

The gocui implementation provides better overlay support while maintaining clean, organized code structure.