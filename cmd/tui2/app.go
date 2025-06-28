package tui2

import (
	"fmt"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/kcaldas/genie/cmd/tui2/component"
	"github.com/kcaldas/genie/cmd/tui2/controllers"
	"github.com/kcaldas/genie/cmd/tui2/helpers"
	"github.com/kcaldas/genie/cmd/tui2/layout"
	"github.com/kcaldas/genie/cmd/tui2/presentation"
	"github.com/kcaldas/genie/cmd/tui2/state"
	"github.com/kcaldas/genie/cmd/tui2/types"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
)

type App struct {
	gui     *gocui.Gui
	genie   genie.Genie
	helpers *helpers.Helpers
	
	chatState     *state.ChatState
	uiState       *state.UIState
	stateAccessor *state.StateAccessor
	
	messageFormatter *presentation.MessageFormatter
	layoutManager    *layout.LayoutManager
	
	messagesCtx *component.MessagesComponent
	inputCtx    *component.InputComponent
	debugCtx    *component.DebugComponent
	statusCtx   *component.StatusComponent
	
	chatController *controllers.ChatController
	commandHandler *controllers.SlashCommandHandler
}

func NewApp(genieService genie.Genie) (*App, error) {
	g, err := gocui.NewGui(gocui.OutputNormal, true)
	if err != nil {
		return nil, err
	}
	
	helpers, err := helpers.NewHelpers()
	if err != nil {
		g.Close()
		return nil, err
	}
	
	config, err := helpers.Config.Load()
	if err != nil {
		g.Close()
		return nil, err
	}
	
	app := &App{
		gui:       g,
		genie:     genieService,
		helpers:   helpers,
		chatState: state.NewChatState(),
		uiState:   state.NewUIState(config),
	}
	
	app.stateAccessor = state.NewStateAccessor(app.chatState, app.uiState)
	
	layoutConfig := &layout.LayoutConfig{
		MessagesWeight: 3,                            // Messages panel weight (main content)
		InputHeight:    4,                            // Input panel height
		DebugWeight:    1,                            // Debug panel weight when shown
		StatusHeight:   2,                            // Status bar height
		ShowSidebar:    config.Layout.ShowSidebar,    // Keep legacy field
		CompactMode:    config.Layout.CompactMode,    // Keep compact mode
		MinPanelWidth:  config.Layout.MinPanelWidth,  // Keep minimum constraints
		MinPanelHeight: config.Layout.MinPanelHeight,
	}
	app.layoutManager = layout.NewLayoutManager(g, layoutConfig)
	
	theme := presentation.GetTheme(config.Theme)
	app.messageFormatter, err = presentation.NewMessageFormatter(config, theme)
	if err != nil {
		g.Close()
		return nil, err
	}
	
	if err := app.setupComponentsAndControllers(); err != nil {
		g.Close()
		return nil, err
	}
	
	app.setupEventSubscriptions()
	
	g.SetManagerFunc(app.layoutManager.Layout)
	
	if err := app.setupKeybindings(); err != nil {
		g.Close()
		return nil, err
	}
	
	g.Cursor = true  // Force cursor enabled for debugging
	g.SelFgColor = gocui.ColorBlack
	g.SelBgColor = gocui.ColorCyan
	
	return app, nil
}

func (app *App) setupComponentsAndControllers() error {
	guiCommon := &guiCommon{app: app}
	
	app.messagesCtx = component.NewMessagesComponent(guiCommon, app.stateAccessor, app.messageFormatter)
	app.inputCtx = component.NewInputComponent(guiCommon, app.handleUserInput)
	app.debugCtx = component.NewDebugComponent(guiCommon, app.stateAccessor)
	app.statusCtx = component.NewStatusComponent(guiCommon, app.stateAccessor)
	
	// Map components to 5-panel layout
	app.layoutManager.SetWindowComponent("center", app.messagesCtx)  // messages in center
	app.layoutManager.SetWindowComponent("bottom", app.inputCtx)     // input at bottom
	app.layoutManager.SetWindowComponent("right", app.debugCtx)      // debug on right side
	app.layoutManager.SetWindowComponent("top", app.statusCtx)       // status at top
	
	app.commandHandler = controllers.NewSlashCommandHandler()
	app.setupCommands()
	
	app.chatController = controllers.NewChatController(
		app.messagesCtx,
		guiCommon,
		app.genie,
		app.stateAccessor,
		app.commandHandler,
	)
	
	
	return nil
}

func (app *App) setupCommands() {
	app.commandHandler.RegisterCommand("help", app.cmdHelp)
	app.commandHandler.RegisterCommand("clear", app.cmdClear)
	app.commandHandler.RegisterCommand("debug", app.cmdDebug)
	app.commandHandler.RegisterCommand("config", app.cmdConfig)
	app.commandHandler.RegisterCommand("exit", app.cmdExit)
	app.commandHandler.RegisterCommand("quit", app.cmdExit)
	app.commandHandler.RegisterCommand("theme", app.cmdTheme)
	app.commandHandler.RegisterCommand("focus", app.cmdFocus)
	app.commandHandler.RegisterCommand("toggle", app.cmdToggle)
	app.commandHandler.RegisterCommand("layout", app.cmdLayout)
	
	app.commandHandler.RegisterAlias("h", "help")
	app.commandHandler.RegisterAlias("q", "quit")
	app.commandHandler.RegisterAlias("cls", "clear")
	app.commandHandler.RegisterAlias("f", "focus")
	app.commandHandler.RegisterAlias("t", "toggle")
}


