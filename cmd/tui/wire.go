//go:build wireinject
// +build wireinject

package tui

import (
	"path/filepath"

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
	internalDI "github.com/kcaldas/genie/internal/di"
	pkgEvents "github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/logging"
)

// ============================================================================
// Type Definitions
// ============================================================================

// HistoryPath represents the path to the chat history file
type HistoryPath string

// ConfirmationInitializer is a marker type to ensure confirmation controllers are initialized
type ConfirmationInitializer struct{}

// ============================================================================
// Singleton Instances
// ============================================================================

// Shared command event bus instance
var commandEventBus = events.NewCommandEventBus()

// Shared genie instance (singleton)
var (
	genieInstance    genie.Genie
	genieError       error
	genieInitialized bool
)

// ============================================================================
// Core Service Providers
// ============================================================================

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

// ProvideEventBus extracts the event bus from the Genie service
func ProvideEventBus(genieService genie.Genie) pkgEvents.EventBus {
	return genieService.GetEventBus()
}

// ProvideSlashCommandManager provides a shared instance of SlashCommandManager
func ProvideSlashCommandManager() *slashcommands.Manager {
	return slashcommands.NewManager()
}

// ============================================================================
// Configuration and Helper Providers
// ============================================================================

func ProvideConfigManager() (*helpers.ConfigManager, error) {
	wire.Build(helpers.NewConfigManager)
	return nil, nil
}

func ProvideClipboard() *helpers.Clipboard {
	wire.Build(helpers.NewClipboard)
	return nil
}

// ProvideHistoryPath provides the chat history file path based on session working directory
func ProvideHistoryPath(session *genie.Session) HistoryPath {
	return HistoryPath(filepath.Join(session.WorkingDirectory, ".genie", "history"))
}

func ProvideHistoryPathString(historyPath HistoryPath) string {
	return string(historyPath)
}

// ============================================================================
// GUI Providers
// ============================================================================

