package tui2

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/commands"
	"github.com/kcaldas/genie/cmd/tui2/component"
	"github.com/kcaldas/genie/cmd/tui2/controllers"
	"github.com/kcaldas/genie/cmd/tui2/helpers"
	"github.com/kcaldas/genie/cmd/tui2/layout"
	"github.com/kcaldas/genie/cmd/tui2/presentation"
	"github.com/kcaldas/genie/cmd/tui2/state"
	"github.com/kcaldas/genie/cmd/tui2/types"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
)

type App struct {
	gui     *gocui.Gui
	genie   genie.Genie
	session *genie.Session
	helpers *helpers.Helpers

	chatState     *state.ChatState
	uiState       *state.UIState
	stateAccessor *state.StateAccessor

	messageFormatter *presentation.MessageFormatter
	layoutManager    *layout.LayoutManager

	messagesComponent *component.MessagesComponent
	inputComponent    *component.InputComponent
	debugComponent    *component.DebugComponent
	statusComponent   *component.StatusComponent
	
	// Component for confirmation mode (shares same view as input)
	confirmationComponent *component.ConfirmationComponent

	chatController *controllers.ChatController
	commandHandler *controllers.SlashCommandHandler

	// Dialog management
	currentDialog types.Component

	// Keymap for centralized keybinding management
	keymap *Keymap

	keybindingsSetup bool // Track if keybindings have been set up
	
	// Note: Auto-scroll removed for now - always scroll to bottom after messages
}

