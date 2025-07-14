package controllers

import (
	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/component"
	"github.com/kcaldas/genie/cmd/tui/helpers"
	"github.com/kcaldas/genie/cmd/tui/layout"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type WriteController struct {
	gui             types.Gui
	configManager   *helpers.ConfigManager
	commandEventBus *events.CommandEventBus
	layoutManager   *layout.LayoutManager
}

func NewWriteController(
	gui types.Gui,
	configManager *helpers.ConfigManager,
	commandEventBus *events.CommandEventBus,
	layoutManager *layout.LayoutManager,
) *WriteController {
	return &WriteController{
		gui:             gui,
		configManager:   configManager,
		commandEventBus: commandEventBus,
		layoutManager:   layoutManager,
	}
}

func (c *WriteController) Show() error {
	// Disable all panel keybindings so write component is the only thing handling events
	c.layoutManager.DisableAllKeybindings()

	// Create write component
	writeComponent := component.NewWriteComponent(
		c.gui,
		c.configManager,
		c.commandEventBus,
		func() error {
			// On close, delete the write view and restore all keybindings
			if gui := c.gui.GetGui(); gui != nil {
				gui.DeleteView("write")
			}
			// Re-enable all panel keybindings
			c.layoutManager.EnableAllKeybindings()
			return c.layoutManager.FocusPanel("input")
		},
	)

	// Show the write component using its Show() method
	return writeComponent.Show()
}

func (c *WriteController) Close() error {
	// Delete the write view and restore all keybindings
	if gui := c.gui.GetGui(); gui != nil {
		gui.DeleteView("write")
	}
	// Re-enable all panel keybindings
	c.layoutManager.EnableAllKeybindings()
	return c.layoutManager.FocusPanel("input")
}