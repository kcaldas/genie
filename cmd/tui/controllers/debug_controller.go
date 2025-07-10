package controllers

import (
	"fmt"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
)

type DebugController struct {
	*BaseController
	debugState      *state.DebugState
	debugComponent  *component.DebugComponent
	commandEventBus *events.CommandEventBus
}

func NewDebugController(
	genieService genie.Genie,
	gui types.IGuiCommon,
	debugState *state.DebugState,
	debugComponent *component.DebugComponent,
	commandEventBus *events.CommandEventBus,
) *DebugController {
	c := &DebugController{
		BaseController:  NewBaseController(debugComponent, gui),
		debugState:      debugState,
		debugComponent:  debugComponent,
		commandEventBus: commandEventBus,
	}

	eventBus := genieService.GetEventBus()
	eventBus.Subscribe("chat.started", func(e interface{}) {
		c.Debug("Event consumed: chat.started")
	})

	// Subscribe to debug mode toggle events
	commandEventBus.Subscribe("debug.toggle", func(data interface{}) {
		c.toggleDebugMode()
	})

	// Subscribe to debug clear events
	commandEventBus.Subscribe("debug.clear", func(data interface{}) {
		c.ClearDebugMessages()
	})

	return c
}

// AddDebugMessage adds a debug message and notifies component
func (c *DebugController) AddDebugMessage(message string) {
	c.debugState.AddDebugMessage(message)
	c.gui.PostUIUpdate(func() {
		c.debugComponent.OnMessageAdded()
	})
}

// ClearDebugMessages clears all debug messages and triggers render
func (c *DebugController) ClearDebugMessages() {
	c.debugState.ClearDebugMessages()
	c.renderDebugComponent()
}

// GetDebugMessages returns all debug messages
func (c *DebugController) GetDebugMessages() []string {
	return c.debugState.GetDebugMessages()
}

// IsDebugMode returns whether debug mode is enabled
func (c *DebugController) IsDebugMode() bool {
	return c.debugState.IsDebugMode()
}

// SetDebugMode sets the debug mode and triggers render
func (c *DebugController) SetDebugMode(enabled bool) {
	c.debugState.SetDebugMode(enabled)
	c.renderDebugComponent()
}

// toggleDebugMode toggles debug mode on/off
func (c *DebugController) toggleDebugMode() {
	c.debugState.SetDebugMode(!c.debugState.IsDebugMode())
	c.renderDebugComponent()
}

// Debug implements the Logger interface
func (c *DebugController) Debug(message string) {
	c.AddDebugMessage(message)
}

// renderDebugComponent triggers a render of the debug component
func (c *DebugController) renderDebugComponent() {
	c.gui.PostUIUpdate(func() {
		if err := c.debugComponent.Render(); err != nil {
			// Log error but don't propagate - debug rendering shouldn't break the app
			c.debugState.AddDebugMessage(fmt.Sprintf("Error rendering debug: %v", err))
		}
	})
}
