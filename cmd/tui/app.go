package tui

import (
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/controllers"
	"github.com/kcaldas/genie/cmd/tui/controllers/commands"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
)

type App struct {
	gui           *gocui.Gui
	genie         genie.Genie
	session       *genie.Session
	configManager *helpers.ConfigManager
	clipboard     *helpers.Clipboard

	// Event bus for command-level communication
	commandEventBus *events.CommandEventBus

	chatState     *state.ChatState
	uiState       *state.UIState
	debugState    *state.DebugState
	stateAccessor *state.StateAccessor

	layoutManager *layout.LayoutManager

	messagesComponent    *component.MessagesComponent
	inputComponent       *component.InputComponent
	debugComponent       *component.DebugComponent
	statusComponent      *component.StatusComponent
	textViewerComponent  *component.TextViewerComponent
	diffViewerComponent  *component.DiffViewerComponent
	llmContextController *controllers.LLMContextController

	chatController  *controllers.ChatController
	debugController *controllers.DebugController
	helpController  *controllers.HelpController
	commandHandler  *commands.CommandHandler

	// Confirmation controllers
	toolConfirmationController *controllers.ToolConfirmationController
	userConfirmationController *controllers.UserConfirmationController

	// Keymap for centralized keybinding management
	keymap *Keymap

	// Help renderer for unified documentation
	helpRenderer HelpRenderer

	keybindingsSetup bool // Track if keybindings have been set up

	// Note: Auto-scroll removed for now - always scroll to bottom after messages
}

func NewApp(gui *gocui.Gui, genieService genie.Genie, session *genie.Session, commandEventBus *events.CommandEventBus, configManager *helpers.ConfigManager, layoutManager *layout.LayoutManager) (*App, error) {
	return NewAppWithOutputMode(gui, genieService, session, commandEventBus, configManager, layoutManager, nil)
}

func NewAppWithOutputMode(gui *gocui.Gui, genieService genie.Genie, session *genie.Session, commandEventBus *events.CommandEventBus, configManager *helpers.ConfigManager, layoutManager *layout.LayoutManager, outputMode *gocui.OutputMode) (*App, error) {
	// Disable standard Go logging to prevent interference with TUI
	log.SetOutput(io.Discard)

	// Configure the global slog-based logger to discard output during TUI operation
	quietLogger := logging.NewLogger(logging.Config{
		Level:   slog.LevelError, // Only show errors, but to discard
		Format:  logging.FormatText,
		Output:  io.Discard, // Discard all output
		AddTime: false,
	})
	logging.SetGlobalLogger(quietLogger)

	// Initialize clipboard helper
	clipboard := ProvideClipboard()

	// Get config from the injected ConfigManager
	config := configManager.GetConfig()

	app := &App{
		gui:             gui,
		genie:           genieService,
		session:         session,
		clipboard:       clipboard,
		configManager:   configManager,
		layoutManager:   layoutManager,
		commandEventBus: commandEventBus,
	}

	app.chatState = ProvideChatState(configManager)
	app.uiState = ProvideUIState()
	app.debugState = ProvideDebugState()
	app.stateAccessor = ProvideStateAccessor(app.chatState, app.uiState)

	// Initialize keymap after components are set up
	app.keymap = app.createKeymap()

	if err := app.setupComponentsAndControllers(gui); err != nil {
		gui.Close()
		return nil, err
	}

	// Create command handler after controllers are created (needs chatController for notifications)
	app.commandHandler = commands.NewCommandHandler(app.commandEventBus, app.chatController)

	// Create help renderer after command handler and keymap are initialized
	app.helpRenderer = NewManPageHelpRenderer(app.commandHandler.GetRegistry(), app.keymap)

	// Create help controller after help renderer is available
	app.helpController = controllers.NewHelpController(
		app,
		app.layoutManager,
		app.textViewerComponent,
		app.helpRenderer,
		app.configManager,
	)

	// Register new command types (must be after all controllers are created)
	app.commandHandler.RegisterNewCommand(commands.NewHelpCommand(app.helpController))
	app.commandHandler.RegisterNewCommand(commands.NewContextCommand(app.llmContextController))
	app.commandHandler.RegisterNewCommand(commands.NewClearCommand(app.chatController))
	app.commandHandler.RegisterNewCommand(commands.NewDebugCommand(app.debugController, app.debugController, app.chatController))
	app.commandHandler.RegisterNewCommand(commands.NewExitCommand(app.commandEventBus))
	app.commandHandler.RegisterNewCommand(commands.NewYankCommand(app.chatState, app.clipboard, app.chatController))
	app.commandHandler.RegisterNewCommand(commands.NewThemeCommand(app.configManager, app.commandEventBus, app.chatController))
	app.commandHandler.RegisterNewCommand(commands.NewConfigCommand(app.configManager, app.commandEventBus, app, app.chatController, app.debugController))

	app.commandEventBus.Subscribe("app.exit", func(i interface{}) {
		app.exit()
	})

	gui.Cursor = true // Force cursor enabled for debugging

	theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)

	// Set global frame colors from theme as fallback
	if theme != nil {
		gui.FrameColor = presentation.ConvertAnsiToGocuiColor(theme.BorderDefault)
		gui.SelFrameColor = presentation.ConvertAnsiToGocuiColor(theme.BorderFocused)
	}

	// Set the layout manager function with keybinding setup
	gui.SetManagerFunc(func(gui *gocui.Gui) error {
		// First run the layout manager
		if err := app.layoutManager.Layout(gui); err != nil {
			return err
		}
		// Set up keybindings after views are created (only once)
		if !app.keybindingsSetup {
			if err := app.setupKeybindings(); err != nil {
				return err
			}
			app.keybindingsSetup = true
		}
		return nil
	})

	return app, nil
}

