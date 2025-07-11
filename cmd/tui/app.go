package tui

import (
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
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
	gui     *gocui.Gui
	genie   genie.Genie
	session *genie.Session
	helpers *helpers.Helpers

	// Event bus for command-level communication
	commandEventBus *events.CommandEventBus

	chatState     *state.ChatState
	uiState       *state.UIState
	debugState    *state.DebugState
	stateAccessor *state.StateAccessor

	todoFormatter *presentation.TodoFormatter
	layoutManager *layout.LayoutManager

	messagesComponent    *component.MessagesComponent
	inputComponent       *component.InputComponent
	debugComponent       *component.DebugComponent
	statusComponent      *component.StatusComponent
	textViewerComponent  *component.TextViewerComponent
	diffViewerComponent  *component.DiffViewerComponent
	llmContextController controllers.LLMContextControllerInterface

	chatController  *controllers.ChatController
	debugController *controllers.DebugController
	helpController  *controllers.HelpController
	commandHandler  *commands.CommandHandler

	// Confirmation controllers
	toolConfirmationController *controllers.ToolConfirmationController
	userConfirmationController *controllers.UserConfirmationController

	// Track which type of confirmation is currently active
	activeConfirmationType string // "tool" or "user" or ""

	// Dialog management
	currentDialog types.Component

	// Keymap for centralized keybinding management
	keymap *Keymap

	// Help renderer for unified documentation
	helpRenderer HelpRenderer

	keybindingsSetup bool // Track if keybindings have been set up

	// Context viewer state
	contextViewerActive bool

	// Note: Auto-scroll removed for now - always scroll to bottom after messages
}

func NewApp(genieService genie.Genie, session *genie.Session, commandEventBus *events.CommandEventBus) (*App, error) {
	return NewAppWithOutputMode(genieService, session, commandEventBus, nil)
}

