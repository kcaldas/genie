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
╭ Chat ───────────────────────────────╮╭ Help/Diff/Debug/Todos╮
│ messages (markdown, streaming)      ││ right panel          │
│                                     ││ (F1 / F12 / tools)   │
╰─────────────────────────────────────╯╰──────────────────────╯
╭──────────────────── Shift+Enter newline · F4 expand ─╮
│ > prompt: grows to 8 rows, pastes collapse to chips  │
╰──────────────────────────────────────────────────────╯
 Ready            Debug is ON        Tokens: 1.2K | Msgs: 4 | Mem: 31MB
```

Dialogs (the LLM context viewer, confirmations, pickers) open above this
and take every key until they close.

## Keys

| Key | Action |
|---|---|
| Enter | Send the prompt; `:` runs a command, `/` a slash command |
| Shift+Enter, Alt+Enter, Ctrl+J, or `\` then Enter | New line in the prompt (Shift+Enter needs the kitty keyboard protocol; in tmux set `extended-keys on`) |
| Esc | Cancel the running request |
| Up / Down | Prompt history on a one-line prompt (or its first/last line); otherwise move between lines; scroll when messages focused |
| Tab | Cycle focus: prompt, messages, right panel |
| PgUp / PgDn, Ctrl+U / Ctrl+D | Page the right panel if shown, else the chat |
| Home / End | Top / bottom |
| Mouse wheel | Scroll the panel under the pointer |
| F1 | Toggle help in the right panel |
| F12 | Toggle the debug log in the right panel |
| F4 | Expand the prompt to the full height (vi keys with `VimMode`) |
| Ctrl+Z | Zoom the right panel (or a confirmation dialog) |
| Ctrl+/ | LLM context viewer |
| Ctrl+P, Shift+Tab | Next persona |
| Ctrl+V, Alt+V, Shift+Insert | Paste; three or more lines (or 400+ characters) collapse to a `[Pasted N lines]` chip that expands on send |
| Ctrl+Shift+V | Paste into the expanded prompt |
| `/` on an empty prompt | Command palette: fuzzy list of `/` and `:` commands |
| `@` | File picker: fuzzy list of workspace files, inserts `@path` |
| Ctrl+R | Search prompt history |
| Ctrl+G | Edit the prompt in `$VISUAL` / `$EDITOR` |
| Alt+Z, Ctrl+_ / Alt+Shift+Z, Ctrl+Y | Undo / redo in the prompt |
| y / Y | Copy the last / all messages (messages focused) |
| Ctrl+O, or click a card | Expand or collapse tool output and thinking |
| Ctrl+T | Toggle the todo panel (opens by itself on the first todo list) |
| Ctrl+C | Quit on an empty prompt; clears a non-empty one |

Expanded prompt (F4 or `:write`): the prompt takes the full height; Enter
still sends, Esc or F4 collapses. With `VimMode` on, the expanded prompt
uses vi keys: insert, normal and command modes (`i a A o O h j k l w W b B
x dd d$ d0 c$ c0 $ 0 gg G`), `:w` sends, `:q` collapses; Esc returns to
normal mode. The status bar centre shows the vi mode.

Tool calls appear as one-line cards (`● bash ls -la … ✓ 3 lines`); Ctrl+O
expands them all, a click expands one. Successful file writes expand to the
diff; failures show the error. Thinking collapses to its first line.

Confirmations: `1`/`y` accept once, `a` accept this tool for the rest of
the session (kept in memory only), `A` accept it always (only that tool's
entry is written to `.genie/settings.tui.json`), `2`/`n`/Esc decline,
arrows scroll the diff. On a file-write diff the tool is `writeFile`, so
`a` applies every later write this session without showing the diff.

While a request runs, Enter queues the next message (the status bar shows
`· 1 queued`) and sends it when the reply lands; Esc cancels the request
and drops the queue.

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
