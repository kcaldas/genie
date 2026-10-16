package controllers

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/cmd/tui/ui/layout"
	"github.com/kcaldas/genie/cmd/tui/ui/uimsg"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
)

// The controllers in this file drive the Bubble Tea UI through a
// types.Dispatcher. They own no view state: the UI decides what is
// visible, they decide what content to show and when.

// HelpPanel shows the help page in the right-hand text viewer.
type HelpPanel struct {
	ui       types.Dispatcher
	renderer HelpRenderer
	help     string // rendered once; the registry does not change at runtime
}

// NewHelpPanel builds a help controller over the dispatcher.
func NewHelpPanel(ui types.Dispatcher, renderer HelpRenderer) *HelpPanel {
	return &HelpPanel{ui: ui, renderer: renderer}
}

// ShowHelp shows the main help page.
func (h *HelpPanel) ShowHelp() error {
	h.ui.Dispatch(h.helpPanelFor(uimsg.PanelShow))
	return nil
}

// ToggleHelp shows the help page, or hides the panel if it is showing.
func (h *HelpPanel) ToggleHelp() error {
	h.ui.Dispatch(h.helpPanelFor(uimsg.PanelToggle))
	return nil
}

// ShowSlashCommandsHelp shows the slash command reference.
func (h *HelpPanel) ShowSlashCommandsHelp() error {
	h.ui.Dispatch(uimsg.RightPanel{
		Mode: layout.RightText, Action: uimsg.PanelShow,
		Title: "Slash Commands Help", Content: h.renderer.RenderSlashCommands(), ContentType: "markdown",
	})
	return nil
}

// helpPanelFor is the right-panel message that shows the help page.
func (h *HelpPanel) helpPanelFor(action uimsg.PanelAction) uimsg.RightPanel {
	if h.help == "" {
		h.help = h.renderer.RenderHelp()
	}
	return uimsg.RightPanel{Mode: layout.RightText, Action: action, Title: "Help", Content: h.help, ContentType: "markdown"}
}

// DebugPanel shows the debug log in the right panel and owns the debug
// log level.
type DebugPanel struct {
	ui        types.Dispatcher
	state     *state.DebugState
	clipboard interface{ Copy(string) error }
}

// NewDebugPanel builds the controller and subscribes to the debug.* bus
// topics the UI and commands emit.
func NewDebugPanel(ui types.Dispatcher, st *state.DebugState, clipboard interface{ Copy(string) error }, bus *events.CommandEventBus) *DebugPanel {
	d := &DebugPanel{ui: ui, state: st, clipboard: clipboard}
	bus.Subscribe(uimsg.TopicDebugView, func(interface{}) {
		LoadDebugLog(d.state)
		d.ui.Dispatch(uimsg.RightPanel{Mode: layout.RightDebug, Action: uimsg.PanelToggle})
	})
	bus.Subscribe(uimsg.TopicDebugClear, func(interface{}) {
		d.state.ClearDebugMessages()
		d.ui.Dispatch(uimsg.DebugChanged{})
	})
	bus.Subscribe(uimsg.TopicDebugCopy, func(interface{}) {
		if d.clipboard != nil {
			_ = d.clipboard.Copy(strings.Join(d.state.GetDebugMessages(), "\n"))
		}
	})
	return d
}

// UpdateLogLevel switches the global file logger to the given level.
func (d *DebugPanel) UpdateLogLevel(level string) { UpdateLogLevel(level) }

// UpdateLogLevel installs a fresh file logger at the named level
// ("debug", "info", "warn", "error"; anything else means error).
func UpdateLogLevel(levelStr string) {
	var logLevel slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	default:
		logLevel = slog.LevelError
	}
	newLogger := logging.NewFileLoggerFromEnv("genie-debug.log")
	if levelUpdater, ok := newLogger.(interface{ SetLevel(slog.Level) }); ok {
		levelUpdater.SetLevel(logLevel)
	}
	logging.SetGlobalLogger(newLogger)
}

// LoadDebugLog replaces the debug state with the debug log file's lines,
// or with a hint when the file cannot be read.
func LoadDebugLog(st *state.DebugState) {
	debugFile := logging.GetDebugFilePath("genie-debug.log")
	st.ClearDebugMessages()
	content, err := os.ReadFile(debugFile)
	if err != nil {
		st.AddDebugMessage(fmt.Sprintf("Debug file not found or unreadable: %s", debugFile))
		st.AddDebugMessage("Use :debug command to enable debug logging")
		return
	}
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) != "" {
			st.AddDebugMessage(line)
		}
	}
}

// LLMContextPanel opens the context viewer dialog with the parts the core
// would send to the model.
type LLMContextPanel struct {
	ui    types.Dispatcher
	genie genie.Genie
	state types.IStateAccessor
}

// NewLLMContextPanel builds the controller and subscribes to the
// context.refresh topic the open viewer emits.
func NewLLMContextPanel(ui types.Dispatcher, g genie.Genie, st types.IStateAccessor, bus *events.CommandEventBus) *LLMContextPanel {
	c := &LLMContextPanel{ui: ui, genie: g, state: st}
	bus.Subscribe(uimsg.TopicContextRefresh, func(interface{}) {
		if data, err := c.load(); err == nil {
			c.ui.Dispatch(uimsg.ContextData{Data: data})
		}
	})
	return c
}

// Show opens the viewer, or closes it if it is already open.
func (c *LLMContextPanel) Show() error {
	if c.state.IsContextViewerActive() {
		c.ui.Dispatch(uimsg.CloseContextViewer{})
		return nil
	}
	data, err := c.load()
	if err != nil {
		return fmt.Errorf("failed to load context data: %w", err)
	}
	c.ui.Dispatch(uimsg.OpenContextViewer{Data: data})
	return nil
}

func (c *LLMContextPanel) load() (map[string]string, error) {
	return c.genie.GetContext(context.Background())
}

// WritePanel opens write mode for the :write command.
type WritePanel struct{ ui types.Dispatcher }

// NewWritePanel builds the controller.
func NewWritePanel(ui types.Dispatcher) *WritePanel { return &WritePanel{ui: ui} }

// Show opens an empty composer.
func (w *WritePanel) Show() error {
	w.ui.Dispatch(uimsg.OpenWrite{})
	return nil
}
