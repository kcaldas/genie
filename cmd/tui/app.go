package tui

import (
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/slashcommands"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/controllers"
	"github.com/kcaldas/genie/cmd/tui/controllers/commands"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/logging"
)

type App struct {
	// Core dependencies
	gui           types.Gui
	configManager *helpers.ConfigManager
	notification  types.Notification

	// Event handling
	commandEventBus *events.CommandEventBus

	// State management
	uiState *state.UIState

	// Layout and UI
	layoutManager *layout.LayoutManager

	// Controllers and commands
	helpController *controllers.HelpController
	commandHandler *commands.CommandHandler

	// Input handling
	keymap                   *Keymap
	globalKeybindingsEnabled bool // Track global keybinding state

	// Help system
	helpRenderer HelpRenderer

	// Internal state
	keybindingsSetup bool
}

func NewApp(
	gui types.Gui,
	commandEventBus *events.CommandEventBus,
	configManager *helpers.ConfigManager,
	layoutManager *layout.LayoutManager,
	commandHandler *commands.CommandHandler,
	notification types.Notification,
	uiState *state.UIState,
	confirmationInit *ConfirmationInitializer,
	slashCommandManager *slashcommands.Manager,
) (*App, error) {
	return NewAppWithOutputMode(
		gui,
		commandEventBus,
		configManager,
		layoutManager,
		commandHandler,
		notification,
		uiState,
		confirmationInit,
		slashCommandManager,
		nil,
	)
}

func NewAppWithOutputMode(
	gui types.Gui,
	commandEventBus *events.CommandEventBus,
	configManager *helpers.ConfigManager,
	layoutManager *layout.LayoutManager,
	commandHandler *commands.CommandHandler,
	notification types.Notification,
	uiState *state.UIState,
	confirmationInit *ConfirmationInitializer,
	slashCommandManager *slashcommands.Manager,
	outputMode *gocui.OutputMode,
) (*App, error) {
	// Disable standard Go logging to prevent interference with TUI
	log.SetOutput(io.Discard)

	// Configure the global slog-based logger using the centralized file logger
	tuiLogger := logging.NewFileLoggerFromEnv("genie-debug.log")
	logging.SetGlobalLogger(tuiLogger)

	// Get config from the injected ConfigManager
	config := configManager.GetConfig()
	
	// Debug: Print mouse configuration after logger is set up
	logger := logging.GetGlobalLogger()
	logger.Info("DEBUG: Mouse config loaded - EnableMouse = %v", config.EnableMouse)
	logger.Info("DEBUG: GUI Mouse setting = %v", gui.GetGui().Mouse)
	
	// Debug: Read local config file directly to see raw JSON
	if data, err := os.ReadFile(".genie/settings.tui.json"); err == nil {
		logger.Info("DEBUG: Raw local config file content: %s", string(data))
	} else {
		logger.Info("DEBUG: Could not read local config: %v", err)
	}

	app := &App{
		gui:                      gui,
		configManager:            configManager,
		notification:             notification,
		layoutManager:            layoutManager,
		commandEventBus:          commandEventBus,
		commandHandler:           commandHandler,
		uiState:                  uiState,
		globalKeybindingsEnabled: true, // Start with global keybindings enabled
	}

	// Initialize keymap after components are set up
	app.keymap = app.createKeymap()

	// Create help renderer after command handler and keymap are initialized
	app.helpRenderer = NewManPageHelpRenderer(app.commandHandler.GetRegistry(), app.keymap, slashCommandManager)

	textViewer := app.layoutManager.GetComponent("text-viewer").(*component.TextViewerComponent)

	// Create help controller after help renderer is available
	app.helpController = controllers.NewHelpController(
		app.gui,
		app.layoutManager,
		textViewer,
		app.helpRenderer,
		app.configManager,
	)

	// Register help command (only one left to register manually)
	app.commandHandler.RegisterNewCommand(commands.NewHelpCommand(app.helpController))

	app.commandEventBus.Subscribe("app.exit", func(i interface{}) {
		app.exit()
	})

	// Subscribe to global keybinding control events
	app.commandEventBus.Subscribe("keybindings.disable.global", func(i interface{}) {
		// Use gui.Update to ensure thread safety
		gui.GetGui().Update(func(g *gocui.Gui) error {
			return app.DisableGlobalKeybindings()
		})
	})

	app.commandEventBus.Subscribe("keybindings.enable.global", func(i interface{}) {
		// Use gui.Update to ensure thread safety
		gui.GetGui().Update(func(g *gocui.Gui) error {
			return app.EnableGlobalKeybindings()
		})
	})

	gui.GetGui().Cursor = true // Force cursor enabled for debugging

	theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)

	// Set global frame colors from theme as fallback
	if theme != nil {
		gui.GetGui().FrameColor = presentation.ConvertAnsiToGocuiColor(theme.BorderDefault)
		gui.GetGui().SelFrameColor = presentation.ConvertAnsiToGocuiColor(theme.BorderFocused)
	}

	// Set the layout manager function with keybinding setup
	gui.GetGui().SetManagerFunc(func(gui *gocui.Gui) error {
		// First run the layout manager
		if err := app.layoutManager.Layout(gui); err != nil {
			return err
		}
		// Set up keybindings after views are created (only once)
		if !app.keybindingsSetup {
			if err := app.setupKeybindings(); err != nil {
				return err
			}
			// Reset keybindings for all panels to ensure they're properly registered
			if err := app.layoutManager.ResetKeybindings(); err != nil {
				return err
			}
			app.keybindingsSetup = true
		}
		return nil
	})

	return app, nil
}