// NewGocuiGui - Production GUI provider (uses config-based output mode)
func NewGocuiGui(configManager *helpers.ConfigManager) (*gocui.Gui, error) {
	config := configManager.GetConfig()
	guiOutputMode := configManager.GetGocuiOutputMode(config.OutputMode)

	g, err := gocui.NewGui(guiOutputMode, true)
	if err != nil {
		return nil, err
	}
	
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

// ============================================================================
// State Providers
// ============================================================================

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

// ============================================================================
// Layout Providers
// ============================================================================

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

// ============================================================================
// Component Providers
// ============================================================================

func ProvideMessagesComponent(gui types.Gui, chatState *state.ChatState, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus) (*component.MessagesComponent, error) {
	wire.Build(component.NewMessagesComponent)
	return nil, nil
}

func ProvideInputComponent(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, clipboard *helpers.Clipboard, historyPath HistoryPath, commandSuggester *shell.CommandSuggester, slashCommandSuggester *shell.SlashCommandSuggester) (*component.InputComponent, error) {
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

func ProvideTextViewerComponent(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus) (*component.TextViewerComponent, error) {
	wire.Build(
		wire.Value("Help"),
		component.NewTextViewerComponent,
	)
	return nil, nil
}

func ProvideDiffViewerComponent(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus) (*component.DiffViewerComponent, error) {
	wire.Build(
		wire.Value("Diff"),
		component.NewDiffViewerComponent,
	)
	return nil, nil
}

func ProvideDebugComponent(gui types.Gui, debugState *state.DebugState, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus) (*component.DebugComponent, error) {
	wire.Build(component.NewDebugComponent)
	return nil, nil
}

// ============================================================================
// Controller Providers
// ============================================================================

// ProvideGlobalLogger provides the global logger instance
func ProvideGlobalLogger() logging.Logger {
	return logging.GetGlobalLogger()
}

func ProvideDebugController(genieService genie.Genie, gui types.Gui, debugState *state.DebugState, debugComponent *component.DebugComponent, layoutManager *layout.LayoutManager, clipboard *helpers.Clipboard, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus) (*controllers.DebugController, error) {
	wire.Build(controllers.NewDebugController)
	return nil, nil
}

func ProvideChatController(messagesComponent *component.MessagesComponent, gui types.Gui, genieService genie.Genie, stateAccessor *state.StateAccessor, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus) (*controllers.ChatController, error) {
	wire.Build(
		wire.Bind(new(types.Component), new(*component.MessagesComponent)),
		wire.Bind(new(types.IStateAccessor), new(*state.StateAccessor)),
		controllers.NewChatController,
	)
	return nil, nil
}

func ProvideLLMContextController(gui types.Gui, genieService genie.Genie, layoutManager *layout.LayoutManager, stateAccessor *state.StateAccessor, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus) (*controllers.LLMContextController, error) {
	wire.Build(
		wire.Bind(new(types.IStateAccessor), new(*state.StateAccessor)),
		controllers.NewLLMContextController,
	)
	return nil, nil
}

func ProvideToolConfirmationController(gui types.Gui, stateAccessor *state.StateAccessor, layoutManager *layout.LayoutManager, inputComponent *component.InputComponent, configManager *helpers.ConfigManager, eventBus pkgEvents.EventBus, commandEventBus *events.CommandEventBus) (*controllers.ToolConfirmationController, error) {
	wire.Build(
		wire.Bind(new(types.IStateAccessor), new(*state.StateAccessor)),
		wire.Bind(new(types.Component), new(*component.InputComponent)),
		controllers.NewToolConfirmationController,
	)
	return nil, nil
}

func ProvideUserConfirmationController(gui types.Gui, stateAccessor *state.StateAccessor, layoutManager *layout.LayoutManager, inputComponent *component.InputComponent, diffViewerComponent *component.DiffViewerComponent, configManager *helpers.ConfigManager, eventBus pkgEvents.EventBus, commandEventBus *events.CommandEventBus) (*controllers.UserConfirmationController, error) {
	wire.Build(
		wire.Bind(new(types.IStateAccessor), new(*state.StateAccessor)),
		wire.Bind(new(types.Component), new(*component.InputComponent)),
		controllers.NewUserConfirmationController,
	)
	return nil, nil
}

func ProvideWriteController(gui types.Gui, configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, layoutManager *layout.LayoutManager) (*controllers.WriteController, error) {
	wire.Build(controllers.NewWriteController)
	return nil, nil
}

func ProvideSlashCommandController(commandEventBus *events.CommandEventBus, slashCommandManager *slashcommands.Manager, notification types.Notification) *controllers.SlashCommandController {
	return controllers.NewSlashCommandController(commandEventBus, slashCommandManager, notification)
}

// ============================================================================
// Command Providers
// ============================================================================

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

func ProvideExitCommand(commandEventBus *events.CommandEventBus) *commands.ExitCommand {
	return commands.NewExitCommand(commandEventBus)
}

func ProvideYankCommand(chatState *state.ChatState, clipboard *helpers.Clipboard, chatController *controllers.ChatController) *commands.YankCommand {
	return commands.NewYankCommand(chatState, clipboard, chatController)
}

func ProvideThemeCommand(configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, chatController *controllers.ChatController) *commands.ThemeCommand {
	return commands.NewThemeCommand(configManager, commandEventBus, chatController)
}

func ProvideConfigCommand(configManager *helpers.ConfigManager, commandEventBus *events.CommandEventBus, gui types.Gui, chatController *controllers.ChatController) *commands.ConfigCommand {
	return commands.NewConfigCommand(configManager, commandEventBus, gui, chatController)
}

func ProvideStatusCommand(chatController *controllers.ChatController, genieService genie.Genie) *commands.StatusCommand {
	return commands.NewStatusCommand(chatController, genieService)
}

func ProvideWriteCommand(writeController *controllers.WriteController) *commands.WriteCommand {
	return commands.NewWriteCommand(writeController)
}

func ProvideUpdateCommand(notification types.Notification) *commands.UpdateCommand {
	return commands.NewUpdateCommand(notification)
}

func ProvidePersonaCommand(notification types.Notification, genieService genie.Genie) *commands.PersonaCommand {
	return commands.NewPersonaCommand(notification, genieService)
}

func ProvideCommandHandler(
	commandEventBus *events.CommandEventBus, 
	chatController *controllers.ChatController, 
	registry *commands.CommandRegistry,
	contextCommand *commands.ContextCommand,
	clearCommand *commands.ClearCommand,
	debugCommand *commands.DebugCommand,
	exitCommand *commands.ExitCommand,
	yankCommand *commands.YankCommand,
	themeCommand *commands.ThemeCommand,
	configCommand *commands.ConfigCommand,
	statusCommand *commands.StatusCommand,
	writeCommand *commands.WriteCommand,
	updateCommand *commands.UpdateCommand,
	personaCommand *commands.PersonaCommand,
) *commands.CommandHandler {
	handler := commands.NewCommandHandler(commandEventBus, chatController, registry)

	// Register all commands (except help for now)
	// Order of registration doesn't matter functionally, but keeping alphabetical for readability
	handler.RegisterNewCommand(clearCommand)
	handler.RegisterNewCommand(configCommand)
	handler.RegisterNewCommand(contextCommand)
	handler.RegisterNewCommand(debugCommand)
	handler.RegisterNewCommand(exitCommand)
	handler.RegisterNewCommand(personaCommand)
	handler.RegisterNewCommand(statusCommand)
	handler.RegisterNewCommand(themeCommand)
	handler.RegisterNewCommand(updateCommand)
	handler.RegisterNewCommand(writeCommand)
	handler.RegisterNewCommand(yankCommand)

	return handler
}

// ============================================================================
// Side Effect Providers
// ============================================================================

// InitializeConfirmationControllers forces Wire to create confirmation controllers
// They will subscribe to events during construction but don't need to be held by anything
func InitializeConfirmationControllers(
	toolController *controllers.ToolConfirmationController,
	userController *controllers.UserConfirmationController,
	slashCommandController *controllers.SlashCommandController,
) *ConfirmationInitializer {
	// Controllers are created and have subscribed to events during construction
	// We don't need to do anything with them here
	return &ConfirmationInitializer{}
}

// ============================================================================
// Wire Sets - Organized Dependency Groups
// ============================================================================

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
	// Global logger provider
	ProvideGlobalLogger,

	// Core controllers
	ProvideDebugController,
	ProvideChatController,
	ProvideLLMContextController,
	ProvideWriteController,
	ProvideSlashCommandController,

	// Confirmation controllers
	ProvideToolConfirmationController,
	ProvideUserConfirmationController,
	InitializeConfirmationControllers,

	// Interface bindings
	wire.Bind(new(types.Notification), new(*controllers.ChatController)),
)

// CommandProvidersSet - All individual command providers
var CommandProvidersSet = wire.NewSet(
	ProvideContextCommand,
	ProvideClearCommand,
	ProvideDebugCommand,
	ProvideExitCommand,
	ProvideYankCommand,
	ProvideThemeCommand,
	ProvideConfigCommand,
	ProvideStatusCommand,
	ProvideWriteCommand,
	ProvideUpdateCommand,
	ProvidePersonaCommand,
)

// CommandSet - All commands and command handler
var CommandSet = wire.NewSet(
	// Command registry and suggesters
	ProvideCommandRegistry,
	ProvideCommandSuggester,
	ProvideSlashCommandSuggester,

	// All command providers
	CommandProvidersSet,

	// Command handler
	ProvideCommandHandler,
)

// GuiSet - GUI and interface types
var GuiSet = wire.NewSet(
	ProvideGui,
	ProvideHistoryPath,
	ProvideHistoryPathString,
)

// ============================================================================
// Aggregate Sets - High-level dependency composition
// ============================================================================

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

// ============================================================================
// Application Sets - Final composition for injection
// ============================================================================

// ProdAppDepsSet - Production app dependencies (includes config-based GUI)
var ProdAppDepsSet = wire.NewSet(
	CoreServicesSet,
	AllComponentsSet,
	AllControllersSet,
	NewGocuiGui, // Uses config-based output mode
	NewApp,      // App constructor
)

// TestAppDepsSet - Test app dependencies (uses custom output mode GUI)
var TestAppDepsSet = wire.NewSet(
	// Core services except Genie (provided as parameter)
	ProvideCommandEventBus,
	ProvideEventBus,
	ProvideConfigManager,
	ProvideClipboard,
	ProvideSlashCommandManager, // Add this line

	AllComponentsSet,
	AllControllersSet,
	NewGocuiGuiWithOutputMode, // Uses provided output mode
)

// ============================================================================
// Wire Injectors
// ============================================================================

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

