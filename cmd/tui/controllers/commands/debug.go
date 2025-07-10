package commands

import (
	"github.com/kcaldas/genie/cmd/tui/controllers"
)

type DebugCommand struct {
	BaseCommand
	ctx        *CommandContext
	controller *controllers.DebugController
}

func NewDebugCommand(ctx *CommandContext, controller *controllers.DebugController) *DebugCommand {
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
		ctx:        ctx,
		controller: controller,
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
		c.ctx.Logger.Debug("Debug mode enabled via :debug command")
	} else {
		c.ctx.Logger.Debug("Debug mode disabled via :debug command")
	}

	c.ctx.Notification.AddSystemMessage("Debug logging " + status + ". Use F12 to view debug panel.")

	return nil
}
