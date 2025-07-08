package tui

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/commands"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/controllers"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/presentation"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	pkgEvents "github.com/kcaldas/genie/pkg/events"
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
	stateAccessor *state.StateAccessor

	messageFormatter *presentation.MessageFormatter
	todoFormatter    *presentation.TodoFormatter
	layoutManager    *layout.LayoutManager

	messagesComponent         *component.MessagesComponent
	inputComponent            *component.InputComponent
	debugComponent            *component.DebugComponent
	statusComponent           *component.StatusComponent
	textViewerComponent       *component.TextViewerComponent
	diffViewerComponent       *component.DiffViewerComponent
	llmContextViewerComponent *component.LLMContextViewerComponent

	// Right panel state
	rightPanelVisible bool
	rightPanelMode    string // "debug", "text-viewer", or "diff-viewer"

	chatController *controllers.ChatController
	commandHandler *controllers.CommandHandler

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
	app.messageFormatter, err = presentation.NewMessageFormatter(config, theme)
	if err != nil {
		g.Close()
		return nil, err
	}

	app.todoFormatter = presentation.NewTodoFormatter(theme)

	if err := app.setupComponentsAndControllers(); err != nil {
		g.Close()
		return nil, err
	}

	// Initialize keymap after components are set up
	app.keymap = app.createKeymap()

	// Create help renderer after keymap is initialized
	app.helpRenderer = NewManPageHelpRenderer(app.commandHandler.GetRegistry(), app.keymap)

	app.setupEventSubscriptions()

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

	app.messagesComponent = component.NewMessagesComponent(guiCommon, app.stateAccessor, app.messageFormatter)
	app.inputComponent = component.NewInputComponent(guiCommon, app.commandEventBus, historyPath)
	app.debugComponent = component.NewDebugComponent(guiCommon, app.stateAccessor)
	app.statusComponent = component.NewStatusComponent(guiCommon, app.stateAccessor)
	app.textViewerComponent = component.NewTextViewerComponent(guiCommon, "Help")
	app.diffViewerComponent = component.NewDiffViewerComponent(guiCommon, "Diff")
	app.llmContextViewerComponent = component.NewLLMContextViewerComponent(guiCommon, app.genie, func() error {
		app.currentDialog = nil
		app.contextViewerActive = false // Clear the flag
		// Restore focus to input component
		return app.focusViewByName("input")
	})

	// Initialize right panel state
	app.rightPanelVisible = false
	app.rightPanelMode = "debug" // Default to debug mode

	// Load history on startup
	if err := app.inputComponent.LoadHistory(); err != nil {
		// Don't fail startup if history loading fails, just log it
		// Since we're discarding logs in TUI mode, this won't show up
	}

	// Set tab handlers for components to enable tab navigation
	app.inputComponent.SetTabHandler(app.nextView)
	app.messagesComponent.SetTabHandler(app.nextView)
	app.debugComponent.SetTabHandler(app.nextView)
	app.textViewerComponent.SetTabHandler(app.nextView)
	app.diffViewerComponent.SetTabHandler(app.nextView)

	// Map components using semantic names
	app.layoutManager.SetWindowComponent("messages", app.messagesComponent)      // messages in center
	app.layoutManager.SetWindowComponent("input", app.inputComponent)            // input at bottom
	app.layoutManager.SetWindowComponent("debug", app.debugComponent)            // debug on right side
	app.layoutManager.SetWindowComponent("text-viewer", app.textViewerComponent) // text viewer on right side
	app.layoutManager.SetWindowComponent("diff-viewer", app.diffViewerComponent) // diff viewer on right side
	app.layoutManager.SetWindowComponent("status", app.statusComponent)          // status at top

	// Register status sub-components
	app.layoutManager.SetWindowComponent("status-left", app.statusComponent.GetLeftComponent())
	app.layoutManager.SetWindowComponent("status-center", app.statusComponent.GetCenterComponent())
	app.layoutManager.SetWindowComponent("status-right", app.statusComponent.GetRightComponent())

	app.commandHandler = controllers.NewCommandHandler(app.commandEventBus)

	// Set up unknown command handler
	app.commandHandler.SetUnknownCommandHandler(func(commandName string) {
		app.stateAccessor.AddMessage(types.Message{
			Role:    "error",
			Content: fmt.Sprintf("Unknown command: %s. Type :? for available commands.", commandName),
		})
		app.refreshUI()
	})

	app.chatController = controllers.NewChatController(
		app.messagesComponent,
		guiCommon,
		app.genie,
		app.stateAccessor,
		app.commandEventBus,
	)

	// Initialize confirmation controllers
	eventBus := app.genie.GetEventBus()
	app.toolConfirmationController = controllers.NewToolConfirmationController(
		guiCommon,
		app.stateAccessor,
		app.layoutManager,
		app.inputComponent,
		app.statusComponent,
		eventBus,
		app.renderMessagesWithAutoScroll,
		app.focusViewByName,
		func(confirmationType string) { app.activeConfirmationType = confirmationType },
	)

	app.userConfirmationController = controllers.NewUserConfirmationController(
		guiCommon,
		app.stateAccessor,
		app.layoutManager,
		app.inputComponent,
		eventBus,
		app.renderMessagesWithAutoScroll,
		app.focusViewByName,
		app.ShowDiffInViewer,
		app.HideRightPanel,
		func(confirmationType string) { app.activeConfirmationType = confirmationType },
	)

	app.setupCommands()

	return nil
}

