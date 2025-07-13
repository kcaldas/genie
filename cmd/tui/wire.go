//go:build wireinject
// +build wireinject

package tui

import (
	"path/filepath"

	"github.com/awesome-gocui/gocui"
	"github.com/google/wire"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/controllers"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	internalDI "github.com/kcaldas/genie/internal/di"
	pkgEvents "github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
)

// HistoryPath represents the path to the chat history file
type HistoryPath string

// Shared command event bus instance
var commandEventBus = events.NewCommandEventBus()

// Shared genie instance (singleton)
var (
	genieInstance    genie.Genie
	genieError       error
	genieInitialized bool
)

// Shared GUI instance (singleton)
var (
	guiInstance    *gocui.Gui
	guiError       error
	guiInitialized bool
)

// ProvideCommandEventBus provides a shared command event bus instance
func ProvideCommandEventBus() *events.CommandEventBus {
	return commandEventBus
}

// ProvideGenie provides a shared Genie singleton instance
func ProvideGenie() (genie.Genie, error) {
	if !genieInitialized {
		genieInstance, genieError = internalDI.ProvideGenie()
		genieInitialized = true
	}
	return genieInstance, genieError
}

func ProvideConfigManager() (*helpers.ConfigManager, error) {
	wire.Build(helpers.NewConfigManager)
	return nil, nil
}

func ProvideClipboard() *helpers.Clipboard {
	wire.Build(helpers.NewClipboard)
	return nil
}

