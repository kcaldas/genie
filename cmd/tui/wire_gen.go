// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package tui

import (
	"github.com/awesome-gocui/gocui"
	"github.com/google/wire"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/slashcommands"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/controllers"
	"github.com/kcaldas/genie/cmd/tui/controllers/commands"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/shell"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/internal/di"
	events2 "github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
	"path/filepath"
)

// Injectors from wire.go:

func ProvideConfigManager() (*helpers.ConfigManager, error) {
	configManager, err := helpers.NewConfigManager()
	if err != nil {
		return nil, err
	}
	return configManager, nil
}

func ProvideClipboard() *helpers.Clipboard {
	clipboard := helpers.NewClipboard()
	return clipboard
}

func ProvideMessagesComponent(gui types.Gui, chatState *state.ChatState, configManager *helpers.ConfigManager, commandEventBus2 *events.CommandEventBus) (*component.MessagesComponent, error) {
	messagesComponent := component.NewMessagesComponent(gui, chatState, configManager, commandEventBus2)
	return messagesComponent, nil
}

func ProvideInputComponent(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus2 *events.CommandEventBus, clipboard *helpers.Clipboard, historyPath HistoryPath, commandSuggester *shell.CommandSuggester, slashCommandSuggester *shell.SlashCommandSuggester) (*component.InputComponent, error) {
	string2 := ProvideHistoryPathString(historyPath)
	inputComponent := component.NewInputComponent(gui, configManager, commandEventBus2, clipboard, string2, commandSuggester, slashCommandSuggester)
	return inputComponent, nil
}

func ProvideStatusComponent(gui types.Gui, stateAccessor *state.StateAccessor, configManager *helpers.ConfigManager, commandEventBus2 *events.CommandEventBus) (*component.StatusComponent, error) {
	statusComponent := component.NewStatusComponent(gui, stateAccessor, configManager, commandEventBus2)
	return statusComponent, nil
}

func ProvideTextViewerComponent(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus2 *events.CommandEventBus) (*component.TextViewerComponent, error) {
	string2 := _wireStringValue
	textViewerComponent := component.NewTextViewerComponent(gui, string2, configManager, commandEventBus2)
	return textViewerComponent, nil
}

var (
	_wireStringValue = "Help"
)

func ProvideDiffViewerComponent(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus2 *events.CommandEventBus) (*component.DiffViewerComponent, error) {
	string2 := _wireStringValue2
	diffViewerComponent := component.NewDiffViewerComponent(gui, string2, configManager, commandEventBus2)
	return diffViewerComponent, nil
}

var (
	_wireStringValue2 = "Diff"
)

func ProvideDebugComponent(gui types.Gui, debugState *state.DebugState, configManager *helpers.ConfigManager, commandEventBus2 *events.CommandEventBus) (*component.DebugComponent, error) {
	debugComponent := component.NewDebugComponent(gui, debugState, configManager, commandEventBus2)
	return debugComponent, nil
}

func ProvideDebugController(genieService genie.Genie, gui types.Gui, debugState *state.DebugState, debugComponent *component.DebugComponent, layoutManager *layout.LayoutManager, clipboard *helpers.Clipboard, configManager *helpers.ConfigManager, commandEventBus2 *events.CommandEventBus) (*controllers.DebugController, error) {
	debugController := controllers.NewDebugController(genieService, gui, debugState, debugComponent, layoutManager, clipboard, configManager, commandEventBus2)
	return debugController, nil
}

func ProvideChatController(messagesComponent *component.MessagesComponent, gui types.Gui, genieService genie.Genie, stateAccessor *state.StateAccessor, configManager *helpers.ConfigManager, commandEventBus2 *events.CommandEventBus) (*controllers.ChatController, error) {
	chatController := controllers.NewChatController(messagesComponent, gui, genieService, stateAccessor, configManager, commandEventBus2)
	return chatController, nil
}

