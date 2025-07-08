//go:build wireinject

package di

import (
	"github.com/google/wire"
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui"
	internalDI "github.com/kcaldas/genie/internal/di"
	"github.com/kcaldas/genie/pkg/genie"
)

// Command-level providers shared between CLI and TUI

// Shared command event bus instance
var commandEventBus = events.NewCommandEventBus()

// Shared genie instance (singleton)
var genieInstance genie.Genie
var genieError error
var genieInitialized bool

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

// Wire set for shared command infrastructure
var CommandWireSet = wire.NewSet(
	ProvideCommandEventBus,
)

// TUI-specific providers

// ProvideApp provides an App instance with injected dependencies
func ProvideApp(genieService genie.Genie, session *genie.Session, commandEventBus *events.CommandEventBus) (*tui.App, error) {
	return tui.NewApp(genieService, session, commandEventBus)
}

// ProvideTUI provides a TUI instance with injected App
func ProvideTUI(app *tui.App) *tui.TUI {
	return tui.New(app)
}

// Wire injector functions

// InjectCommandEventBus is a wire injector function
func InjectCommandEventBus() *events.CommandEventBus {
	wire.Build(CommandWireSet)
	return nil
}

// InjectApp is a wire injector function for creating App with dependencies
func InjectApp(genieService genie.Genie, session *genie.Session) (*tui.App, error) {
	wire.Build(ProvideCommandEventBus, ProvideApp)
	return nil, nil
}

// InjectTUI is a wire injector function for creating TUI with dependencies
func InjectTUI(session *genie.Session) (*tui.TUI, error) {
	wire.Build(ProvideGenie, ProvideCommandEventBus, ProvideApp, ProvideTUI)
	return nil, nil
}