func (app *App) setupCommands() {
	// Create command context for dependency injection
	ctx := &commands.CommandContext{
		StateAccessor:          app.stateAccessor,
		GuiCommon:              app,
		ClipboardHelper:        app.helpers.Clipboard,
		ConfigHelper:           app.helpers.Config,
		ShowLLMContextViewer:   app.showLLMContextViewer,
		SetCurrentView:         app.setCurrentView,
		ChatController:         app.chatController,
		DebugComponent:         app.debugComponent,
		LayoutManager:          app.layoutManager,
		MessageFormatter:       app.messageFormatter,
		RefreshTheme:           app.refreshComponentThemes,
		GetHelpText:            func() string { return app.helpRenderer.RenderHelp() },
		ShowHelpInTextViewer:   app.ShowHelpInTextViewer,
		ToggleHelpInTextViewer: app.ToggleHelpInTextViewer,
		Exit:                   app.exit,
		CommandEventBus:        app.commandEventBus,
	}

	// Register new command types
	app.commandHandler.RegisterNewCommand(commands.NewHelpCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewContextCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewClearCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewDebugCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewExitCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewFocusCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewYankCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewLayoutCommand(ctx))

	// Note: HelpRenderer will be created after keymap is initialized
	app.commandHandler.RegisterNewCommand(commands.NewThemeCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewConfigCommand(ctx))

	// Note: Shortcuts are now managed by the keymap system, not the command registry

	// Register hidden debug/demo commands
	// debugCommands := []*controllers.Command{
	// 	{
	// 		Name:        "theme-debug",
	// 		Description: "Show theme debug information and force refresh",
	// 		Usage:       ":theme-debug",
	// 		Examples: []string{
	// 			":theme-debug",
	// 		},
	// 		Aliases:  []string{"td"},
	// 		Category: "Configuration",
	// 		Handler:  app.cmdThemeDebug,
	// 		Hidden:   true, // Hidden from main help, accessible via alias
	// 	},
	// 	{
	// 		Name:        "markdown-demo",
	// 		Description: "Show markdown rendering demo with current theme",
	// 		Usage:       ":markdown-demo",
	// 		Examples: []string{
	// 			":markdown-demo",
	// 		},
	// 		Aliases:  []string{"md"},
	// 		Category: "Configuration",
	// 		Handler:  app.cmdMarkdownDemo,
	// 		Hidden:   true, // Hidden from main help, accessible via alias
	// 	},
	// 	{
	// 		Name:        "glamour-test",
	// 		Description: "Test specific glamour markdown styles",
	// 		Usage:       ":glamour-test [style]",
	// 		Examples: []string{
	// 			":glamour-test",
	// 			":glamour-test dracula",
	// 			":glamour-test pink",
	// 			":glamour-test tokyo-night",
	// 		},
	// 		Aliases:  []string{"gt"},
	// 		Category: "Configuration",
	// 		Handler:  app.cmdGlamourTest,
	// 		Hidden:   true, // Hidden from main help, accessible via alias
	// 	},
	// 	{
	// 		Name:        "diff-demo",
	// 		Description: "Show diff viewer demo with sample diff content",
	// 		Usage:       ":diff-demo [title]",
	// 		Examples: []string{
	// 			":diff-demo",
	// 			":diff-demo \"My Changes\"",
	// 		},
	// 		Aliases:  []string{"dd"},
	// 		Category: "Configuration",
	// 		Handler:  app.cmdDiffDemo,
	// 		Hidden:   true, // Hidden from main help, accessible via alias
	// 	},
	// }
	//
	// // Register debug commands only
	// for _, cmd := range debugCommands {
	// 	app.commandHandler.Register(cmd)
	// }
}

func (app *App) createKeymap() *Keymap {
	keymap := NewKeymap()

	// Event-driven shortcuts - these emit events for consistent behavior
	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyF1,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(func() error {
			app.commandEventBus.Emit("shortcut.help", nil)
			return nil
		}),
		Description: "Show help dialog",
	})

	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyF12,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.toggleDebugPanel),
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
		Action:      FunctionAction(app.globalPageUpKeymap),
		Description: "Scroll messages up",
	})
	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyCtrlU,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.globalPageUpKeymap),
		Description: "Scroll messages up",
	})

	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyPgdn,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.globalPageDownKeymap),
		Description: "Scroll messages down",
	})
	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyCtrlD,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.globalPageDownKeymap),
		Description: "Scroll messages down",
	})

	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyArrowUp,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.scrollUpMessages),
		Description: "Scroll messages up",
	})
	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyArrowDown,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.scrollDownMessages),
		Description: "Scroll messages down",
	})

	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyHome,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.scrollToTopMessages),
		Description: "Scroll to top of messages",
	})

	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyEnd,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.scrollToBottomMessages),
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
		Key:         gocui.KeyCtrlC,
		Mod:         gocui.ModNone,
		Action:      FunctionAction(app.quitKeymap),
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
			return app.llmContextViewerComponent.Close()
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
	// Determine if this is a "yes" or "no" key
	var confirmed bool
	switch key {
	case '1', 'y', 'Y':
		confirmed = true
	case '2', 'n', 'N', gocui.KeyEsc:
		confirmed = false
	default:
		return nil // Unknown key
	}

	// Clear the active confirmation type
	app.activeConfirmationType = ""

	// Call the tool confirmation controller's response handler
	if app.toolConfirmationController.ConfirmationComponent != nil {
		executionID := app.toolConfirmationController.ConfirmationComponent.ExecutionID
		return app.toolConfirmationController.HandleToolConfirmationResponse(executionID, confirmed)
	}

	return nil
}

func (app *App) handleUserConfirmationKey(key interface{}) error {
	// Determine if this is a "yes" or "no" key
	var confirmed bool
	switch key {
	case '1', 'y', 'Y':
		confirmed = true
	case '2', 'n', 'N', gocui.KeyEsc:
		confirmed = false
	default:
		return nil // Unknown key
	}

	// Clear the active confirmation type
	app.activeConfirmationType = ""

	// Call the user confirmation controller's response handler
	if app.userConfirmationController.ConfirmationComponent != nil {
		executionID := app.userConfirmationController.ConfirmationComponent.ExecutionID
		return app.userConfirmationController.HandleUserConfirmationResponse(executionID, confirmed)
	}

	return nil
}