func ProvideLLMContextController(gui types.Gui, genieService genie.Genie, layoutManager *layout.LayoutManager, stateAccessor *state.StateAccessor, configManager *helpers.ConfigManager, commandEventBus2 *events.CommandEventBus) (*controllers.LLMContextController, error) {
	llmContextController := controllers.NewLLMContextController(gui, genieService, layoutManager, stateAccessor, configManager, commandEventBus2)
	return llmContextController, nil
}

func ProvideToolConfirmationController(gui types.Gui, stateAccessor *state.StateAccessor, layoutManager *layout.LayoutManager, inputComponent *component.InputComponent, configManager *helpers.ConfigManager, eventBus events2.EventBus, commandEventBus2 *events.CommandEventBus) (*controllers.ToolConfirmationController, error) {
	toolConfirmationController := controllers.NewToolConfirmationController(gui, stateAccessor, layoutManager, inputComponent, configManager, eventBus, commandEventBus2)
	return toolConfirmationController, nil
}

func ProvideUserConfirmationController(gui types.Gui, stateAccessor *state.StateAccessor, layoutManager *layout.LayoutManager, inputComponent *component.InputComponent, diffViewerComponent *component.DiffViewerComponent, configManager *helpers.ConfigManager, eventBus events2.EventBus, commandEventBus2 *events.CommandEventBus) (*controllers.UserConfirmationController, error) {
	userConfirmationController := controllers.NewUserConfirmationController(gui, stateAccessor, layoutManager, inputComponent, diffViewerComponent, configManager, eventBus, commandEventBus2)
	return userConfirmationController, nil
}

func ProvideWriteController(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus2 *events.CommandEventBus, layoutManager *layout.LayoutManager) (*controllers.WriteController, error) {
	writeController := controllers.NewWriteController(gui, configManager, commandEventBus2, layoutManager)
	return writeController, nil
}