func (app *App) setupComponentsAndControllers(gui *gocui.Gui) error {
	guiCommon := ProvideGui(gui)

	// Create history path in WorkingDirectory/.genie/
	historyPath := string(ProvideHistoryPath(app.session))

	// Create debug component first
	app.debugComponent = component.NewDebugComponent(guiCommon, app.debugState, app.configManager, app.commandEventBus)
	app.messagesComponent = component.NewMessagesComponent(guiCommon, app.chatState, app.configManager, app.commandEventBus)
	app.inputComponent = component.NewInputComponent(guiCommon, app.configManager, app.commandEventBus, historyPath)
	app.statusComponent = component.NewStatusComponent(guiCommon, app.stateAccessor, app.configManager, app.commandEventBus)
	app.textViewerComponent = component.NewTextViewerComponent(guiCommon, "Help", app.configManager, app.commandEventBus)
	app.diffViewerComponent = component.NewDiffViewerComponent(guiCommon, "Diff", app.configManager, app.commandEventBus)

	// Create and configure layout using wireable LayoutBuilder
	layoutBuilder := ProvideLayoutBuilder(
		gui,
		app.configManager,
		app.messagesComponent,
		app.inputComponent,
		app.statusComponent,
		app.textViewerComponent,
		app.diffViewerComponent,
		app.debugComponent,
	)

	// Get the configured layout manager
	app.layoutManager = ProvideLayoutManager(layoutBuilder)

	// Initialize debug controller with the component
	app.debugController = controllers.NewDebugController(
		app.genie,
		guiCommon,
		app.debugState,
		app.debugComponent,
		app.layoutManager,
		app.clipboard,
		app.configManager,
		app.commandEventBus,
	)

	app.chatController = controllers.NewChatController(
		app.messagesComponent,
		guiCommon,
		app.genie,
		app.stateAccessor,
		app.configManager,
		app.commandEventBus,
		app.debugController, // Pass logger
	)

	// Create LLM context controller after debug controller
	app.llmContextController = controllers.NewLLMContextController(guiCommon, app.genie, app.layoutManager, app.stateAccessor, app.configManager, app.commandEventBus, app.debugController)

	// Initialize confirmation controllers
	eventBus := app.genie.GetEventBus()
	app.toolConfirmationController = controllers.NewToolConfirmationController(
		guiCommon,
		app.stateAccessor,
		app.layoutManager,
		app.inputComponent,
		app.configManager,
		eventBus,
		app.commandEventBus,
		app.debugController, // Pass logger
	)

	app.userConfirmationController = controllers.NewUserConfirmationController(
		guiCommon,
		app.stateAccessor,
		app.layoutManager,
		app.inputComponent,
		app.diffViewerComponent,
		app.configManager,
		eventBus,
		app.commandEventBus,
		app.debugController, // Pass logger
	)

	return nil
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
		// Special handling for Esc and q when context viewer is active
		if app.uiState.IsContextViewerActive() && (entry.Key == gocui.KeyEsc || entry.Key == 'q') {
			return app.llmContextController.Close()
		}

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
		if err := app.gui.SetKeybinding("", entry.Key, entry.Mod, handler); err != nil {
			return err
		}
	}
	return nil
}

func (app *App) Run() error {
	// Add welcome message first
	app.chatController.AddSystemMessage("Welcome to Genie! Type :? for help.")

	// Set focus to input after everything is set up using semantic naming
	app.gui.Update(func(g *gocui.Gui) error {
		return app.focusPanelByName("input") // Use semantic name directly
	})

	// Start periodic status updates
	app.statusComponent.StartStatusUpdates()

	// Setup signal handling for graceful shutdown on Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		// Gracefully exit the application when receiving SIGINT (Ctrl+C) or SIGTERM
		app.gui.Close()
	}()

	return app.gui.MainLoop()
}

func (app *App) Close() {
	if app.gui != nil {
		app.gui.Close()
	}
}

func (app *App) nextView(g *gocui.Gui, v *gocui.View) error {
	// Delegate to layout manager for panel navigation
	app.layoutManager.FocusNextPanel()
	return nil
}

func (app *App) focusPanelByName(panelName string) error {
	// Delegate to layout manager for panel focusing
	if err := app.layoutManager.FocusPanel(panelName); err != nil {
		return err
	}
	return nil
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

// IGuiCommon interface implementation
func (app *App) GetGui() *gocui.Gui {
	return app.gui
}

func (app *App) PostUIUpdate(fn func()) {
	app.gui.Update(func(g *gocui.Gui) error {
		fn()
		return nil
	})
}

func (app *App) exit() error {
	// Set a flag to exit the main loop
	app.gui.Update(func(g *gocui.Gui) error {
		return gocui.ErrQuit
	})
	return nil
}