func NewApp(genieService genie.Genie, session *genie.Session) (*App, error) {
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
	outputMode := helpers.Config.GetGocuiOutputMode(config.OutputMode)
	g, err := gocui.NewGui(outputMode, true)
	if err != nil {
		return nil, err
	}

	app := &App{
		gui:       g,
		genie:     genieService,
		session:   session,
		helpers:   helpers,
		chatState: state.NewChatState(),
		uiState:   state.NewUIState(config),
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

	if err := app.setupComponentsAndControllers(); err != nil {
		g.Close()
		return nil, err
	}

	// Initialize keymap after components are set up
	app.keymap = app.createKeymap()

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
	app.inputComponent = component.NewInputComponent(guiCommon, app.handleUserInput, historyPath)
	app.debugComponent = component.NewDebugComponent(guiCommon, app.stateAccessor)
	app.statusComponent = component.NewStatusComponent(guiCommon, app.stateAccessor)

	// Load history on startup
	if err := app.inputComponent.LoadHistory(); err != nil {
		// Don't fail startup if history loading fails, just log it
		// Since we're discarding logs in TUI mode, this won't show up
	}

	// Set tab handlers for components to enable tab navigation
	app.inputComponent.SetTabHandler(app.nextView)
	app.messagesComponent.SetTabHandler(app.nextView)
	app.debugComponent.SetTabHandler(app.nextView)

	// Map components using semantic names
	app.layoutManager.SetWindowComponent("messages", app.messagesComponent) // messages in center
	app.layoutManager.SetWindowComponent("input", app.inputComponent)       // input at bottom
	app.layoutManager.SetWindowComponent("debug", app.debugComponent)       // debug on right side
	app.layoutManager.SetWindowComponent("status", app.statusComponent)     // status at top

	// Register status sub-components
	app.layoutManager.SetWindowComponent("status-left", app.statusComponent.GetLeftComponent())
	app.layoutManager.SetWindowComponent("status-center", app.statusComponent.GetCenterComponent())
	app.layoutManager.SetWindowComponent("status-right", app.statusComponent.GetRightComponent())

	app.commandHandler = controllers.NewSlashCommandHandler()
	
	// Set up unknown command handler
	app.commandHandler.SetUnknownCommandHandler(func(commandName string) {
		app.stateAccessor.AddMessage(types.Message{
			Role:    "error",
			Content: fmt.Sprintf("Unknown command: %s. Type :? for available commands.", commandName),
		})
		app.refreshUI()
	})
	
	app.setupCommands()

	app.chatController = controllers.NewChatController(
		app.messagesComponent,
		guiCommon,
		app.genie,
		app.stateAccessor,
		app.commandHandler,
	)

	return nil
}

func (app *App) setupCommands() {
	// Create command context for dependency injection
	ctx := &commands.CommandContext{
		StateAccessor:    app.stateAccessor,
		GuiCommon:        app,
		ClipboardHelper:  app.helpers.Clipboard,
		ConfigHelper:     app.helpers.Config,
		RefreshUI:        app.refreshUI,
		ShowHelpDialog:   app.showHelpDialog,
		SetCurrentView:   app.setCurrentView,
		ChatController:   app.chatController,
		DebugComponent:   app.debugComponent,
		LayoutManager:    app.layoutManager,
		MessageFormatter: app.messageFormatter,
		RefreshTheme:     app.refreshComponentThemes,
	}

	// Register new command types
	app.commandHandler.RegisterNewCommand(commands.NewHelpCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewClearCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewDebugCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewExitCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewFocusCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewYankCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewLayoutCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewToggleCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewThemeCommand(ctx))
	app.commandHandler.RegisterNewCommand(commands.NewConfigCommand(ctx))

	// Note: Shortcuts are now managed by the keymap system, not the command registry

	// Register hidden debug/demo commands
	debugCommands := []*controllers.Command{
		{
			Name:        "theme-debug",
			Description: "Show theme debug information and force refresh",
			Usage:       ":theme-debug",
			Examples: []string{
				":theme-debug",
			},
			Aliases:  []string{"td"},
			Category: "Configuration",
			Handler:  app.cmdThemeDebug,
			Hidden:   true, // Hidden from main help, accessible via alias
		},
		{
			Name:        "markdown-demo",
			Description: "Show markdown rendering demo with current theme",
			Usage:       ":markdown-demo",
			Examples: []string{
				":markdown-demo",
			},
			Aliases:  []string{"md"},
			Category: "Configuration",
			Handler:  app.cmdMarkdownDemo,
			Hidden:   true, // Hidden from main help, accessible via alias
		},
		{
			Name:        "glamour-test",
			Description: "Test specific glamour markdown styles",
			Usage:       ":glamour-test [style]",
			Examples: []string{
				":glamour-test",
				":glamour-test dracula",
				":glamour-test pink",
				":glamour-test tokyo-night",
			},
			Aliases:  []string{"gt"},
			Category: "Configuration",
			Handler:  app.cmdGlamourTest,
			Hidden:   true, // Hidden from main help, accessible via alias
		},
	}

	// Register debug commands only
	for _, cmd := range debugCommands {
		app.commandHandler.Register(cmd)
	}
}

func (app *App) createKeymap() *Keymap {
	keymap := NewKeymap()

	// Command-mapped shortcuts - these execute equivalent commands
	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyF1,
		Mod:         gocui.ModNone,
		Action:      CommandAction("help"),
		Description: "Show help dialog",
	})

	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyF12,
		Mod:         gocui.ModNone,
		Action:      CommandAction("debug"),
		Description: "Toggle debug panel",
	})

	// Direct action shortcuts - these call App methods directly
	keymap.AddEntry(KeymapEntry{
		Key:         gocui.KeyPgup,
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
	// Setup keybindings for all components
	// Note: inputComponent and confirmationComponent share the same view name,
	// so both sets of keybindings will be registered to the "input" view
	components := []types.Component{app.messagesComponent, app.inputComponent, app.debugComponent, app.statusComponent}
	
	// Only add confirmation component if it exists (created on demand)
	if app.confirmationComponent != nil {
		components = append(components, app.confirmationComponent)
	}
	
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
					app.statusComponent.SetLeftText(fmt.Sprintf("Thinking (%ds) %s", seconds, spinner))
				}
				return app.statusComponent.Render()
			})
		}
	}()
}

func (app *App) getSpinnerFrame() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frame := frames[time.Now().UnixNano()/100000000%int64(len(frames))]
	return frame
}

