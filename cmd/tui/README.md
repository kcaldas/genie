# Genie TUI (Terminal User Interface)

This package provides the interactive REPL experience for Genie, including TUI-specific configuration management.

## Usage

Within the Genie REPL, use the `/config` command to manage TUI settings:

```
/config                    # Show current settings and usage
/config show               # Show current settings  
/config set cursor_blink true   # Enable cursor blinking
/config set cursor_blink false  # Disable cursor blinking (default)
```

## Settings

- `cursor_blink` (boolean): Controls whether the cursor blinks in the input field
  - Default: `false` (solid cursor)
  - Requires REPL restart to take effect

## Configuration File

Settings are automatically saved to `~/.genie/settings.tui.json`:

```json
{
  "cursor_blink": false
}
```

The configuration file is automatically created when you change settings. Local project settings can be stored in `.genie/settings.tui.json` (relative to project root).

## Architecture

This package is intentionally separate from core Genie functionality, as TUI settings are specific to the terminal interface and should not affect other Genie clients (CLI commands, future web interface, etc.).