func (app *App) Run() error {
	// Add welcome message first
	app.showWelcomeMessage()

	// Initial render
	if err := app.renderMessagesWithAutoScroll(); err != nil {
		return err
	}

	if err := app.statusComponent.Render(); err != nil {
		return err
	}

	// Set focus to input after everything is set up using semantic naming
	app.gui.Update(func(g *gocui.Gui) error {
		return app.focusViewByName("input") // Use semantic name directly
	})

	// Start periodic status updates
	app.startStatusUpdates()

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

func (app *App) startStatusUpdates() {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond) // Update 10 times per second for smooth spinner
		defer ticker.Stop()

		for range ticker.C {
			app.gui.Update(func(g *gocui.Gui) error {
				// Update spinner based on current state - confirmation takes priority
				if app.stateAccessor.IsWaitingConfirmation() {
					spinner := app.getConfirmationSpinnerFrame()
					app.statusComponent.SetLeftText("Your call " + spinner)
				} else if app.stateAccessor.IsLoading() {
					spinner := app.getSpinnerFrame()
					duration := app.stateAccessor.GetLoadingDuration()
					seconds := int(duration.Seconds())
					thinkingText := app.getThinkingText(&seconds)
					config := app.GetConfig()
					theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
					tertiaryColor := presentation.ConvertColorToAnsi(theme.TextTertiary)
					resetColor := "\033[0m"
					escHint := "(ESC to cancel)"
					if tertiaryColor != "" {
						escHint = tertiaryColor + escHint + resetColor
					}
					app.statusComponent.SetLeftText(fmt.Sprintf("%s %s %s", thinkingText, spinner, escHint))
				}
				return app.statusComponent.Render()
			})
		}
	}()
}

func (app *App) getSpinnerFrame() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frame := frames[time.Now().UnixNano()/100000000%int64(len(frames))]

	// Color the spinner with error color
	config := app.GetConfig()
	theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
	errorColor := presentation.ConvertColorToAnsi(theme.Error)
	resetColor := "\033[0m"

	if errorColor != "" {
		frame = errorColor + frame + resetColor
	}
	return frame
}

func (app *App) getConfirmationSpinnerFrame() string {
	frames := []string{"◐", "◓", "◑", "◒"}
	frame := frames[time.Now().UnixNano()/200000000%int64(len(frames))]

	// Color the confirmation spinner with error color
	config := app.GetConfig()
	theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
	errorColor := presentation.ConvertColorToAnsi(theme.Error)
	resetColor := "\033[0m"

	if errorColor != "" {
		frame = errorColor + frame + resetColor
	}
	return frame
}

// getThinkingText returns "Thinking" text with optional time in tertiary color
func (app *App) getThinkingText(seconds *int) string {
	config := app.GetConfig()
	theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
	tertiaryColor := presentation.ConvertColorToAnsi(theme.TextTertiary)
	resetColor := "\033[0m"

	thinkingText := "Thinking"
	if tertiaryColor != "" {
		thinkingText = tertiaryColor + thinkingText + resetColor
	}

	if seconds != nil {
		timeText := fmt.Sprintf("(%ds)", *seconds)
		if tertiaryColor != "" {
			timeText = tertiaryColor + timeText + resetColor
		}
		return fmt.Sprintf("%s %s", thinkingText, timeText)
	}

	return thinkingText
}

func (app *App) formatToolCall(toolName string, params map[string]any) string {
	// Special case for TodoWrite - just show "Updated Todos"
	if toolName == "TodoWrite" {
		return "Updated Todos"
	}

	// Get theme colors for formatting
	config := app.GetConfig()
	theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
	tertiaryColor := presentation.ConvertColorToAnsi(theme.TextTertiary)
	resetColor := "\033[0m"

	if len(params) == 0 {
		paramsText := "()"
		if tertiaryColor != "" {
			paramsText = tertiaryColor + paramsText + resetColor
		}
		return fmt.Sprintf("%s%s", toolName, paramsText)
	}

	var paramPairs []string
	for key, value := range params {
		// Format the value appropriately
		var valueStr string
		switch v := value.(type) {
		case string:
			// Truncate long strings
			if len(v) > 50 {
				valueStr = fmt.Sprintf(`"%s..."`, v[:50])
			} else {
				valueStr = fmt.Sprintf(`"%s"`, v)
			}
		case bool:
			valueStr = fmt.Sprintf("%t", v)
		case nil:
			valueStr = "null"
		default:
			valueStr = fmt.Sprintf("%v", v)
		}
		paramPairs = append(paramPairs, fmt.Sprintf("%s: %s", key, valueStr))
	}

	// Sort for consistent display
	sort.Strings(paramPairs)

	paramsText := fmt.Sprintf("(%s)", strings.Join(paramPairs, ", "))
	if tertiaryColor != "" {
		paramsText = tertiaryColor + paramsText + resetColor
	}

	return fmt.Sprintf("%s%s", toolName, paramsText)
}

func (app *App) Close() {
	if app.gui != nil {
		app.gui.Close()
	}
}

func (app *App) setCurrentView(name string) error {
	app.layoutManager.SetFocus(name)

	oldPanel := app.uiState.GetFocusedPanel()
	newPanel := app.viewNameToPanel(name)

	// Handle focus lost for old component
	if oldCtx := app.panelToComponent(oldPanel); oldCtx != nil {
		oldCtx.HandleFocusLost()
	}

	// Handle focus gained for new component
	if newCtx := app.layoutManager.GetWindowComponent(name); newCtx != nil {
		newCtx.HandleFocus()
	}

	app.uiState.SetFocusedPanel(newPanel)

	return nil
}