func (app *App) createKeymap() *Keymap {
	keymap := NewKeymap()

	// Event-driven shortcuts - these emit events for consistent behavior
	keymap.AddEntry(KeymapEntry{
		Key: gocui.KeyF1,
		Mod: gocui.ModNone,
		Action: FunctionAction(func() error {
			app.commandEventBus.Emit("shortcut.help", nil)
			return nil
		}),
		Description: "Show help dialog",
	})

	keymap.AddEntry(KeymapEntry{
		Key: gocui.KeyF12,
		Mod: gocui.ModNone,
		Action: FunctionAction(func() error {
			app.commandEventBus.Emit("debug.view", nil)
			return nil
		}),
		Description: "Toggle debug panel visibility",
	})


	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyF4,
		Mod:         gocui.ModNone,
		Action:      CommandAction("write"),
		Description: "Open text area zoom for composing longer messages",
	})
	keymap.AddEntry(KeymapEntry{
		Key: gocui.KeyCtrlZ,
		Mod: gocui.ModNone,
		Action: FunctionAction(func() error {
			app.layoutManager.ToggleRightPanelZoom()
			return nil
		}),
		Description: "Toggle right panel zoom (confirmations, debug, etc.)",
	})

	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyCtrlSlash,
		Mod:         gocui.ModNone,
		Action:      CommandAction("context"),
		Description: "Show LLM context viewer",
	})

	// Direct action shortcuts - these call App methods directly
	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyPgup,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.PageUp),
		Description: "Scroll messages up",
	})
	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyCtrlU,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.PageUp),
		Description: "Scroll messages up",
	})

	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyPgdn,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.PageDown),
		Description: "Scroll messages down",
	})
	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyCtrlD,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.PageDown),
		Description: "Scroll messages down",
	})

	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyArrowUp,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.ScrollUp),
		Description: "Scroll messages up",
	})
	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyArrowDown,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.ScrollDown),
		Description: "Scroll messages down",
	})
	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyHome,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.ScrollToTop),
		Description: "Scroll to top of messages",
	})

	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyEnd,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.ScrollToBottom),
		Description: "Scroll to bottom of messages",
	})

	keymap.AddEntry(KeymapEntry{
		Key:         gocui.MouseWheelUp,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.handleMouseWheelUp),
		Description: "Scroll messages up (mouse)",
	})

	keymap.AddEntry(KeymapEntry{
		Key:         gocui.MouseWheelDown,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.handleMouseWheelDown),
		Description: "Scroll messages down (mouse)",
	})

	keymap.AddEntry(KeymapEntry{
		Key: gocui.KeyCtrlC,
		Mod: gocui.ModNone,
		Action: FunctionAction(func() error {
			return gocui.ErrQuit
		}),
		Description: "Exit application",
	})

	keymap.AddEntry(KeymapEntry{
		Key: gocui.KeyTab,
		Mod: gocui.ModNone,
		Action: FunctionAction(func() error {
			app.layoutManager.FocusNextPanel()
			return nil
		}),
		Description: "Focus on next panel",
	})
	return keymap
}

func (app *App) createKeymapHandler(entry KeymapEntry) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		switch entry.Action.Type {
		case "command":
			if cmd := app.commandHandler.GetCommand(entry.Action.CommandName); cmd != nil {
				return cmd.Execute([]string{})
			}
		case "function":
			return entry.Action.Function()
		}
		return nil
	}
}