func (app *App) setupKeybindings() error {
	// Setup keybindings for all components
	components := []types.Component{app.messagesCtx, app.inputCtx, app.debugCtx, app.statusCtx}
	for _, ctx := range components {
		for _, kb := range ctx.GetKeybindings() {
			if err := app.gui.SetKeybinding(kb.View, kb.Key, kb.Mod, kb.Handler); err != nil {
				return err
			}
		}
	}
	
	if err := app.gui.SetKeybinding("", gocui.KeyTab, gocui.ModNone, app.nextView); err != nil {
		return err
	}
	
	if err := app.gui.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, app.quit); err != nil {
		return err
	}
	
	return nil
}

func (app *App) Run() error {
	// Add welcome message first
	app.showWelcomeMessage()
	
	// Initial render
	if err := app.messagesCtx.Render(); err != nil {
		return err
	}
	
	if err := app.statusCtx.Render(); err != nil {
		return err
	}
	
	// Set focus to input after everything is set up
	if _, err := app.gui.SetCurrentView("bottom"); err == nil {  // Use new panel name
		// Force input properties after focus is set
		app.gui.Update(func(g *gocui.Gui) error {
			if inputView, err := g.View("bottom"); err == nil && inputView != nil {
				inputView.Editable = true
				inputView.SetCursor(0, 0)
			}
			return nil
		})
	}
	
	// Start periodic status updates
	app.startStatusUpdates()
	
	return app.gui.MainLoop()
}

func (app *App) startStatusUpdates() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			app.gui.Update(func(g *gocui.Gui) error {
				return app.statusCtx.Render()
			})
		}
	}()
}

func (app *App) Close() {
	if app.gui != nil {
		app.gui.Close()
	}
}

func (app *App) handleUserInput(input types.UserInput) error {
	return app.chatController.HandleInput(input.Message)
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
	case "center":
		return types.PanelMessages
	case "bottom":
		return types.PanelInput
	case "right":
		return types.PanelDebug
	default:
		return types.PanelInput
	}
}

func (app *App) panelToComponent(panel types.FocusablePanel) types.Component {
	switch panel {
	case types.PanelMessages:
		return app.layoutManager.GetWindowComponent("center")
	case types.PanelInput:
		return app.layoutManager.GetWindowComponent("bottom")
	case types.PanelDebug:
		return app.layoutManager.GetWindowComponent("right")
	default:
		return nil
	}
}

func (app *App) nextView(g *gocui.Gui, v *gocui.View) error {
	// Get available panels from layout manager
	availablePanels := app.layoutManager.GetAvailablePanels()
	views := []string{}
	
	// Prioritize input and messages panels, then add others
	for _, panel := range []string{"bottom", "center", "top", "right", "left"} {
		for _, available := range availablePanels {
			if panel == available {
				// Only add debug panel if it's visible
				if panel == "right" && !app.debugCtx.IsVisible() {
					continue
				}
				views = append(views, panel)
				break
			}
		}
	}
	
	currentView := g.CurrentView()
	if currentView == nil {
		return app.setCurrentView(views[0])
	}
	
	currentName := currentView.Name()
	for i, name := range views {
		if name == currentName {
			nextIndex := (i + 1) % len(views)
			return app.setCurrentView(views[nextIndex])
		}
	}
	
	return app.setCurrentView(views[0])
}

func (app *App) quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (app *App) setupEventSubscriptions() {
	eventBus := app.genie.GetEventBus()
	
	// Subscribe to chat response events
	eventBus.Subscribe("chat.response", func(e interface{}) {
		if event, ok := e.(events.ChatResponseEvent); ok {
			app.gui.Update(func(g *gocui.Gui) error {
				app.stateAccessor.SetLoading(false)
				
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
				
				return app.messagesCtx.Render()
			})
		}
	})
	
	// Subscribe to chat started events
	eventBus.Subscribe("chat.started", func(e interface{}) {
		if _, ok := e.(events.ChatStartedEvent); ok {
			app.gui.Update(func(g *gocui.Gui) error {
				app.stateAccessor.SetLoading(true)
				return app.messagesCtx.Render()
			})
		}
	})
	
	// Subscribe to tool call events for debug panel
	eventBus.Subscribe("tool.call", func(e interface{}) {
		if event, ok := e.(events.ToolCallEvent); ok {
			debugMsg := fmt.Sprintf("[TOOL CALL] %s: %v", event.ToolName, event.Parameters)
			app.gui.Update(func(g *gocui.Gui) error {
				app.stateAccessor.AddDebugMessage(debugMsg)
				if app.debugCtx.IsVisible() {
					return app.debugCtx.Render()
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
				if app.debugCtx.IsVisible() {
					return app.debugCtx.Render()
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
				return app.messagesCtx.Render()
			})
		}
	})
}

func (app *App) showWelcomeMessage() {
	app.stateAccessor.AddMessage(types.Message{
		Role:    "system",
		Content: "Welcome to Genie TUI! Type /help for available commands.",
	})
	
	app.gui.Update(func(g *gocui.Gui) error {
		return app.messagesCtx.Render()
	})
}