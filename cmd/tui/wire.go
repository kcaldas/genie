//go:build wireinject
// +build wireinject

//go:generate go run -mod=mod github.com/google/wire/cmd/tui/wire
package tui

import (
	"path/filepath"

	"github.com/awesome-gocui/gocui"
	"github.com/google/wire"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	internalDI "github.com/kcaldas/genie/internal/di"
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

func ProvideLayoutConfig(configManager *helpers.ConfigManager) *layout.LayoutConfig {
	config := configManager.GetConfig()
	return &layout.LayoutConfig{
		MessagesWeight: 3,                           // Messages panel weight (main content)
		InputHeight:    4,                           // Input panel height
		DebugWeight:    1,                           // Debug panel weight when shown
		StatusHeight:   2,                           // Status bar height
		ShowSidebar:    config.Layout.ShowSidebar,   // Keep legacy field
		CompactMode:    config.Layout.CompactMode,   // Keep compact mode
		MinPanelWidth:  config.Layout.MinPanelWidth, // Keep minimum constraints
		MinPanelHeight: config.Layout.MinPanelHeight,
	}
}

func ProvideLayoutManager(gui *gocui.Gui) (*layout.LayoutManager, error) {
	wire.Build(ProvideConfigManager, ProvideLayoutConfig, layout.NewLayoutManager)
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
