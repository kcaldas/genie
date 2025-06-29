package commands

import (
	"github.com/awesome-gocui/gocui"
)

type DebugCommand struct {
	BaseCommand
	ctx *CommandContext
}

func NewDebugCommand(ctx *CommandContext) *DebugCommand {
	return &DebugCommand{
		BaseCommand: BaseCommand{
			Name:        "debug",
			Description: "Toggle debug panel visibility to show tool calls and system events",
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
	c.ctx.DebugComponent.ToggleVisibility()
	
	// Force layout refresh and cleanup when hiding
	gui := c.ctx.GuiCommon.GetGui()
	gui.Update(func(g *gocui.Gui) error {
		if !c.ctx.DebugComponent.IsVisible() {
			// Delete the debug view when hiding
			g.DeleteView("debug")
		}
		return nil
	})
	
	return nil
}