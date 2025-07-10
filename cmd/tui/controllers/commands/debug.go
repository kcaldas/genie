package commands

type DebugCommand struct {
	BaseCommand
	ctx *CommandContext
}

func NewDebugCommand(ctx *CommandContext) *DebugCommand {
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
		ctx: ctx,
	}
}

func (c *DebugCommand) Execute(args []string) error {
	// Toggle debug mode in DebugController
	currentState := c.ctx.DebugController.IsDebugMode()
	newState := !currentState
	c.ctx.DebugController.SetDebugMode(newState)

	// Also update config for persistence
	config := c.ctx.GuiCommon.GetConfig()
	config.DebugEnabled = newState

	// Save the config
	if err := c.ctx.ConfigHelper.Save(config); err != nil {
		c.ctx.ChatController.AddErrorMessage("Failed to save debug setting: " + err.Error())
		c.ctx.DebugController.AddDebugMessage("Failed to save debug config: " + err.Error())
		return nil
	}

	// Show status message
	status := "disabled"
	if newState {
		status = "enabled"
		c.ctx.DebugController.AddDebugMessage("Debug mode enabled via :debug command")
	} else {
		c.ctx.DebugController.AddDebugMessage("Debug mode disabled via :debug command")
	}

	c.ctx.ChatController.AddSystemMessage("Debug logging " + status + ". Use F12 to view debug panel.")

	return nil
}