func (app *App) viewNameToPanel(name string) types.FocusablePanel {
	switch name {
	case "messages":
		return types.PanelMessages
	case "input":
		return types.PanelInput
	case "debug":
		return types.PanelDebug
	case "text-viewer":
		return types.PanelDebug // Right panels use same enum value
	case "diff-viewer":
		return types.PanelDebug // Right panels use same enum value
	case "status":
		return types.PanelStatus
	default:
		return types.PanelInput
	}
}

func (app *App) panelToComponent(panel types.FocusablePanel) types.Component {
	switch panel {
	case types.PanelMessages:
		return app.layoutManager.GetWindowComponent("messages")
	case types.PanelInput:
		return app.layoutManager.GetWindowComponent("input")
	case types.PanelDebug:
		return app.layoutManager.GetWindowComponent("debug")
	case types.PanelStatus:
		return app.layoutManager.GetWindowComponent("status")
	default:
		return nil
	}
}

func (app *App) nextView(g *gocui.Gui, v *gocui.View) error {
	// Get available panels from layout manager and convert to view names
	availablePanels := app.layoutManager.GetAvailablePanels()
	views := []string{}

	// Prioritize input and messages panels, then add others (using semantic names)
	// Note: status panel excluded from tab navigation
	panelOrder := []string{"input", "messages", "debug", "text-viewer", "diff-viewer", "left"}
	for _, panel := range panelOrder {
		for _, available := range availablePanels {
			if panel == available {
				// Only add right panel components if they're visible
				if panel == "debug" && !app.debugComponent.IsVisible() {
					continue
				}
				if panel == "text-viewer" && !app.textViewerComponent.IsVisible() {
					continue
				}
				if panel == "diff-viewer" && !app.diffViewerComponent.IsVisible() {
					continue
				}

				// Get the actual view name from the component
				if component := app.layoutManager.GetWindowComponent(panel); component != nil {
					viewName := component.GetViewName()
					views = append(views, viewName)
				}
				break
			}
		}
	}

	if len(views) == 0 {
		return nil
	}

	currentView := g.CurrentView()
	if currentView == nil {
		return app.focusViewByName(views[0])
	}

	currentName := currentView.Name()

	// Note: Auto-scroll control removed - always scroll to bottom

	for i, name := range views {
		if name == currentName {
			nextIndex := (i + 1) % len(views)
			return app.focusViewByName(views[nextIndex])
		}
	}

	return app.focusViewByName(views[0])
}

func (app *App) focusViewByName(viewName string) error {
	// Handle focus events for components using semantic names
	// Note: status excluded from tab navigation but can still be focused programmatically
	for panelName, component := range map[string]types.Component{
		"messages":    app.messagesComponent,
		"input":       app.inputComponent,
		"debug":       app.debugComponent,
		"text-viewer": app.textViewerComponent,
		"diff-viewer": app.diffViewerComponent,
		"status":      app.statusComponent, // Keep for programmatic focus
	} {
		if component.GetViewName() == viewName {
			oldPanel := app.uiState.GetFocusedPanel()
			newPanel := app.viewNameToPanel(panelName)

			// Handle focus lost for old component
			if oldCtx := app.panelToComponent(oldPanel); oldCtx != nil {
				oldCtx.HandleFocusLost()
			}

			// Set current view and ensure it's properly configured
			if view, err := app.gui.SetCurrentView(viewName); err == nil && view != nil {
				// Special handling for input view
				if panelName == "input" {
					view.Editable = true
					view.SetCursor(0, 0)
					// Ensure cursor is visible
					app.gui.Cursor = true
				}
			}

			// Handle focus gained for new component
			component.HandleFocus()
			app.uiState.SetFocusedPanel(newPanel)
			break
		}
	}

	// Fallback: try to set focus directly (for dialog views)
	if view, err := app.gui.SetCurrentView(viewName); err == nil && view != nil {
		view.Highlight = true
		return nil
	}

	return nil
}

func (app *App) globalPageUp(g *gocui.Gui, v *gocui.View) error {
	// Use central scroll management
	return app.pageUpMessages()
}

func (app *App) globalPageDown(g *gocui.Gui, v *gocui.View) error {
	// Use central scroll management
	return app.pageDownMessages()
}

func (app *App) quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (app *App) handleEscKey() error {
	// First check if context viewer is active
	if app.contextViewerActive {
		return app.llmContextViewerComponent.Close()
	}

	// Cancel the chat
	app.chatController.CancelChat()
	// Render messages to show the cancellation
	return app.renderMessagesWithAutoScroll()
}

// Keymap-compatible wrapper methods (no gocui parameters)
func (app *App) globalPageUpKeymap() error {
	return app.pageUpMessages()
}

func (app *App) globalPageDownKeymap() error {
	return app.pageDownMessages()
}

func (app *App) quitKeymap() error {
	return gocui.ErrQuit
}

func (app *App) toggleDebugPanel() error {
	app.debugComponent.ToggleVisibility()

	// Force layout refresh and cleanup when hiding
	app.gui.Update(func(g *gocui.Gui) error {
		if !app.debugComponent.IsVisible() {
			// Delete the debug view when hiding
			g.DeleteView("debug")
		} else {
			// If becoming visible, render to show all collected messages
			app.debugComponent.Render()
		}
		return nil
	})

	return nil
}

// Central scroll management methods (lazygit-style)

func (app *App) getMessagesView() *gocui.View {
	if app.messagesComponent != nil {
		return app.messagesComponent.GetView()
	}
	return nil
}

func (app *App) scrollUpMessages() error {
	// Check if text viewer is active and redirect scrolling there
	if app.rightPanelVisible && app.rightPanelMode == "text-viewer" {
		if textView := app.textViewerComponent.GetView(); textView != nil {
			ox, oy := textView.Origin()
			if oy > 0 {
				textView.SetOrigin(ox, oy-1)
			}
			return nil
		}
	}

	// Check if diff viewer is active and redirect scrolling there
	if app.rightPanelVisible && app.rightPanelMode == "diff-viewer" {
		if diffView := app.diffViewerComponent.GetView(); diffView != nil {
			ox, oy := diffView.Origin()
			if oy > 0 {
				diffView.SetOrigin(ox, oy-1)
			}
			return nil
		}
	}

	// Default: scroll messages
	view := app.getMessagesView()
	if view == nil {
		return nil
	}

	// Simple scroll up - no auto-scroll state management
	ox, oy := view.Origin()
	if oy > 0 {
		view.SetOrigin(ox, oy-1)
	}
	return nil
}