// InjectTUI - Production TUI injector (default output mode from config)
func InjectTUI(session *genie.Session) (*TUI, error) {
	configManager, err := ProvideConfigManager()
	if err != nil {
		return nil, err
	}
	gui, err := NewGocuiGui(configManager)
	if err != nil {
		return nil, err
	}
	typesGui := ProvideGui(gui)
	eventsCommandEventBus := ProvideCommandEventBus()
	chatState := ProvideChatState(configManager)
	messagesComponent, err := ProvideMessagesComponent(typesGui, chatState, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	clipboard := ProvideClipboard()
	historyPath := ProvideHistoryPath(session)
	commandRegistry := ProvideCommandRegistry()
	commandSuggester := ProvideCommandSuggester(commandRegistry)
	manager := ProvideSlashCommandManager()
	slashCommandSuggester := ProvideSlashCommandSuggester(manager)
	inputComponent, err := ProvideInputComponent(typesGui, configManager, eventsCommandEventBus, clipboard, historyPath, commandSuggester, slashCommandSuggester)
	if err != nil {
		return nil, err
	}
	uiState := ProvideUIState()
	stateAccessor := ProvideStateAccessor(chatState, uiState)
	statusComponent, err := ProvideStatusComponent(typesGui, stateAccessor, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	textViewerComponent, err := ProvideTextViewerComponent(typesGui, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	diffViewerComponent, err := ProvideDiffViewerComponent(typesGui, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	debugState := ProvideDebugState()
	debugComponent, err := ProvideDebugComponent(typesGui, debugState, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	layoutBuilder := ProvideLayoutBuilder(gui, configManager, messagesComponent, inputComponent, statusComponent, textViewerComponent, diffViewerComponent, debugComponent)
	layoutManager := ProvideLayoutManager(layoutBuilder)
	genieGenie, err := ProvideGenie()
	if err != nil {
		return nil, err
	}
	chatController, err := ProvideChatController(messagesComponent, typesGui, genieGenie, stateAccessor, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	llmContextController, err := ProvideLLMContextController(typesGui, genieGenie, layoutManager, stateAccessor, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	contextCommand := ProvideContextCommand(llmContextController)
	clearCommand := ProvideClearCommand(chatController)
	debugController, err := ProvideDebugController(genieGenie, typesGui, debugState, debugComponent, layoutManager, clipboard, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	debugCommand := ProvideDebugCommand(debugController, chatController)
	exitCommand := ProvideExitCommand(eventsCommandEventBus)
	yankCommand := ProvideYankCommand(chatState, clipboard, chatController)
	themeCommand := ProvideThemeCommand(configManager, eventsCommandEventBus, chatController)
	configCommand := ProvideConfigCommand(configManager, eventsCommandEventBus, typesGui, chatController)
	statusCommand := ProvideStatusCommand(chatController, genieGenie)
	writeController, err := ProvideWriteController(typesGui, configManager, eventsCommandEventBus, layoutManager)
	if err != nil {
		return nil, err
	}
	writeCommand := ProvideWriteCommand(writeController)
	commandHandler := ProvideCommandHandler(eventsCommandEventBus, chatController, commandRegistry, contextCommand, clearCommand, debugCommand, exitCommand, yankCommand, themeCommand, configCommand, statusCommand, writeCommand)
	eventBus := ProvideEventBus(genieGenie)
	toolConfirmationController, err := ProvideToolConfirmationController(typesGui, stateAccessor, layoutManager, inputComponent, configManager, eventBus, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	userConfirmationController, err := ProvideUserConfirmationController(typesGui, stateAccessor, layoutManager, inputComponent, diffViewerComponent, configManager, eventBus, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	slashCommandController := ProvideSlashCommandController(eventsCommandEventBus, manager, chatController)
	confirmationInitializer := InitializeConfirmationControllers(toolConfirmationController, userConfirmationController, slashCommandController)
	app, err := NewApp(typesGui, eventsCommandEventBus, configManager, layoutManager, commandHandler, chatController, uiState, confirmationInitializer, manager)
	if err != nil {
		return nil, err
	}
	tui := New(app)
	return tui, nil
}

// InjectTestApp - Test App injector (custom output mode)
func InjectTestApp(genieService genie.Genie, session *genie.Session, outputMode gocui.OutputMode) (*App, error) {
	gui, err := NewGocuiGuiWithOutputMode(outputMode)
	if err != nil {
		return nil, err
	}
	typesGui := ProvideGui(gui)
	eventsCommandEventBus := ProvideCommandEventBus()
	configManager, err := ProvideConfigManager()
	if err != nil {
		return nil, err
	}
	chatState := ProvideChatState(configManager)
	messagesComponent, err := ProvideMessagesComponent(typesGui, chatState, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	clipboard := ProvideClipboard()
	historyPath := ProvideHistoryPath(session)
	commandRegistry := ProvideCommandRegistry()
	commandSuggester := ProvideCommandSuggester(commandRegistry)
	manager := ProvideSlashCommandManager()
	slashCommandSuggester := ProvideSlashCommandSuggester(manager)
	inputComponent, err := ProvideInputComponent(typesGui, configManager, eventsCommandEventBus, clipboard, historyPath, commandSuggester, slashCommandSuggester)
	if err != nil {
		return nil, err
	}
	uiState := ProvideUIState()
	stateAccessor := ProvideStateAccessor(chatState, uiState)
	statusComponent, err := ProvideStatusComponent(typesGui, stateAccessor, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	textViewerComponent, err := ProvideTextViewerComponent(typesGui, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	diffViewerComponent, err := ProvideDiffViewerComponent(typesGui, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	debugState := ProvideDebugState()
	debugComponent, err := ProvideDebugComponent(typesGui, debugState, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	layoutBuilder := ProvideLayoutBuilder(gui, configManager, messagesComponent, inputComponent, statusComponent, textViewerComponent, diffViewerComponent, debugComponent)
	layoutManager := ProvideLayoutManager(layoutBuilder)
	chatController, err := ProvideChatController(messagesComponent, typesGui, genieService, stateAccessor, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	llmContextController, err := ProvideLLMContextController(typesGui, genieService, layoutManager, stateAccessor, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	contextCommand := ProvideContextCommand(llmContextController)
	clearCommand := ProvideClearCommand(chatController)
	debugController, err := ProvideDebugController(genieService, typesGui, debugState, debugComponent, layoutManager, clipboard, configManager, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	debugCommand := ProvideDebugCommand(debugController, chatController)
	exitCommand := ProvideExitCommand(eventsCommandEventBus)
	yankCommand := ProvideYankCommand(chatState, clipboard, chatController)
	themeCommand := ProvideThemeCommand(configManager, eventsCommandEventBus, chatController)
	configCommand := ProvideConfigCommand(configManager, eventsCommandEventBus, typesGui, chatController)
	statusCommand := ProvideStatusCommand(chatController, genieService)
	writeController, err := ProvideWriteController(typesGui, configManager, eventsCommandEventBus, layoutManager)
	if err != nil {
		return nil, err
	}
	writeCommand := ProvideWriteCommand(writeController)
	commandHandler := ProvideCommandHandler(eventsCommandEventBus, chatController, commandRegistry, contextCommand, clearCommand, debugCommand, exitCommand, yankCommand, themeCommand, configCommand, statusCommand, writeCommand)
	eventBus := ProvideEventBus(genieService)
	toolConfirmationController, err := ProvideToolConfirmationController(typesGui, stateAccessor, layoutManager, inputComponent, configManager, eventBus, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	userConfirmationController, err := ProvideUserConfirmationController(typesGui, stateAccessor, layoutManager, inputComponent, diffViewerComponent, configManager, eventBus, eventsCommandEventBus)
	if err != nil {
		return nil, err
	}
	slashCommandController := ProvideSlashCommandController(eventsCommandEventBus, manager, chatController)
	confirmationInitializer := InitializeConfirmationControllers(toolConfirmationController, userConfirmationController, slashCommandController)
	app, err := NewApp(typesGui, eventsCommandEventBus, configManager, layoutManager, commandHandler, chatController, uiState, confirmationInitializer, manager)
	if err != nil {
		return nil, err
	}
	return app, nil
}

// wire.go:

// HistoryPath represents the path to the chat history file
type HistoryPath string

// ConfirmationInitializer is a marker type to ensure confirmation controllers are initialized
type ConfirmationInitializer struct{}

// Shared command event bus instance
var commandEventBus = events.NewCommandEventBus()

// Shared genie instance (singleton)
var (
	genieInstance    genie.Genie
	genieError       error
	genieInitialized bool
)

// ProvideCommandEventBus provides a shared command event bus instance
func ProvideCommandEventBus() *events.CommandEventBus {
	return commandEventBus
}

// ProvideGenie provides a shared Genie singleton instance
func ProvideGenie() (genie.Genie, error) {
	if !genieInitialized {
		genieInstance, genieError = di.ProvideGenie()
		genieInitialized = true
	}
	return genieInstance, genieError
}

// ProvideEventBus extracts the event bus from the Genie service
func ProvideEventBus(genieService genie.Genie) events2.EventBus {
	return genieService.GetEventBus()
}

// ProvideSlashCommandManager provides a shared instance of SlashCommandManager
func ProvideSlashCommandManager() *slashcommands.Manager {
	return slashcommands.NewManager()
}

// ProvideHistoryPath provides the chat history file path based on session working directory
func ProvideHistoryPath(session *genie.Session) HistoryPath {
	return HistoryPath(filepath.Join(session.WorkingDirectory, ".genie", "history"))
}

func ProvideHistoryPathString(historyPath HistoryPath) string {
	return string(historyPath)
}

// NewGocuiGui - Production GUI provider (uses config-based output mode)
func NewGocuiGui(configManager *helpers.ConfigManager) (*gocui.Gui, error) {
	config := configManager.GetConfig()
	guiOutputMode := configManager.GetGocuiOutputMode(config.OutputMode)

	g, err := gocui.NewGui(guiOutputMode, true)
	if err != nil {
		return nil, err
	}

	logger := logging.GetGlobalLogger()
	logger.Info("DEBUG: EnableMouse config = %v", config.EnableMouse)
	logger.Info("DEBUG: Theme config = %s", config.Theme)
	logger.Info("DEBUG: Setting g.Mouse = %v", config.IsMouseEnabled())

	g.Mouse = config.IsMouseEnabled()
	return g, nil
}

// NewGocuiGuiWithOutputMode - Test/Custom GUI provider (accepts custom output mode)
func NewGocuiGuiWithOutputMode(outputMode gocui.OutputMode) (*gocui.Gui, error) {
	g, err := gocui.NewGui(outputMode, true)
	if err != nil {
		return nil, err
	}
	g.Mouse = true
	return g, nil
}

func ProvideGui(gui *gocui.Gui) types.Gui {
	return &Gui{gui: gui}
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

func ProvideLayoutManager(layoutBuilder *LayoutBuilder) *layout.LayoutManager {
	return layoutBuilder.GetLayoutManager()
}

// ProvideGlobalLogger provides the global logger instance
func ProvideGlobalLogger() logging.Logger {
	return logging.GetGlobalLogger()
}

func ProvideSlashCommandController(commandEventBus2 *events.CommandEventBus, slashCommandManager *slashcommands.Manager, notification types.Notification) *controllers.SlashCommandController {
	return controllers.NewSlashCommandController(commandEventBus2, slashCommandManager, notification)
}

func ProvideCommandRegistry() *commands.CommandRegistry {
	return commands.NewCommandRegistry()
}

func ProvideCommandSuggester(registry *commands.CommandRegistry) *shell.CommandSuggester {
	return shell.NewCommandSuggester(registry)
}

func ProvideSlashCommandSuggester(manager *slashcommands.Manager) *shell.SlashCommandSuggester {
	return shell.NewSlashCommandSuggester(manager)
}

func ProvideContextCommand(llmContextController *controllers.LLMContextController) *commands.ContextCommand {
	return commands.NewContextCommand(llmContextController)
}

func ProvideClearCommand(chatController *controllers.ChatController) *commands.ClearCommand {
	return commands.NewClearCommand(chatController)
}

func ProvideDebugCommand(debugController *controllers.DebugController, chatController *controllers.ChatController) *commands.DebugCommand {
	return commands.NewDebugCommand(debugController, chatController)
}

func ProvideExitCommand(commandEventBus2 *events.CommandEventBus) *commands.ExitCommand {
	return commands.NewExitCommand(commandEventBus2)
}

func ProvideYankCommand(chatState *state.ChatState, clipboard *helpers.Clipboard, chatController *controllers.ChatController) *commands.YankCommand {
	return commands.NewYankCommand(chatState, clipboard, chatController)
}

func ProvideThemeCommand(configManager *helpers.ConfigManager, commandEventBus2 *events.CommandEventBus, chatController *controllers.ChatController) *commands.ThemeCommand {
	return commands.NewThemeCommand(configManager, commandEventBus2, chatController)
}

func ProvideConfigCommand(configManager *helpers.ConfigManager, commandEventBus2 *events.CommandEventBus, gui types.Gui, chatController *controllers.ChatController) *commands.ConfigCommand {
	return commands.NewConfigCommand(configManager, commandEventBus2, gui, chatController)
}

func ProvideStatusCommand(chatController *controllers.ChatController, genieService genie.Genie) *commands.StatusCommand {
	return commands.NewStatusCommand(chatController, genieService)
}

func ProvideWriteCommand(writeController *controllers.WriteController) *commands.WriteCommand {
	return commands.NewWriteCommand(writeController)
}

func ProvideCommandHandler(commandEventBus2 *events.CommandEventBus, chatController *controllers.ChatController, registry *commands.CommandRegistry, contextCommand *commands.ContextCommand, clearCommand *commands.ClearCommand, debugCommand *commands.DebugCommand, exitCommand *commands.ExitCommand, yankCommand *commands.YankCommand, themeCommand *commands.ThemeCommand, configCommand *commands.ConfigCommand, statusCommand *commands.StatusCommand, writeCommand *commands.WriteCommand) *commands.CommandHandler {
	handler := commands.NewCommandHandler(commandEventBus2, chatController, registry)

	handler.RegisterNewCommand(contextCommand)
	handler.RegisterNewCommand(clearCommand)
	handler.RegisterNewCommand(debugCommand)
	handler.RegisterNewCommand(exitCommand)
	handler.RegisterNewCommand(yankCommand)
	handler.RegisterNewCommand(themeCommand)
	handler.RegisterNewCommand(configCommand)
	handler.RegisterNewCommand(statusCommand)
	handler.RegisterNewCommand(writeCommand)

	return handler
}

// InitializeConfirmationControllers forces Wire to create confirmation controllers
// They will subscribe to events during construction but don't need to be held by anything
func InitializeConfirmationControllers(
	toolController *controllers.ToolConfirmationController,
	userController *controllers.UserConfirmationController,
	slashCommandController *controllers.SlashCommandController,
) *ConfirmationInitializer {

	return &ConfirmationInitializer{}
}

// StateSet - All state management
var StateSet = wire.NewSet(
	ProvideChatState,
	ProvideUIState,
	ProvideDebugState,
	ProvideStateAccessor,
)

// ComponentSet - UI components
var ComponentSet = wire.NewSet(
	ProvideMessagesComponent,
	ProvideInputComponent,
	ProvideStatusComponent,
	ProvideTextViewerComponent,
	ProvideDiffViewerComponent,
	ProvideDebugComponent,
)

// LayoutSet - Layout management
var LayoutSet = wire.NewSet(
	ProvideLayoutBuilder,
	ProvideLayoutManager,
)

// ControllerSet - Controllers with interface bindings
var ControllerSet = wire.NewSet(

	ProvideGlobalLogger,

	ProvideDebugController,
	ProvideChatController,
	ProvideLLMContextController,
	ProvideWriteController,
	ProvideSlashCommandController,

	ProvideToolConfirmationController,
	ProvideUserConfirmationController,
	InitializeConfirmationControllers, wire.Bind(new(types.Notification), new(*controllers.ChatController)),
)

// CommandSet - All commands and command handler
var CommandSet = wire.NewSet(

	ProvideCommandRegistry,
	ProvideCommandSuggester,
	ProvideSlashCommandSuggester,

	ProvideContextCommand,
	ProvideClearCommand,
	ProvideDebugCommand,
	ProvideExitCommand,
	ProvideYankCommand,
	ProvideThemeCommand,
	ProvideConfigCommand,
	ProvideStatusCommand,
	ProvideWriteCommand,

	ProvideCommandHandler,
)

// GuiSet - GUI and interface types
var GuiSet = wire.NewSet(
	ProvideGui,
	ProvideHistoryPath,
	ProvideHistoryPathString,
)

// CoreServicesSet - Core services and dependencies
var CoreServicesSet = wire.NewSet(
	ProvideCommandEventBus,
	ProvideGenie,
	ProvideEventBus,
	ProvideConfigManager,
	ProvideClipboard,
	ProvideSlashCommandManager,
)

// AllComponentsSet - All UI components and layout
var AllComponentsSet = wire.NewSet(
	StateSet,
	ComponentSet,
	LayoutSet,
	GuiSet,
)

// AllControllersSet - All controllers and commands
var AllControllersSet = wire.NewSet(
	ControllerSet,
	CommandSet,
)

// ProdAppDepsSet - Production app dependencies (includes config-based GUI)
var ProdAppDepsSet = wire.NewSet(
	CoreServicesSet,
	AllComponentsSet,
	AllControllersSet,
	NewGocuiGui,
	NewApp,
)

// TestAppDepsSet - Test app dependencies (uses custom output mode GUI)
var TestAppDepsSet = wire.NewSet(

	ProvideCommandEventBus,
	ProvideEventBus,
	ProvideConfigManager,
	ProvideClipboard,
	ProvideSlashCommandManager,

	AllComponentsSet,
	AllControllersSet,
	NewGocuiGuiWithOutputMode,
)
