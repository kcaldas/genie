package controllers

import (
	"fmt"
	"strings"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/cmd/tui/types"
	"github.com/kcaldas/genie/pkg/genie"
)

type DebugController struct {
	*BaseController
	debugState      *state.DebugState
	debugComponent  *component.DebugComponent
	layoutManager   *layout.LayoutManager
	clipboard       *helpers.Clipboard
	commandEventBus *events.CommandEventBus
}

func NewDebugController(
	genieService genie.Genie,
	gui types.IGuiCommon,
	debugState *state.DebugState,
	debugComponent *component.DebugComponent,
	layoutManager *layout.LayoutManager,
	clipboard *helpers.Clipboard,
	config *helpers.ConfigManager,
	commandEventBus *events.CommandEventBus,
) *DebugController {
	c := &DebugController{
		BaseController:  NewBaseController(debugComponent, gui, config),
		debugState:      debugState,
		debugComponent:  debugComponent,
		layoutManager:   layoutManager,
		clipboard:       clipboard,
		commandEventBus: commandEventBus,
	}

	// Set initial debug mode from config
	c.SetDebugMode(c.GetConfig().DebugEnabled)

	eventBus := genieService.GetEventBus()
	eventBus.Subscribe("chat.started", func(e interface{}) {
		c.Debug("Event consumed: chat.started")
	})

	// Subscribe to debug mode toggle events
	commandEventBus.Subscribe("debug.toggle", func(data interface{}) {
		c.toggleDebugMode()
	})

	// Subscribe to debug view events
	commandEventBus.Subscribe("debug.view", func(data interface{}) {
		c.toggleDebugPanel()
	})

	// Subscribe to debug clear events
	commandEventBus.Subscribe("debug.clear", func(data interface{}) {
		c.ClearDebugMessages()
	})

	// Subscribe to debug copy events
	commandEventBus.Subscribe("debug.copy", func(data interface{}) {
		c.CopyDebugMessages()
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

// CopyDebugMessages copy all debug messages to clipboard
func (c *DebugController) CopyDebugMessages() {
	messages := c.debugState.GetDebugMessages()
	messagesStr := strings.Join(messages, "\n")
	c.clipboard.Copy(messagesStr)
}

// IsDebugMode returns whether debug mode is enabled
func (c *DebugController) IsDebugMode() bool {
	return c.debugState.IsDebugMode()
}

// SetDebugMode sets the debug mode and triggers render
func (c *DebugController) SetDebugMode(enabled bool) {
	// The debug mode on the state is used to accept/not-accept messages
	// So we dont keep accumulating messages if not debugging.
	c.debugState.SetDebugMode(enabled)

	// Also update config for persistence
	err := c.GetConfigManager().UpdateConfig(func(config *types.Config) {
		config.DebugEnabled = enabled
	}, true)
	if err != nil {
		c.AddDebugMessage("Failed to save debug config: " + err.Error())
	}
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

func (c *DebugController) toggleDebugPanel() {
	// Use the new Panel system for cleaner toggle logic
	if debugPanel := c.layoutManager.GetPanel("debug"); debugPanel != nil {
		// Toggle visibility - Panel handles view lifecycle automatically
		isVisible := debugPanel.IsVisible()
		debugPanel.SetVisible(!isVisible)

		// If becoming visible, render to show all collected messages
		if !isVisible {
			debugPanel.Render()
		}
	}
}