func (app *App) scrollDownMessages() error {
	// Check if text viewer is active and redirect scrolling there
	if app.rightPanelVisible && app.rightPanelMode == "text-viewer" {
		if textView := app.textViewerComponent.GetView(); textView != nil {
			ox, oy := textView.Origin()
			textView.SetOrigin(ox, oy+1)
			return nil
		}
	}

	// Check if diff viewer is active and redirect scrolling there
	if app.rightPanelVisible && app.rightPanelMode == "diff-viewer" {
		if diffView := app.diffViewerComponent.GetView(); diffView != nil {
			ox, oy := diffView.Origin()
			diffView.SetOrigin(ox, oy+1)
			return nil
		}
	}

	// Default: scroll messages
	view := app.getMessagesView()
	if view == nil {
		return nil
	}

	// Simple scroll down - no auto-scroll state management
	ox, oy := view.Origin()
	view.SetOrigin(ox, oy+1)
	return nil
}

func (app *App) pageUpMessages() error {
	// Check if text viewer is active and redirect scrolling there
	if app.rightPanelVisible && app.rightPanelMode == "text-viewer" {
		if textView := app.textViewerComponent.GetView(); textView != nil {
			ox, oy := textView.Origin()
			_, height := textView.Size()
			newY := oy - height
			if newY < 0 {
				newY = 0
			}
			textView.SetOrigin(ox, newY)
			return nil
		}
	}

	// Check if diff viewer is active and redirect scrolling there
	if app.rightPanelVisible && app.rightPanelMode == "diff-viewer" {
		if diffView := app.diffViewerComponent.GetView(); diffView != nil {
			ox, oy := diffView.Origin()
			_, height := diffView.Size()
			newY := oy - height
			if newY < 0 {
				newY = 0
			}
			diffView.SetOrigin(ox, newY)
			return nil
		}
	}

	// Default: scroll messages
	view := app.getMessagesView()
	if view == nil {
		return nil
	}

	// Simple page up - no auto-scroll state management
	ox, oy := view.Origin()
	_, height := view.Size()
	newY := oy - height
	if newY < 0 {
		newY = 0
	}
	view.SetOrigin(ox, newY)
	return nil
}

func (app *App) pageDownMessages() error {
	// Check if text viewer is active and redirect scrolling there
	if app.rightPanelVisible && app.rightPanelMode == "text-viewer" {
		if textView := app.textViewerComponent.GetView(); textView != nil {
			ox, oy := textView.Origin()
			_, height := textView.Size()
			textView.SetOrigin(ox, oy+height)
			return nil
		}
	}

	// Check if diff viewer is active and redirect scrolling there
	if app.rightPanelVisible && app.rightPanelMode == "diff-viewer" {
		if diffView := app.diffViewerComponent.GetView(); diffView != nil {
			ox, oy := diffView.Origin()
			_, height := diffView.Size()
			diffView.SetOrigin(ox, oy+height)
			return nil
		}
	}

	// Default: scroll messages
	view := app.getMessagesView()
	if view == nil {
		return nil
	}

	// Simple page down - no auto-scroll state management
	ox, oy := view.Origin()
	_, height := view.Size()
	view.SetOrigin(ox, oy+height)
	return nil
}

func (app *App) scrollToTopMessages() error {
	view := app.getMessagesView()
	if view == nil {
		return nil
	}

	// Simple scroll to top
	view.SetOrigin(0, 0)
	return nil
}

func (app *App) scrollToBottomMessages() error {
	view := app.getMessagesView()
	if view == nil {
		return nil
	}

	// Get the view buffer content to count lines
	lines := len(strings.Split(view.ViewBuffer(), "\n"))
	_, height := view.Size()

	// Calculate where bottom should be
	targetY := lines - height
	if targetY < 0 {
		targetY = 0
	}

	view.SetOrigin(0, targetY)
	return nil
}

// Mouse wheel handlers for keymap
func (app *App) handleMouseWheelUp() error {
	// Scroll up by 3 lines for smooth scrolling
	for i := 0; i < 3; i++ {
		app.scrollUpMessages()
	}
	return nil
}

func (app *App) handleMouseWheelDown() error {
	// Scroll down by 3 lines for smooth scrolling
	for i := 0; i < 3; i++ {
		app.scrollDownMessages()
	}
	return nil
}

// Helper method to render messages and always scroll to bottom
func (app *App) renderMessagesWithAutoScroll() error {
	// Render messages first
	if err := app.messagesComponent.Render(); err != nil {
		return err
	}

	// Schedule scroll to bottom after the UI update completes
	app.gui.Update(func(g *gocui.Gui) error {
		return app.scrollToBottomMessages()
	})

	return nil
}