func (app *App) setupKeybindings() error {
	// Setup keymap-driven global keybindings
	for _, entry := range app.keymap.GetEntries() {
		handler := app.createKeymapHandler(entry)
		if err := app.gui.GetGui().SetKeybinding("", entry.Key, entry.Mod, handler); err != nil {
			return err
		}
	}
	return nil
}

func (app *App) Run() error {
	return app.RunWithMessage("")
}

func (app *App) RunWithMessage(initialMessage string) error {
	// Add welcome message first
	app.notification.AddSystemMessage("Welcome to Genie! Type :? for help.")

	// Set focus to input after everything is set up using semantic naming
	app.gui.GetGui().Update(func(g *gocui.Gui) error {
		return app.layoutManager.FocusPanel("input") // Use semantic name directly
	})

	// If we have an initial message, send it after a short delay to ensure TUI is ready
	if initialMessage != "" {
		go func() {
			// Small delay to ensure TUI is fully initialized
			time.Sleep(100 * time.Millisecond)
			app.commandEventBus.Emit("user.input.text", initialMessage)
		}()
	}

	// Setup signal handling for graceful shutdown on Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		// Gracefully exit the application when receiving SIGINT (Ctrl+C) or SIGTERM
		app.gui.GetGui().Close()
	}()

	return app.gui.GetGui().MainLoop()
}

func (app *App) Close() {
	if app.gui.GetGui() != nil {
		app.gui.GetGui().Close()
	}
}

// GetGui for testing.
func (app *App) GetGui() *gocui.Gui {
	return app.gui.GetGui()
}

// getActiveScrollable returns the currently active scrollable component
func (app *App) getActiveScrollable() component.Scrollable {
	var c types.Component
	// Check if right panel is visible and return appropriate component
	if app.layoutManager.IsRightPanelVisible() {
		switch app.layoutManager.GetRightPanelMode() {
		case "text-viewer":
			c = app.layoutManager.GetComponent("text-viewer")
		case "diff-viewer":
			c = app.layoutManager.GetComponent("diff-viewer")
		}
	} else {
		// Default to messages component
		c = app.layoutManager.GetComponent("messages")
	}
	return c.(component.Scrollable)
}

func (app *App) ScrollUp() error {
	return app.getActiveScrollable().ScrollUp()
}

func (app *App) ScrollDown() error {
	return app.getActiveScrollable().ScrollDown()
}

// DisableGlobalKeybindings removes global keybindings (those with empty view name "")
func (app *App) DisableGlobalKeybindings() error {
	if !app.globalKeybindingsEnabled {
		return nil // Already disabled
	}

	gui := app.gui.GetGui()
	if gui == nil {
		return nil
	}

	// Delete all global keybindings (empty view name)
	gui.DeleteKeybindings("")
	app.globalKeybindingsEnabled = false
	return nil
}

// EnableGlobalKeybindings restores global keybindings
func (app *App) EnableGlobalKeybindings() error {
	if app.globalKeybindingsEnabled {
		return nil // Already enabled
	}

	gui := app.gui.GetGui()
	if gui == nil {
		return nil
	}

	// Re-setup global keybindings
	for _, entry := range app.keymap.GetEntries() {
		handler := app.createKeymapHandler(entry)
		if err := gui.SetKeybinding("", entry.Key, entry.Mod, handler); err != nil {
			return err
		}
	}
	app.globalKeybindingsEnabled = true
	return nil
}

func (app *App) PageUp() error {
	return app.getActiveScrollable().PageUp()
}

func (app *App) PageDown() error {
	return app.getActiveScrollable().PageDown()
}

func (app *App) ScrollToTop() error {
	return app.getActiveScrollable().ScrollToTop()
}

func (app *App) ScrollToBottom() error {
	return app.getActiveScrollable().ScrollToBottom()
}

// Mouse wheel handlers for keymap
func (app *App) handleMouseWheelUp() error {
	// Scroll up by 3 lines for smooth scrolling
	for range 3 {
		app.ScrollUp()
	}
	return nil
}

func (app *App) handleMouseWheelDown() error {
	// Scroll down by 3 lines for smooth scrolling
	for range 3 {
		app.ScrollDown()
	}
	return nil
}


func (app *App) exit() error {
	// Set a flag to exit the main loop
	app.gui.GetGui().Update(func(g *gocui.Gui) error {
		return gocui.ErrQuit
	})
	return nil
}