func (app *App) getConfirmationSpinnerFrame() string {
	frames := []string{"◐", "◓", "◑", "◒"}
	frame := frames[time.Now().UnixNano()/200000000%int64(len(frames))]
	return frame
}

func (app *App) Close() {
	if app.gui != nil {
		app.gui.Close()
	}
}

func (app *App) handleUserInput(input types.UserInput) error {
	if err := app.chatController.HandleInput(input.Message); err != nil {
		// Special handling for quit command - propagate ErrQuit to exit application
		if err == gocui.ErrQuit {
			return err
		}
		
		// For other errors, display them as error messages
		app.stateAccessor.AddMessage(types.Message{
			Role:    "error",
			Content: err.Error(),
		})
		// Refresh UI to show the error message immediately
		app.refreshUI()
	} else {
		// Always refresh UI after handling input to show any new messages
		app.refreshUI()
	}
	return nil // Never return errors to avoid crashing the input component (except ErrQuit)
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
	panelOrder := []string{"input", "messages", "debug", "left"}
	for _, panel := range panelOrder {
		for _, available := range availablePanels {
			if panel == available {
				// Only add debug panel if it's visible
				if panel == "debug" && !app.debugComponent.IsVisible() {
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
		"messages": app.messagesComponent,
		"input":    app.inputComponent,
		"debug":    app.debugComponent,
		"status":   app.statusComponent, // Keep for programmatic focus
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

// Central scroll management methods (lazygit-style)

func (app *App) getMessagesView() *gocui.View {
	if app.messagesComponent != nil {
		return app.messagesComponent.GetView()
	}
	return nil
}

func (app *App) scrollUpMessages() error {
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

	// Subscribe to chat response events
	eventBus.Subscribe("chat.response", func(e interface{}) {
		if event, ok := e.(events.ChatResponseEvent); ok {
			app.gui.Update(func(g *gocui.Gui) error {
				app.stateAccessor.SetLoading(false)
				// Reset status left back to Ready
				app.statusComponent.SetLeftText("Ready")

				if event.Error != nil {
					app.stateAccessor.AddMessage(types.Message{
						Role:    "error",
						Content: fmt.Sprintf("Error: %v", event.Error),
					})
				} else {
					app.stateAccessor.AddMessage(types.Message{
						Role:    "assistant",
						Content: event.Response,
					})
				}

				return app.renderMessagesWithAutoScroll()
			})
		}
	})

	// Subscribe to chat started events
	eventBus.Subscribe("chat.started", func(e interface{}) {
		if _, ok := e.(events.ChatStartedEvent); ok {
			app.gui.Update(func(g *gocui.Gui) error {
				app.stateAccessor.SetLoading(true)
				// Show spinner in status left
				spinner := app.getSpinnerFrame()
				app.statusComponent.SetLeftText("Thinking " + spinner)
				return app.renderMessagesWithAutoScroll()
			})
		}
	})

	// Subscribe to tool call events for debug panel
	eventBus.Subscribe("tool.call", func(e interface{}) {
		if event, ok := e.(events.ToolCallEvent); ok {
			debugMsg := fmt.Sprintf("[TOOL CALL] %s: %v", event.ToolName, event.Parameters)
			app.gui.Update(func(g *gocui.Gui) error {
				app.stateAccessor.AddDebugMessage(debugMsg)
				if app.debugComponent.IsVisible() {
					return app.debugComponent.Render()
				}
				return nil
			})
		}
	})

	// Subscribe to tool executed events for debug panel
	eventBus.Subscribe("tool.executed", func(e interface{}) {
		if event, ok := e.(events.ToolExecutedEvent); ok {
			debugMsg := fmt.Sprintf("[TOOL EXECUTED] %s: %v", event.ToolName, event.Result)
			app.gui.Update(func(g *gocui.Gui) error {
				app.stateAccessor.AddDebugMessage(debugMsg)
				if app.debugComponent.IsVisible() {
					return app.debugComponent.Render()
				}
				return nil
			})
		}
	})

	// Subscribe to tool call message events
	eventBus.Subscribe("tool.call.message", func(e interface{}) {
		if event, ok := e.(events.ToolCallMessageEvent); ok {
			app.gui.Update(func(g *gocui.Gui) error {
				app.stateAccessor.AddMessage(types.Message{
					Role:    "system",
					Content: fmt.Sprintf("[%s] %s", event.ToolName, event.Message),
				})
				return app.renderMessagesWithAutoScroll()
			})
		}
	})

	// Subscribe to tool confirmation requests
	eventBus.Subscribe("tool.confirmation.request", func(e interface{}) {
		if event, ok := e.(events.ToolConfirmationRequest); ok {
			app.gui.Update(func(g *gocui.Gui) error {
				// Set confirmation state
				app.stateAccessor.SetWaitingConfirmation(true)
				return app.handleToolConfirmationRequest(event)
			})
		}
	})

	// Subscribe to user confirmation requests (rich confirmations with content preview)
	eventBus.Subscribe("user.confirmation.request", func(e interface{}) {
		if event, ok := e.(events.UserConfirmationRequest); ok {
			app.gui.Update(func(g *gocui.Gui) error {
				return app.handleUserConfirmationRequest(event)
			})
		}
	})
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

func (app *App) showHelpDialog(category string) error {
	// Close any existing dialog first
	if err := app.closeCurrentDialog(); err != nil {
		return err
	}

	// Create help dialog
	helpDialog := component.NewHelpDialogComponent(
		&guiCommon{app: app},
		app.commandHandler,
		func() error { return app.closeCurrentDialog() },
	)

	// Select specific category if provided
	if category != "" {
		helpDialog.SelectCategory(category)
	}

	// Show the dialog
	if err := helpDialog.Show(); err != nil {
		return err
	}

	// Set up keybindings for the dialog
	for _, kb := range helpDialog.GetKeybindings() {
		if err := app.gui.SetKeybinding(kb.View, kb.Key, kb.Mod, kb.Handler); err != nil {
			return err
		}
	}

	// Render initial content
	if err := helpDialog.Render(); err != nil {
		return err
	}

	// Set as current dialog and focus the categories panel
	app.currentDialog = helpDialog
	
	// Focus the categories panel for navigation
	categoriesViewName := helpDialog.GetViewName() + "-categories"
	return app.focusViewByName(categoriesViewName)
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

func (app *App) showConfirmationDialogWithDirectKeys(title, message, content, contentType, confirmText, cancelText string, onConfirm, onCancel, onClose func() error, executionID string) error {
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

	// Set up keybindings for the dialog (but we'll override them)
	for _, kb := range confirmDialog.GetKeybindings() {
		if err := app.gui.SetKeybinding(kb.View, kb.Key, kb.Mod, kb.Handler); err != nil {
			return err
		}
	}
	
	// Set up global keybindings for confirmation dialog
	app.gui.SetKeybinding("", '1', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		// Publish confirmation response
		eventBus := app.genie.GetEventBus()
		eventBus.Publish("tool.confirmation.response", events.ToolConfirmationResponse{
			ExecutionID: executionID,
			Confirmed:   true,
		})
		return app.closeCurrentDialog()
	})
	
	app.gui.SetKeybinding("", '2', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		// Publish rejection response
		eventBus := app.genie.GetEventBus()
		eventBus.Publish("tool.confirmation.response", events.ToolConfirmationResponse{
			ExecutionID: executionID,
			Confirmed:   false,
		})
		return app.closeCurrentDialog()
	})

	// Render initial content
	if err := confirmDialog.Render(); err != nil {
		return err
	}

	// Set as current dialog and focus it
	app.currentDialog = confirmDialog
	
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

func (app *App) handleToolConfirmationRequest(event events.ToolConfirmationRequest) error {
	// Show confirmation message in chat
	app.stateAccessor.AddMessage(types.Message{
		Role:    "system", 
		Content: event.Message,
	})
	
	// Create confirmation component if it doesn't exist
	if app.confirmationComponent == nil {
		app.confirmationComponent = component.NewConfirmationComponent(
			&guiCommon{app: app},
			event.ExecutionID,
			"1 - Yes | 2 - No",
			func(executionID string, confirmed bool) error {
				// Clear confirmation state
				app.stateAccessor.SetWaitingConfirmation(false)
				
				// Reset status back to Ready
				app.statusComponent.SetLeftText("Ready")
				
				// Publish confirmation response
				eventBus := app.genie.GetEventBus()
				eventBus.Publish("tool.confirmation.response", events.ToolConfirmationResponse{
					ExecutionID: executionID,
					Confirmed:   confirmed,
				})
				
				// Swap back to input component
				app.layoutManager.SetWindowComponent("input", app.inputComponent)
				
				// Re-render to update the view
				app.gui.Update(func(g *gocui.Gui) error {
					if err := app.inputComponent.Render(); err != nil {
						return err
					}
					// Focus back on input
					return app.focusViewByName("input")
				})
				
				return nil
			},
		)
		
		// Set up keybindings for confirmation component (only needs to be done once)
		for _, kb := range app.confirmationComponent.GetKeybindings() {
			if err := app.gui.SetKeybinding(kb.View, kb.Key, kb.Mod, kb.Handler); err != nil {
				return err
			}
		}
	} else {
		// Update existing confirmation component with new execution ID
		app.confirmationComponent.ExecutionID = event.ExecutionID
	}
	
	// Swap to confirmation component
	app.layoutManager.SetWindowComponent("input", app.confirmationComponent)
	
	// Apply secondary theme color to border and title after swap
	app.gui.Update(func(g *gocui.Gui) error {
		if view, err := g.View("input"); err == nil {
			// Get secondary color from theme
			theme := app.GetTheme()
			if theme != nil {
				// Convert secondary color to gocui color for border and title
				secondaryColor := presentation.ConvertAnsiToGocuiColor(theme.Secondary)
				view.FrameColor = secondaryColor
				view.TitleColor = secondaryColor
			}
		}
		return nil
	})
	
	// Refresh UI to show the message and swapped component
	if err := app.renderMessagesWithAutoScroll(); err != nil {
		return err
	}
	if err := app.confirmationComponent.Render(); err != nil {
		return err
	}
	
	// Focus the confirmation component (same view name as input)
	return app.focusViewByName("input")
}

func (app *App) handleUserConfirmationRequest(event events.UserConfirmationRequest) error {
	title := event.Title
	if title == "" {
		title = "Confirm Action"
	}
	
	message := event.Message
	if message == "" {
		if event.FilePath != "" {
			message = fmt.Sprintf("Do you want to proceed with changes to %s?", event.FilePath)
		} else {
			message = "Do you want to proceed?"
		}
	}
	
	confirmText := event.ConfirmText
	if confirmText == "" {
		confirmText = "Confirm"
	}
	
	cancelText := event.CancelText
	if cancelText == "" {
		cancelText = "Cancel"
	}
	
	onConfirm := func() error {
		// Publish confirmation response
		eventBus := app.genie.GetEventBus()
		eventBus.Publish("user.confirmation.response", events.UserConfirmationResponse{
			ExecutionID: event.ExecutionID,
			Confirmed:   true,
		})
		// Close the dialog after confirming
		return app.closeCurrentDialog()
	}
	
	onCancel := func() error {
		// Publish rejection response
		eventBus := app.genie.GetEventBus()
		eventBus.Publish("user.confirmation.response", events.UserConfirmationResponse{
			ExecutionID: event.ExecutionID,
			Confirmed:   false,
		})
		// Close the dialog after canceling
		return app.closeCurrentDialog()
	}
	
	onClose := func() error {
		return app.closeCurrentDialog()
	}
	
	return app.showConfirmationDialog(
		title, message, event.Content, event.ContentType,
		confirmText, cancelText,
		onConfirm, onCancel, onClose,
	)
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