func (app *App) setupEventSubscriptions() {
	eventBus := app.genie.GetEventBus()

	// Log subscription setup
	app.stateAccessor.AddDebugMessage("Setting up event subscriptions")

	// Subscribe to chat response events
	app.stateAccessor.AddDebugMessage("Subscribing to: chat.response")
	eventBus.Subscribe("chat.response", func(e interface{}) {
		app.stateAccessor.AddDebugMessage("Event consumed: chat.response")
		if event, ok := e.(pkgEvents.ChatResponseEvent); ok {
			app.gui.Update(func(g *gocui.Gui) error {
				app.stateAccessor.SetLoading(false)
				// Reset status left back to ready indicator
				app.statusComponent.SetLeftToReady()

				// Handle chat completion - only log errors to debug
				if event.Error != nil {
					app.stateAccessor.AddDebugMessage(fmt.Sprintf("Chat failed: %v", event.Error))
					// Don't show context cancellation errors as they're user-initiated
					if !strings.Contains(event.Error.Error(), "context canceled") {
						app.stateAccessor.AddMessage(types.Message{
							Role:    "error",
							Content: fmt.Sprintf("Error: %v", event.Error),
						})
					}
				} else {
					app.stateAccessor.AddMessage(types.Message{
						Role:        "assistant",
						Content:     event.Response,
						ContentType: "markdown",
					})
				}

				// Render debug panel if visible
				if app.debugComponent.IsVisible() {
					app.debugComponent.Render()
				}

				return app.renderMessagesWithAutoScroll()
			})
		}
	})

	// Subscribe to chat started events
	app.stateAccessor.AddDebugMessage("Subscribing to: chat.started")
	eventBus.Subscribe("chat.started", func(e interface{}) {
		app.stateAccessor.AddDebugMessage("Event consumed: chat.started")
		if _, ok := e.(pkgEvents.ChatStartedEvent); ok {
			app.gui.Update(func(g *gocui.Gui) error {
				app.stateAccessor.SetLoading(true)
				// Show spinner in status left
				spinner := app.getSpinnerFrame()
				thinkingText := app.getThinkingText(nil)
				config := app.GetConfig()
				theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
				tertiaryColor := presentation.ConvertColorToAnsi(theme.TextTertiary)
				resetColor := "\033[0m"
				escHint := "(ESC to cancel)"
				if tertiaryColor != "" {
					escHint = tertiaryColor + escHint + resetColor
				}
				app.statusComponent.SetLeftText(fmt.Sprintf("%s %s %s", thinkingText, spinner, escHint))

				// Render debug panel if visible
				if app.debugComponent.IsVisible() {
					app.debugComponent.Render()
				}

				return app.renderMessagesWithAutoScroll()
			})
		}
	})

	// Subscribe to tool call events for debug panel only
	app.stateAccessor.AddDebugMessage("Subscribing to: tool.call")
	eventBus.Subscribe("tool.call", func(e interface{}) {
		app.stateAccessor.AddDebugMessage("Event consumed: tool.call")
		if _, ok := e.(pkgEvents.ToolCallEvent); ok {
			app.gui.Update(func(g *gocui.Gui) error {
				// Render debug panel if visible
				if app.debugComponent.IsVisible() {
					app.debugComponent.Render()
				}
				return nil
			})
		}
	})

	// Subscribe to tool executed events for debug panel and chat
	app.stateAccessor.AddDebugMessage("Subscribing to: tool.executed")
	eventBus.Subscribe("tool.executed", func(e interface{}) {
		app.stateAccessor.AddDebugMessage("Event consumed: tool.executed")
		if event, ok := e.(pkgEvents.ToolExecutedEvent); ok {
			// Skip TodoRead - don't show it in chat at all
			if event.ToolName == "TodoRead" {
				return
			}

			// Format the function call display for chat
			formattedCall := app.formatToolCall(event.ToolName, event.Parameters)

			// Determine success based on the message (no "Failed:" prefix means success)
			success := !strings.HasPrefix(event.Message, "Failed:")

			app.gui.Update(func(g *gocui.Gui) error {
				// Render debug panel if visible
				if app.debugComponent.IsVisible() {
					app.debugComponent.Render()
				}

				// Add formatted call to chat messages
				// Use assistant role for success (green) and error role for failures (red)
				role := "assistant"
				if !success {
					role = "error"
				}

				// Handle todo tools with special formatting
				var resultPreview string
				if event.ToolName == "TodoWrite" {
					// Use TodoFormatter for todo tools
					formattedTodos := app.todoFormatter.FormatTodoToolResult(event.Result)

					// Get theme colors
					config := app.GetConfig()
					theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
					tertiaryColor := presentation.ConvertColorToAnsi(theme.TextTertiary)
					resetColor := "\033[0m"

					// Add L-shaped formatting like other tools
					lines := strings.Split(strings.TrimSpace(formattedTodos), "\n")
					var formatted []string
					for i, line := range lines {
						if line != "" {
							if i == 0 {
								// First line gets the L-shaped character
								formatted = append(formatted, fmt.Sprintf("%s└─%s %s", tertiaryColor, resetColor, line))
							} else {
								// Subsequent lines just get indentation
								formatted = append(formatted, fmt.Sprintf("   %s", line))
							}
						}
					}
					resultPreview = "\n" + strings.Join(formatted, "\n")
				} else if event.Result != nil && len(event.Result) > 0 {
					// Format the result preview with L-shaped symbol and tertiary color for other tools
					config := app.GetConfig()
					theme := presentation.GetThemeForMode(config.Theme, config.OutputMode)
					tertiaryColor := presentation.ConvertColorToAnsi(theme.TextTertiary)
					resetColor := "\033[0m"

					// Extract a preview from the result
					var preview string
					// Try to get a meaningful preview from common result fields
					if content, ok := event.Result["content"].(string); ok && content != "" {
						preview = content
					} else if output, ok := event.Result["output"].(string); ok && output != "" {
						preview = output
					} else if data, ok := event.Result["data"].(string); ok && data != "" {
						preview = data
					} else {
						// Fallback to first string value found
						for _, v := range event.Result {
							if str, ok := v.(string); ok && str != "" {
								preview = str
								break
							}
						}
					}

					if preview != "" {
						// Show up to 5 lines with truncation
						lines := strings.Split(preview, "\n")
						var displayLines []string

						// Take first 5 lines
						maxLines := 5
						for i, line := range lines {
							if i >= maxLines {
								break
							}
							// Truncate each line to 80 chars
							if len(line) > 80 {
								line = line[:77] + "..."
							}
							displayLines = append(displayLines, line)
						}

						// Build the result preview with L-shaped symbols
						var resultLines []string
						for i, line := range displayLines {
							if i == 0 {
								resultLines = append(resultLines, fmt.Sprintf("%s└─ %s%s", tertiaryColor, line, resetColor))
							} else {
								resultLines = append(resultLines, fmt.Sprintf("%s   %s%s", tertiaryColor, line, resetColor))
							}
						}

						// Add truncation indicator if there are more lines
						if len(lines) > maxLines {
							resultLines = append(resultLines, fmt.Sprintf("%s   ...(truncated)%s", tertiaryColor, resetColor))
						}

						resultPreview = "\n" + strings.Join(resultLines, "\n")
					}
				}

				chatMsg := formattedCall + resultPreview
				app.stateAccessor.AddMessage(types.Message{
					Role:    role,
					Content: chatMsg,
				})

				return app.renderMessagesWithAutoScroll()
			})
		}
	})

	// Subscribe to tool call message events
	app.stateAccessor.AddDebugMessage("Subscribing to: tool.call.message")
	eventBus.Subscribe("tool.call.message", func(e interface{}) {
		app.stateAccessor.AddDebugMessage("Event consumed: tool.call.message")
		if event, ok := e.(pkgEvents.ToolCallMessageEvent); ok {
			app.gui.Update(func(g *gocui.Gui) error {
				app.stateAccessor.AddMessage(types.Message{
					Role:    "system",
					Content: event.Message,
				})
				return app.renderMessagesWithAutoScroll()
			})
		}
	})

	// Subscribe to tool confirmation requests
	app.stateAccessor.AddDebugMessage("Subscribing to: tool.confirmation.request")
	eventBus.Subscribe("tool.confirmation.request", func(e interface{}) {
		app.stateAccessor.AddDebugMessage("Event consumed: tool.confirmation.request")
		if event, ok := e.(pkgEvents.ToolConfirmationRequest); ok {
			app.gui.Update(func(g *gocui.Gui) error {
				return app.toolConfirmationController.HandleToolConfirmationRequest(event)
			})
		}
	})

	// Subscribe to user confirmation requests (rich confirmations with content preview)
	app.stateAccessor.AddDebugMessage("Subscribing to: user.confirmation.request")
	eventBus.Subscribe("user.confirmation.request", func(e interface{}) {
		app.stateAccessor.AddDebugMessage("Event consumed: user.confirmation.request")
		if event, ok := e.(pkgEvents.UserConfirmationRequest); ok {
			app.gui.Update(func(g *gocui.Gui) error {
				return app.userConfirmationController.HandleUserConfirmationRequest(event)
			})
		}
	})

	// Subscribe to UI refresh events from commands
	app.stateAccessor.AddDebugMessage("Subscribing to: ui.refresh")
	app.commandEventBus.Subscribe("ui.refresh", func(e interface{}) {
		app.stateAccessor.AddDebugMessage("Event consumed: ui.refresh")
		app.gui.Update(func(g *gocui.Gui) error {
			return app.refreshUI()
		})
	})

	// Log completion of subscription setup
	app.stateAccessor.AddDebugMessage("Event subscriptions setup complete")
}

