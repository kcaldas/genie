package commands

import (
	"github.com/kcaldas/genie/cmd/tui/controllers"
	"github.com/kcaldas/genie/cmd/tui/types"
)

type DebugCommand struct {
	BaseCommand
	controller   *controllers.DebugController
	logger       types.Logger
	notification *controllers.ChatController
}

func NewDebugCommand(controller *controllers.DebugController, logger types.Logger, notification *controllers.ChatController) *DebugCommand {
	return &DebugCommand{
		BaseCommand: BaseCommand{
			Name:        "debug",
			Description: "Toggle debug logging on/off (use F12 to view debug panel)",
			Usage:       ":debug",
			Examples: []string{
				":debug",
			},
			Aliases:  []string{},
			Category: "Development",
		},
		controller:   controller,
		logger:       logger,
		notification: notification,
	}
}

func (c *DebugCommand) Execute(args []string) error {
	// Toggle debug mode in DebugController
	currentState := c.controller.IsDebugMode()
	newState := !currentState
	c.controller.SetDebugMode(newState)

	// Show status message
	status := "disabled"
	if newState {
		status = "enabled"
		c.logger.Debug("Debug mode enabled via :debug command")
	} else {
		c.logger.Debug("Debug mode disabled via :debug command")
	}

	c.notification.AddSystemMessage("Debug logging " + status + ". Use F12 to view debug panel.")

	return nil
}
