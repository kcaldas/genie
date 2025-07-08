package commands

import (
	"github.com/kcaldas/genie/cmd/tui/types"
)

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
	// Toggle debug enabled state in config
	config := c.ctx.GuiCommon.GetConfig()
	config.DebugEnabled = !config.DebugEnabled
	
	// Save the config
	if err := c.ctx.ConfigHelper.Save(config); err != nil {
		c.ctx.StateAccessor.AddMessage(types.Message{
			Role:    "error",
			Content: "Failed to save debug setting: " + err.Error(),
		})
		c.ctx.CommandEventBus.Emit("ui.refresh", nil)
		return nil
	}
	
	// Show status message
	status := "disabled"
	if config.DebugEnabled {
		status = "enabled"
	}
	c.ctx.StateAccessor.AddMessage(types.Message{
		Role:    "system", 
		Content: "Debug logging " + status + ". Use F12 to view debug panel.",
	})
	
	c.ctx.CommandEventBus.Emit("ui.refresh", nil)
	return nil
}