func (app *App) showWelcomeMessage() {
	app.stateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: "Welcome to Genie! Type :? for help.",
	})

	app.gui.Update(func(g *gocui.Gui) error {
		return app.renderMessagesWithAutoScroll()
	})
}

// Dialog management methods

func (app *App) showLLMContextViewer() error {
	// Toggle behavior - close if already open
	if app.contextViewerActive {
		return app.llmContextViewerComponent.Close()
	}

	// Close any existing dialog first
	if err := app.closeCurrentDialog(); err != nil {
		return err
	}

	// Show the context viewer
	if err := app.llmContextViewerComponent.Show(); err != nil {
		return err
	}

	// Set up keybindings (only navigation keys, not Esc/q)
	for _, kb := range app.llmContextViewerComponent.GetKeybindings() {
		if err := app.gui.SetKeybinding(kb.View, kb.Key, kb.Mod, kb.Handler); err != nil {
			return err
		}
	}

	// Render initial content
	if err := app.llmContextViewerComponent.Render(); err != nil {
		return err
	}

	// Set as current dialog and mark context viewer as active
	app.currentDialog = app.llmContextViewerComponent
	app.contextViewerActive = true

	// Focus the context keys panel for navigation
	contextKeysViewName := app.llmContextViewerComponent.GetViewName() + "-context-keys"
	return app.focusViewByName(contextKeysViewName)
}

func (app *App) showConfirmationDialog(title, message, content, contentType, confirmText, cancelText string, onConfirm, onCancel, onClose func() error) error {
	// Close any existing dialog first
	if err := app.closeCurrentDialog(); err != nil {
		return err
	}

	// Create confirmation dialog
	confirmDialog := component.NewConfirmationDialogComponent(
		title, message, content, contentType,
		confirmText, cancelText,
		&guiCommon{app: app},
		onConfirm, onCancel, onClose,
	)

	// Show the dialog
	if err := confirmDialog.Show(); err != nil {
		return err
	}

	// Set up keybindings for the dialog
	for _, kb := range confirmDialog.GetKeybindings() {
		if err := app.gui.SetKeybinding(kb.View, kb.Key, kb.Mod, kb.Handler); err != nil {
			return err
		}
	}

	// Render initial content
	if err := confirmDialog.Render(); err != nil {
		return err
	}

	// Set as current dialog and focus it
	app.currentDialog = confirmDialog

	// Focus on main dialog view
	viewName := confirmDialog.GetViewName()
	return app.focusViewByName(viewName)
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
		return app.focusViewByName("input")
	})

	return nil
}

func (app *App) hasActiveDialog() bool {
	return app.currentDialog != nil
}

// Tool confirmation handlers