func NewAppWithOutputMode(genieService genie.Genie, session *genie.Session, commandEventBus *events.CommandEventBus, outputMode *gocui.OutputMode) (*App, error) {
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

	// Initialize helpers first to load configuration
	helpers, err := helpers.NewHelpers()
	if err != nil {
		return nil, err
	}

	config, err := helpers.Config.Load()
	if err != nil {
		return nil, err
	}

	// Create gocui instance with configured output mode
	var guiOutputMode gocui.OutputMode
	if outputMode != nil {
		guiOutputMode = *outputMode
	} else {
		guiOutputMode = helpers.Config.GetGocuiOutputMode(config.OutputMode)
	}
	g, err := gocui.NewGui(guiOutputMode, true)
	if err != nil {
		return nil, err
	}

	app := &App{
		gui:             g,
		genie:           genieService,
		session:         session,
		helpers:         helpers,
		commandEventBus: commandEventBus,
		chatState:       state.NewChatState(config.MaxChatMessages),
		uiState:         state.NewUIState(config),
		debugState:      state.NewDebugState(),
	}

	app.stateAccessor = state.NewStateAccessor(app.chatState, app.uiState)

	layoutConfig := &layout.LayoutConfig{
		MessagesWeight: 3,                           // Messages panel weight (main content)
		InputHeight:    4,                           // Input panel height
		DebugWeight:    1,                           // Debug panel weight when shown
		StatusHeight:   2,                           // Status bar height
		ShowSidebar:    config.Layout.ShowSidebar,   // Keep legacy field
		CompactMode:    config.Layout.CompactMode,   // Keep compact mode
		MinPanelWidth:  config.Layout.MinPanelWidth, // Keep minimum constraints
		MinPanelHeight: config.Layout.MinPanelHeight,
	}
	app.layoutManager = layout.NewLayoutManager(g, layoutConfig)

	theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)

	app.todoFormatter = presentation.NewTodoFormatter(theme)

	if err := app.setupComponentsAndControllers(); err != nil {
		g.Close()
		return nil, err
	}

	// Initialize keymap after components are set up
	app.keymap = app.createKeymap()

	// Create command handler first (needed by help renderer)
	ctx := &commands.CommandContext{
		GuiCommon:            app,
		ClipboardHelper:      app.helpers.Clipboard,
		ConfigHelper:         app.helpers.Config,
		ShowLLMContextViewer: app.showLLMContextViewer,
		Notification:         app.chatController,
		Exit:                 app.exit,
		CommandEventBus:      app.commandEventBus,
		Logger:               app.debugController,
	}
	app.commandHandler = commands.NewCommandHandler(ctx, app.commandEventBus)

	// Create help renderer after command handler and keymap are initialized
	app.helpRenderer = NewManPageHelpRenderer(app.commandHandler.GetRegistry(), app.keymap)

	// Create help controller after help renderer is available
	app.helpController = controllers.NewHelpController(
		app,
		app.layoutManager,
		app.textViewerComponent,
		app.helpRenderer,
	)

	// Register new command types
	app.commandHandler.RegisterNewCommand(commands.NewHelpCommand(ctx, app.helpController))
	app.commandHandler.RegisterNewCommand(commands.NewContextCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewClearCommand(ctx, app.chatController))
	app.commandHandler.RegisterNewCommand(commands.NewDebugCommand(ctx, app.debugController))
	app.commandHandler.RegisterNewCommand(commands.NewExitCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewYankCommand(ctx, app.chatState))
	app.commandHandler.RegisterNewCommand(commands.NewThemeCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewConfigCommand(ctx))

	// Subscribe to theme changes for app-level updates
	app.commandEventBus.Subscribe("theme.changed", func(event interface{}) {
		if eventData, ok := event.(map[string]interface{}); ok {
			if config, ok := eventData["config"].(*types.Config); ok {
				// Update todoFormatter with new theme
				theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
				app.todoFormatter = presentation.NewTodoFormatter(theme)
			}
		}
	})

	g.Cursor = true // Force cursor enabled for debugging

	// Set global frame colors from theme as fallback
	if theme != nil {
		g.FrameColor = presentation.ConvertAnsiToGocuiColor(theme.BorderDefault)
		g.SelFrameColor = presentation.ConvertAnsiToGocuiColor(theme.BorderFocused)
	}

	// Set the layout manager function with keybinding setup
	g.SetManagerFunc(func(gui *gocui.Gui) error {
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

func (app *App) setupComponentsAndControllers() error {
	guiCommon := &guiCommon{app: app}

	// Create history path in WorkingDirectory/.genie/
	historyPath := filepath.Join(app.session.WorkingDirectory, ".genie", "history")

	app.messagesComponent = component.NewMessagesComponent(guiCommon, app.chatState, app.commandEventBus)
	app.inputComponent = component.NewInputComponent(guiCommon, app.commandEventBus, historyPath)
	app.statusComponent = component.NewStatusComponent(guiCommon, app.stateAccessor, app.commandEventBus)
	app.textViewerComponent = component.NewTextViewerComponent(guiCommon, "Help", app.commandEventBus)
	app.diffViewerComponent = component.NewDiffViewerComponent(guiCommon, "Diff", app.commandEventBus)

	// Map components using semantic names (debug component mapped later)
	app.layoutManager.SetComponent("messages", app.messagesComponent)      // messages in center
	app.layoutManager.SetComponent("input", app.inputComponent)            // input at bottom
	app.layoutManager.SetComponent("text-viewer", app.textViewerComponent) // text viewer on right side
	app.layoutManager.SetComponent("diff-viewer", app.diffViewerComponent) // diff viewer on right side
	app.layoutManager.SetComponent("status", app.statusComponent)          // status at top

	// Register status sub-components
	app.layoutManager.AddSubPanel("status", "status-left", app.statusComponent.GetLeftComponent())
	app.layoutManager.AddSubPanel("status", "status-center", app.statusComponent.GetCenterComponent())
	app.layoutManager.AddSubPanel("status", "status-right", app.statusComponent.GetRightComponent())

	// Create debug component first
	app.debugComponent = component.NewDebugComponent(guiCommon, app.debugState, app.commandEventBus)

	// Initialize debug controller with the component
	app.debugController = controllers.NewDebugController(
		app.genie,
		guiCommon,
		app.debugState,
		app.debugComponent,
		app.layoutManager,
		app.helpers,
		app.commandEventBus,
	)

	// Set initial debug mode from config
	app.debugController.SetDebugMode(app.uiState.GetConfig().DebugEnabled)

	app.chatController = controllers.NewChatController(
		app.messagesComponent,
		guiCommon,
		app.genie,
		app.stateAccessor,
		app.commandEventBus,
		app.debugController, // Pass logger
	)

	// Create LLM context controller after debug controller
	app.llmContextController = controllers.NewLLMContextController(guiCommon, app.genie, app.stateAccessor, app.commandEventBus, app.debugController, func() error {
		app.currentDialog = nil
		app.contextViewerActive = false // Clear the flag
		// Restore focus to input component
		return app.focusPanelByName("input")
	})

	// Initialize confirmation controllers
	eventBus := app.genie.GetEventBus()
	app.toolConfirmationController = controllers.NewToolConfirmationController(
		guiCommon,
		app.stateAccessor,
		app.layoutManager,
		app.inputComponent,
		eventBus,
		app.debugController, // Pass logger
		func(confirmationType string) { app.activeConfirmationType = confirmationType },
	)

	app.userConfirmationController = controllers.NewUserConfirmationController(
		guiCommon,
		app.stateAccessor,
		app.layoutManager,
		app.inputComponent,
		app.diffViewerComponent,
		eventBus,
		app.debugController, // Pass logger
		func(confirmationType string) { app.activeConfirmationType = confirmationType },
	)

	// Now that all components are created, set up tab handlers and complete layout mapping
	app.inputComponent.SetTabHandler(app.nextView)
	app.messagesComponent.SetTabHandler(app.nextView)
	app.debugComponent.SetTabHandler(app.nextView)
	app.textViewerComponent.SetTabHandler(app.nextView)
	app.diffViewerComponent.SetTabHandler(app.nextView)

	// Map debug component to layout now that it's created
	app.layoutManager.SetComponent("debug", app.debugComponent)

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
		Key:         gocui.KeyEsc,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.handleEscKey),
		Description: "Cancel current operation",
	})

	return keymap
}