// NewGocuiGui - Production GUI provider (uses config-based output mode)
func NewGocuiGui(configManager *helpers.ConfigManager) (*gocui.Gui, error) {
	config := configManager.GetConfig()
	guiOutputMode := configManager.GetGocuiOutputMode(config.OutputMode)

	g, err := gocui.NewGui(guiOutputMode, true)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// NewGocuiGuiWithOutputMode - Test/Custom GUI provider (accepts custom output mode)
func NewGocuiGuiWithOutputMode(outputMode gocui.OutputMode) (*gocui.Gui, error) {
	g, err := gocui.NewGui(outputMode, true)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// ProvideHistoryPath provides the chat history file path based on session working directory
func ProvideHistoryPath(session *genie.Session) HistoryPath {
	return HistoryPath(filepath.Join(session.WorkingDirectory, ".genie", "history"))
}

func ProvideChatState(configManager *helpers.ConfigManager) *state.ChatState {
	config := configManager.GetConfig()
	return state.NewChatState(config.MaxChatMessages)
}

func ProvideUIState() *state.UIState {
	return state.NewUIState()
}

func ProvideDebugState() *state.DebugState {
	return state.NewDebugState()
}

func ProvideStateAccessor(chatState *state.ChatState, uiState *state.UIState) *state.StateAccessor {
	return state.NewStateAccessor(chatState, uiState)
}

func ProvideTabHandler(app *App) types.TabHandler {
	return func(v *gocui.View) error {
		return app.nextView(app.gui, v)
	}
}

func ProvideGui(app *App) types.Gui {
	return &Gui{app: app}
}

func ProvideHistoryPathString(historyPath HistoryPath) string {
	return string(historyPath)
}

func ProvideEventBus(genieService genie.Genie) pkgEvents.EventBus {
	return genieService.GetEventBus()
}

func ProvideLayoutBuilder(
	gui *gocui.Gui,
	configManager *helpers.ConfigManager,
	messagesComponent *component.MessagesComponent,
	inputComponent *component.InputComponent,
	statusComponent *component.StatusComponent,
	textViewerComponent *component.TextViewerComponent,
	diffViewerComponent *component.DiffViewerComponent,
	debugComponent *component.DebugComponent,
) *LayoutBuilder {
	return NewLayoutBuilder(
		gui,
		configManager,
		messagesComponent,
		inputComponent,
		statusComponent,
		textViewerComponent,
		diffViewerComponent,
		debugComponent,
	)
}

func ProvideMessagesComponent(gui types.Gui, chatState *state.ChatState, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, tabHandler types.TabHandler) (*component.MessagesComponent, error) {
	wire.Build(component.NewMessagesComponent)
	return nil, nil
}

func ProvideInputComponent(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, historyPath HistoryPath, tabHandler types.TabHandler) (*component.InputComponent, error) {
	wire.Build(
		ProvideHistoryPathString,
		component.NewInputComponent,
	)
	return nil, nil
}

func ProvideStatusComponent(gui types.Gui, stateAccessor *state.StateAccessor, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus) (*component.StatusComponent, error) {
	wire.Build(
		wire.Bind(new(types.IStateAccessor), new(*state.StateAccessor)),
		component.NewStatusComponent,
	)
	return nil, nil
}

func ProvideTextViewerComponent(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, tabHandler types.TabHandler) (*component.TextViewerComponent, error) {
	wire.Build(
		wire.Value("Help"),
		component.NewTextViewerComponent,
	)
	return nil, nil
}

func ProvideDiffViewerComponent(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, tabHandler types.TabHandler) (*component.DiffViewerComponent, error) {
	wire.Build(
		wire.Value("Diff"),
		component.NewDiffViewerComponent,
	)
	return nil, nil
}

func ProvideDebugComponent(gui types.Gui, debugState *state.DebugState, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, tabHandler types.TabHandler) (*component.DebugComponent, error) {
	wire.Build(component.NewDebugComponent)
	return nil, nil
}

func ProvideDebugController(genieService genie.Genie, gui types.Gui, debugState *state.DebugState, debugComponent *component.DebugComponent, layoutManager *layout.LayoutManager, clipboard *helpers.Clipboard, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus) (*controllers.DebugController, error) {
	wire.Build(controllers.NewDebugController)
	return nil, nil
}

func ProvideChatController(messagesComponent *component.MessagesComponent, gui types.Gui, genieService genie.Genie, stateAccessor *state.StateAccessor, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, debugController *controllers.DebugController) (*controllers.ChatController, error) {
	wire.Build(
		wire.Bind(new(types.Component), new(*component.MessagesComponent)),
		wire.Bind(new(types.IStateAccessor), new(*state.StateAccessor)),
		wire.Bind(new(types.Logger), new(*controllers.DebugController)),
		controllers.NewChatController,
	)
	return nil, nil
}

func ProvideLLMContextController(gui types.Gui, genieService genie.Genie, layoutManager *layout.LayoutManager, stateAccessor *state.StateAccessor, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, debugController *controllers.DebugController) (*controllers.LLMContextController, error) {
	wire.Build(
		wire.Bind(new(types.IStateAccessor), new(*state.StateAccessor)),
		wire.Bind(new(types.Logger), new(*controllers.DebugController)),
		controllers.NewLLMContextController,
	)
	return nil, nil
}

func ProvideToolConfirmationController(gui types.Gui, stateAccessor *state.StateAccessor, layoutManager *layout.LayoutManager, inputComponent *component.InputComponent, configManager *helpers.ConfigManager, eventBus pkgEvents.EventBus, commandEventBus *events.CommandEventBus, debugController *controllers.DebugController) (*controllers.ToolConfirmationController, error) {
	wire.Build(
		wire.Bind(new(types.IStateAccessor), new(*state.StateAccessor)),
		wire.Bind(new(types.Component), new(*component.InputComponent)),
		wire.Bind(new(types.Logger), new(*controllers.DebugController)),
		controllers.NewToolConfirmationController,
	)
	return nil, nil
}

func ProvideUserConfirmationController(gui types.Gui, stateAccessor *state.StateAccessor, layoutManager *layout.LayoutManager, inputComponent *component.InputComponent, diffViewerComponent *component.DiffViewerComponent, configManager *helpers.ConfigManager, eventBus pkgEvents.EventBus, commandEventBus *events.CommandEventBus, debugController *controllers.DebugController) (*controllers.UserConfirmationController, error) {
	wire.Build(
		wire.Bind(new(types.IStateAccessor), new(*state.StateAccessor)),
		wire.Bind(new(types.Component), new(*component.InputComponent)),
		wire.Bind(new(types.Logger), new(*controllers.DebugController)),
		controllers.NewUserConfirmationController,
	)
	return nil, nil
}

// CoreDepsSet - Core dependencies (shared between production and test)
var CoreDepsSet = wire.NewSet(
	ProvideCommandEventBus,
)

// ProdAppDepsSet - Production app dependencies (includes config-based GUI)
var ProdAppDepsSet = wire.NewSet(
	CoreDepsSet,
	ProvideGenie,
	ProvideConfigManager,
	NewGocuiGui, // Uses config-based output mode
	NewApp,      // App now receives *gocui.Gui as dependency
)

// TestAppDepsSet - Test app dependencies (uses custom output mode GUI)
var TestAppDepsSet = wire.NewSet(
	CoreDepsSet,
	ProvideConfigManager,
	NewGocuiGuiWithOutputMode, // Uses provided output mode
)

// InjectTUI - Production TUI injector (default output mode from config)
func InjectTUI(session *genie.Session) (*TUI, error) {
	wire.Build(ProdAppDepsSet, New)
	return nil, nil
}

// InjectTestApp - Test App injector (custom output mode)
func InjectTestApp(genieService genie.Genie, session *genie.Session, outputMode gocui.OutputMode) (*App, error) {
	wire.Build(TestAppDepsSet, NewApp)
	return nil, nil
}