// IGuiCommon interface implementation
func (app *App) GetGui() *gocui.Gui {
	return app.gui
}

func (app *App) GetHelpText() string {
	if app.helpRenderer == nil {
		return "ERROR: Help renderer is nil"
	}
	return app.helpRenderer.RenderHelp()
}

func (app *App) GetConfig() *types.Config {
	return app.uiState.GetConfig()
}

func (app *App) GetTheme() *types.Theme {
	config := app.uiState.GetConfig()
	return presentation.GetThemeForMode(config.Theme, config.OutputMode)
}

func (app *App) SetCurrentComponent(component types.Component) {
	// Implementation if needed
}

func (app *App) GetCurrentComponent() types.Component {
	// Implementation if needed
	return nil
}

func (app *App) PostUIUpdate(fn func()) {
	app.gui.Update(func(g *gocui.Gui) error {
		fn()
		return nil
	})
}

// Right panel management methods

func (app *App) ShowRightPanel(mode string) error {
	app.rightPanelVisible = true
	app.rightPanelMode = mode

	// Hide all right panel components first
	app.debugComponent.SetVisible(false)
	app.textViewerComponent.SetVisible(false)
	app.diffViewerComponent.SetVisible(false)

	// Then show only the target component
	switch mode {
	case "debug":
		app.debugComponent.SetVisible(true)
	case "text-viewer":
		app.textViewerComponent.SetVisible(true)
	case "diff-viewer":
		app.diffViewerComponent.SetVisible(true)
	}

	// Refresh UI first to create the view
	err := app.refreshUI()
	if err != nil {
		return err
	}

	// Update using gocui to directly write to the view
	if mode == "text-viewer" {
		app.gui.Update(func(g *gocui.Gui) error {
			v, err := g.View("text-viewer")
			if err != nil {
				// View might not exist yet, skip direct write
				// The component's Render() method will handle it
				return nil
			}
			v.Clear()
			content := app.textViewerComponent.GetContent()
			if content != "" {
				// Process markdown if needed
				displayContent := content
				if app.textViewerComponent.GetContentType() == "markdown" {
					displayContent = app.textViewerComponent.ProcessMarkdown(content)
				}
				fmt.Fprint(v, displayContent)
			}
			return nil
		})
	} else if mode == "diff-viewer" {
		app.gui.Update(func(g *gocui.Gui) error {
			v, err := g.View("diff-viewer")
			if err != nil {
				// View might not exist yet, skip direct write
				// The component's Render() method will handle it
				return nil
			}
			v.Clear()
			content := app.diffViewerComponent.GetContent()
			if content != "" {
				// Format diff content with theme colors
				formattedContent := app.diffViewerComponent.FormatDiff(content)
				fmt.Fprint(v, formattedContent)
			}
			return nil
		})
	}

	return nil
}

func (app *App) HideRightPanel() error {
	app.rightPanelVisible = false
	app.debugComponent.SetVisible(false)
	app.textViewerComponent.SetVisible(false)
	app.diffViewerComponent.SetVisible(false)

	// Explicitly delete views when hiding
	app.gui.Update(func(g *gocui.Gui) error {
		g.DeleteView("debug")
		g.DeleteView("text-viewer")
		g.DeleteView("diff-viewer")
		return nil
	})

	return app.refreshUI()
}

func (app *App) ToggleRightPanel() error {
	if app.rightPanelVisible {
		return app.HideRightPanel()
	} else {
		return app.ShowRightPanel(app.rightPanelMode)
	}
}

func (app *App) SwitchRightPanelMode(mode string) error {
	app.rightPanelMode = mode
	if app.rightPanelVisible {
		return app.ShowRightPanel(mode)
	}
	return nil
}

func (app *App) IsRightPanelVisible() bool {
	return app.rightPanelVisible
}

func (app *App) GetRightPanelMode() string {
	return app.rightPanelMode
}

// Helper method to show help in text viewer
func (app *App) ShowHelpInTextViewer() error {
	helpText := app.helpRenderer.RenderHelp()
	app.textViewerComponent.SetContentWithType(helpText, "markdown")
	app.textViewerComponent.SetTitle("Help")
	return app.ShowRightPanel("text-viewer")
}

// Helper method to toggle help in text viewer
func (app *App) ToggleHelpInTextViewer() error {
	// If right panel is visible and showing text-viewer, hide it
	if app.rightPanelVisible && app.rightPanelMode == "text-viewer" {
		return app.HideRightPanel()
	}
	// Otherwise show help
	return app.ShowHelpInTextViewer()
}

// Helper methods for diff viewer
func (app *App) ShowDiffInViewer(diffContent, title string) error {
	// Set content first
	app.diffViewerComponent.SetContent(diffContent)
	app.diffViewerComponent.SetTitle(title)

	// Show the right panel first
	if err := app.ShowRightPanel("diff-viewer"); err != nil {
		return err
	}

	// Use a separate GUI update for rendering to avoid race conditions
	app.gui.Update(func(g *gocui.Gui) error {
		// Ensure the view exists before rendering
		if view, err := g.View("diff-viewer"); err == nil && view != nil {
			app.diffViewerComponent.Render()
		}
		// If view doesn't exist yet, that's ok - it will render on next cycle
		return nil
	})

	return nil
}

func (app *App) ToggleDiffInViewer() error {
	// If right panel is visible and showing diff-viewer, hide it
	if app.rightPanelVisible && app.rightPanelMode == "diff-viewer" {
		return app.HideRightPanel()
	}
	// Otherwise, if there's content to show, show the diff viewer
	if app.diffViewerComponent.GetContent() != "" {
		return app.ShowRightPanel("diff-viewer")
	}
	return nil
}

func (app *App) exit() error {
	// Properly close the GUI to restore terminal state
	app.gui.Close()
	// Force exit the application
	os.Exit(0)
	return nil // This will never be reached
}