func (app *App) createKeymapHandler(entry KeymapEntry) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		// Special handling for Esc and q when context viewer is active
		if app.contextViewerActive && (entry.Key == gocui.KeyEsc || entry.Key == 'q') {
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
	// Setup keybindings for all components
	// Note: Confirmation components are now managed by separate controllers
	components := []types.Component{app.messagesComponent, app.inputComponent, app.debugComponent, app.statusComponent, app.textViewerComponent, app.diffViewerComponent}

	for _, ctx := range components {
		for _, kb := range ctx.GetKeybindings() {
			if err := app.gui.SetKeybinding(kb.View, kb.Key, kb.Mod, kb.Handler); err != nil {
				return err
			}
		}
	}

	// Setup keymap-driven global keybindings
	for _, entry := range app.keymap.GetEntries() {
		handler := app.createKeymapHandler(entry)
		if err := app.gui.SetKeybinding("", entry.Key, entry.Mod, handler); err != nil {
			return err
		}
	}

	// Setup global confirmation keybindings that route to the appropriate controller
	confirmationKeys := []interface{}{'1', '2', 'y', 'Y', 'n', 'N', gocui.KeyEsc}
	for _, key := range confirmationKeys {
		capturedKey := key // Capture the key for the closure
		if err := app.gui.SetKeybinding("input", capturedKey, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			return app.handleConfirmationKey(capturedKey)
		}); err != nil {
			return err
		}
	}

	return nil
}

func (app *App) handleConfirmationKey(key interface{}) error {
	// Route confirmation keys based on the active confirmation type
	switch app.activeConfirmationType {
	case "tool":
		return app.handleToolConfirmationKey(key)
	case "user":
		return app.handleUserConfirmationKey(key)
	default:
		// No active confirmation - check if it's ESC for chat cancellation
		if key == gocui.KeyEsc {
			return app.handleEscKey()
		}
		return nil
	}
}

