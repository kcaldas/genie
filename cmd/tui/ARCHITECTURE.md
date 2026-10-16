# TUI Architecture

The TUI is a Bubble Tea v2 program. Everything that draws lives in
`cmd/tui/ui`; everything that talks to the Genie core lives in
`cmd/tui/controllers`; the two meet only through messages.

```
   Genie core ──events──▶ controllers ──Dispatcher──▶ ui.Model (Update/View)
        ▲                     ▲                             │
        └──────── calls ──────┘◀──── command bus ───────────┘
```

## Threading model

Bubble Tea runs `Update` and `View` on one goroutine. Controllers run on
event-bus goroutines. They never touch UI state: they mutate `state.*`
(mutex protected) and dispatch a `uimsg` value through
`types.Dispatcher`. `ui/bridge.Dispatcher` is a buffered channel the root
model drains from `Init` (each delivery re-arms `Next`), so controllers may
start talking before the program runs and order is preserved.

The UI sends intent the other way as command-bus events (`user.input.text`,
`user.input.command`, `user.input.slashcommand`, `user.input.cancel`,
`confirmation.answered`, `debug.view`, `shortcut.help`, `context.refresh`,
...). `ui/bridge.Subscribe` turns the UI-relevant bus topics
(`theme.changed`, `persona.changed`, `request.*`, `token.count`,
`paste.multiline`, `vim.mode.changed`, `ui.cursor.changed`,
`ui.mouse.changed`, `app.exit`) into typed messages.

## Packages

| Package | Role |
|---|---|
| `program.go` | `NewProgram` wires core, controllers, commands and the root model by hand; `Run`/`Stop` |
| `ui/app.go` | Root `tea.Model`: layout, focus, key routing, dialog stack, overlay compositing |
| `ui/keymap.go` | `bubbles/key` bindings; also the source of the help page's shortcut list |
| `ui/layout` | Pure geometry: `Compute(w, h, State) Rects` |
| `ui/style` | Theme hex colours as Lip Gloss styles; `RenderFrame`, `Fit` |
| `ui/chatview` | Transcript: per-message render cache, coalesced streaming flush, stable-prefix incremental glamour, follow-tail |
| `ui/input` | Prompt over `editor.Line`; routes submit by prefix; multi-line paste and F4 open write mode |
| `ui/editor` | `Buffer` (rune lines, snapshots), `Prompt` (multi-line prompt: chips, history, completion, undo), `Vi` (vi state machine for the expanded prompt) |
| `ui/sidepanel` | Right column: text viewer (glamour), diff viewer, debug log |
| `ui/dialog` | `Stack` plus `ContextViewer` (full screen), `Confirm` and `Picker` (overlays) |
| `ui/statusbar` | Ready / Thinking / Your call, counters; ticks only while busy |
| `ui/render` | Shared markdown and diff rendering |
| `ui/uimsg` | Plain message structs controllers dispatch |
| `controllers` | `ChatController`, `SlashCommandController`, `HelpPanel`, `DebugPanel`, `LLMContextPanel`, `WritePanel`, `ConfirmationPanel` |
| `controllers/commands` | `:` commands and the registry; unchanged by the UI |
| `state` | `ChatState` (stable message IDs), `UIState`, `DebugState` |
| `presentation` | Themes, message/tool/diff formatters producing ANSI text |
| `help` | Man-page help renderer over the registry and `help.Shortcut`s |

## Rendering rules

- `View` does no formatting work. Chat lines are cached per message, width
  and theme generation; only the streaming message is re-rendered, and only
  its unsettled tail goes through glamour each flush (33 ms coalescing).
- Panels are framed with `style.RenderFrame` and composed with Lip Gloss
  joins; dialogs that float use `lipgloss.Compositor`.
- Key dispatch order: open dialog, then global shortcuts, then the focused
  panel. A panel's own binding beats a global one for the same key.

## Testing

- Pure models are driven with `Update` and asserted through
  `ansi.Strip(View())` (`ui/*_test.go`).
- `program_test.go` runs the whole program headless with `teatest` over the
  `genietest` fixture. `teatest.WaitFor` only sees output since the previous
  wait, so needles from one redraw are waited for together.
- `scripts/tui-smoke.py` drives the real binary in a pseudo-terminal.
- `make tui-gates` runs gofmt, vet, golangci-lint, builds (incl. Windows),
  the race-enabled suite, golden tests and the smoke.
