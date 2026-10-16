# Genie TUI

The interactive terminal UI for Genie, built on Bubble Tea v2 with Lip Gloss
and Glamour. It is a client of the Genie core: controllers talk to the core
and to the command bus, the UI renders state and sends user intent back as
bus events.

## Running

```bash
./build/genie
echo "hello" | ./build/genie    # stdin becomes the first message
```

## Layout

```
╭ Chat ───────────────────────────────╮╭ Help / Diff / Debug ─╮
│ messages (markdown, streaming)      ││ right panel          │
│                                     ││ (F1 / F12 / tools)   │
╰─────────────────────────────────────╯╰──────────────────────╯
╭──────────────────────────────── F4 / Ctrl+V Expand ─╮
│ > prompt with inline completion                      │
╰──────────────────────────────────────────────────────╯
 Ready            Debug is ON        Tokens: 1.2K | Msgs: 4 | Mem: 31MB
```

Dialogs (write mode, the LLM context viewer, confirmations) open above this
and take every key until they close.

## Keys

| Key | Action |
|---|---|
| Enter | Send the prompt; `:` runs a command, `/` a slash command |
| Esc | Cancel the running request |
| Up / Down | Prompt history (prompt focused) or scroll (messages focused) |
| Tab | Cycle focus: prompt, messages, right panel |
| PgUp / PgDn, Ctrl+U / Ctrl+D | Page the right panel if shown, else the chat |
| Home / End | Top / bottom |
| Mouse wheel | Scroll the panel under the pointer |
| F1 | Toggle help in the right panel |
| F12 | Toggle the debug log in the right panel |
| F4 | Write mode with the current draft |
| Ctrl+Z | Zoom the right panel (or a confirmation dialog) |
| Ctrl+/ | LLM context viewer |
| Ctrl+P, Shift+Tab | Next persona |
| Ctrl+V, Alt+V, Shift+Insert | Paste; multi-line pastes open write mode |
| Ctrl+Shift+V | Paste into write mode |
| y / Y | Copy the last / all messages (messages focused) |
| Ctrl+C | Quit on an empty prompt; clears a non-empty one |

Write mode: Ctrl+S submits, Esc cancels, Ctrl+C/L clears. With `VimMode`
on it has normal, insert and command modes (`i a A o O h j k l w W b B x dd
d$ d0 c$ c0 $ 0 gg G`, `:q :w :wq`); Esc returns to normal mode.

Confirmations: `1`/`y` accept, `2`/`n`/Esc decline, arrows scroll the diff.

## Configuration

`~/.genie/settings.tui.json`, merged with `.genie/settings.tui.json` in the
project, edited with `:config`. Keys: `Theme` (12 themes, `:theme list`),
`GlamourTheme`, `DiffTheme`, `MarkdownRendering`, `WrapMessages`,
`ShowTimestamps`, `ShowMessagesBorder`, `ShowCursor`, `EnableMouse`,
`VimMode`, `MaxChatMessages`, the role labels, `ToolConfigs` (per-tool
`AutoAccept` and `Hide`) and `PersonaCycleList`. `OutputMode` is accepted
for compatibility; colour depth is now detected and downsampled
automatically.

## Logging

`:debug` toggles debug logging to `genie-debug.log` (see
`logging.GetDebugFilePath`); F12 shows the file in the right panel.

## Code map

See `ARCHITECTURE.md`.