func (app *App) handleToolConfirmationKey(key interface{}) error {
	handled, err := app.toolConfirmationController.HandleKeyPress(key)
	if handled {
		// Clear the active confirmation type
		app.activeConfirmationType = ""
	}
	return err
}

func (app *App) handleUserConfirmationKey(key interface{}) error {
	handled, err := app.userConfirmationController.HandleKeyPress(key)
	if handled {
		// Clear the active confirmation type
		app.activeConfirmationType = ""
	}
	return err
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
	focusedPanelName := app.layoutManager.FocusNextPanel()
	if focusedPanelName != "" {
		// Update UI state to track the focused panel
		app.uiState.SetFocusedPanel(focusedPanelName)
	}
	return nil
}

func (app *App) focusPanelByName(panelName string) error {
	// Delegate to layout manager for panel focusing
	if err := app.layoutManager.FocusPanel(panelName); err != nil {
		return err
	}

	// Update UI state to track the focused panel
	app.uiState.SetFocusedPanel(panelName)
	return nil
}

func (app *App) handleEscKey() error {
	// First check if context viewer is active
	if app.contextViewerActive {
		return app.llmContextController.Close()
	}

	// Cancel the chat
	app.chatController.CancelChat()

	return nil
}

// getActiveScrollable returns the currently active scrollable component
func (app *App) getActiveScrollable() component.Scrollable {
	// Check if right panel is visible and return appropriate component
	if app.layoutManager.IsRightPanelVisible() {
		switch app.layoutManager.GetRightPanelMode() {
		case "text-viewer":
			return app.textViewerComponent
		case "diff-viewer":
			return app.diffViewerComponent
		}
	}

	// Default to messages component
	return app.messagesComponent
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

// Dialog management methods
func (app *App) showLLMContextViewer() error {
	// Toggle behavior - close if already open
	if app.contextViewerActive {
		return app.llmContextController.Close()
	}

	// Close any existing dialog first
	if err := app.closeCurrentDialog(); err != nil {
		return err
	}

	// Show the context viewer through the controller
	if err := app.llmContextController.Show(); err != nil {
		return err
	}

	// Controller manages its own component's keybindings and rendering
	// We just need to mark it as active
	app.contextViewerActive = true

	return nil
}

func (app *App) closeCurrentDialog() error {
	if app.currentDialog == nil {
		return nil
	}

	// Store reference and clear current dialog first to prevent reentrancy
	dialog := app.currentDialog
	app.currentDialog = nil

	// Remove keybindings for the dialog (safely)
	keybindings := dialog.GetKeybindings()
	for _, kb := range keybindings {
		// Safely delete keybinding - ignore errors if it doesn't exist
		app.gui.DeleteKeybinding(kb.View, kb.Key, kb.Mod)
	}

	// Hide the dialog (avoid calling Close() to prevent recursion)
	if hideComponent, ok := dialog.(interface{ Hide() error }); ok {
		hideComponent.Hide()
	}

	// Return focus to input panel (safely)
	app.gui.Update(func(g *gocui.Gui) error {
		return app.focusPanelByName("input")
	})

	return nil
}

// IGuiCommon interface implementation
func (app *App) GetGui() *gocui.Gui {
	return app.gui
}

func (app *App) GetConfig() *types.Config {
	return app.uiState.GetConfig()
}

func (app *App) GetTheme() *types.Theme {
	config := app.uiState.GetConfig()
	return presentation.GetThemeForMode(config.Theme, config.OutputMode)
}

func (app *App) PostUIUpdate(fn func()) {
	app.gui.Update(func(g *gocui.Gui) error {
		fn()
		return nil
	})
}

func (app *App) exit() error {
	// Properly close the GUI to restore terminal state
	app.gui.Close()
	// Force exit the application
	os.Exit(0)
	return nil // This will never be reached
}
