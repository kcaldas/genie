//go:build wireinject
// +build wireinject

//
//go:generate go run -mod=mod github.com/google/wire/cmd/wire
package tui

import (
	"github.com/awesome-gocui/gocui"
	"github.com/google/wire"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	internalDI "github.com/kcaldas/genie/internal/di"
	"github.com/kcaldas/genie/pkg/genie"
)

// Shared command event bus instance
var commandEventBus = events.NewCommandEventBus()

// Shared genie instance (singleton)
var genieInstance genie.Genie
var genieError error
var genieInitialized bool

// Shared GUI instance (singleton)
var guiInstance *gocui.Gui
var guiError error
var guiInitialized bool

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

// GUI providers
// Production GUI provider (uses config-based output mode)
func NewGocuiGui(configManager *helpers.ConfigManager) (*gocui.Gui, error) {
	config := configManager.GetConfig()
	guiOutputMode := configManager.GetGocuiOutputMode(config.OutputMode)

	g, err := gocui.NewGui(guiOutputMode, true)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// Test/Custom GUI provider (accepts custom output mode)
func NewGocuiGuiWithOutputMode(outputMode gocui.OutputMode) (*gocui.Gui, error) {
	g, err := gocui.NewGui(outputMode, true)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// Core dependencies (shared between production and test)
var CoreDepsSet = wire.NewSet(
	ProvideCommandEventBus,
)

// Production app dependencies (includes config-based GUI)
var ProdAppDepsSet = wire.NewSet(
	CoreDepsSet,
	ProvideGenie,
	ProvideConfigManager,
	NewGocuiGui, // Uses config-based output mode
	NewApp,      // App now receives *gocui.Gui as dependency
)

// Test app dependencies (uses custom output mode GUI)
var TestAppDepsSet = wire.NewSet(
	CoreDepsSet,
	ProvideConfigManager,
	NewGocuiGuiWithOutputMode, // Uses provided output mode
)

// Production TUI injector (default output mode from config)
func InjectTUI(session *genie.Session) (*TUI, error) {
	wire.Build(ProdAppDepsSet, New)
	return nil, nil
}

// Test App injector (custom output mode)
func InjectTestApp(genieService genie.Genie, session *genie.Session, outputMode gocui.OutputMode) (*App, error) {
	wire.Build(TestAppDepsSet, NewApp)
	return nil, nil
}
