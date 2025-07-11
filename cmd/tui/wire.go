//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build wireinject
// +build wireinject

package tui

import (
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

// Shared provider set
var AppDepsSet = wire.NewSet(
	ProvideGenie,
	ProvideCommandEventBus,
	ProvideConfigManager,
	NewApp,
)

func InjectTUI(session *genie.Session) (*TUI, error) {
	wire.Build(AppDepsSet, New)
	return nil, nil
}
